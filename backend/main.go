package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/riont/crm/backend/internal/agent"
	"github.com/riont/crm/backend/internal/asr"
	"github.com/riont/crm/backend/internal/auth"
	"github.com/riont/crm/backend/internal/config"
	"github.com/riont/crm/backend/internal/database"
	"github.com/riont/crm/backend/internal/handlers"
	"github.com/riont/crm/backend/internal/insight"
	"github.com/riont/crm/backend/internal/logging"
	"github.com/riont/crm/backend/internal/repository"
	"github.com/riont/crm/backend/internal/sse"
	"github.com/riont/crm/backend/internal/zernio"
)

// appVersion identifies the deployed build; bump to force/verify deploys.
const appVersion = "1.5.0"

func main() {
	log := logging.Setup()
	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run migrations (non-fatal: server starts even if DB is down)
	if err := database.Migrate(cfg.DatabaseURL); err != nil {
		log.Warn("migración falló, el servidor arranca igual", "err", err)
	} else {
		log.Info("migraciones aplicadas")
	}

	// Create connection pool
	pool, err := database.NewPool(ctx)
	if err != nil {
		log.Warn("no se pudo conectar a la base, el servidor arranca igual", "err", err)
	}

	// Initialize repositories
	var webhookEvents *repository.WebhookEventRepository
	var contacts *repository.ContactRepository
	var identities *repository.ContactIdentityRepository
	var conversations *repository.ConversationRepository
	var messages *repository.MessageRepository

	if pool != nil {
		defer pool.Close()
		webhookEvents = repository.NewWebhookEventRepository(pool)
		contacts = repository.NewContactRepository(pool)
		identities = repository.NewContactIdentityRepository(pool)
		conversations = repository.NewConversationRepository(pool)
		messages = repository.NewMessageRepository(pool)
	}

	// Initialize Zernio client
	zernioClient := zernio.NewClient(cfg.ZernioAPIKey)

	// Initialize SSE hub
	sseHub := sse.NewHub()

	// Initialize agent system (needs pool)
	var agentWH *agent.WebhookHandler
	var webhookHandler *handlers.WebhookHandler
	var inboxHandler *handlers.InboxHandler
	var sendHandler *handlers.SendHandler

	if pool != nil {
		agentConfigRepo := agent.NewConfigRepository(pool)
		openRouterClient := agent.NewOpenRouterClient()
		aiAgent := agent.NewAgent(agentConfigRepo, openRouterClient, pool)
		agentWH = agent.NewWebhookHandler(aiAgent, messages, conversations, contacts, zernioClient, pool)

		// Initialize inbox handler
		inboxHandler = handlers.NewInboxHandler(conversations, messages, contacts)

		// Initialize send handler
		sendHandler = handlers.NewSendHandler(messages, conversations, contacts, zernioClient)

		// Initialize webhook handler
		webhookHandler = handlers.NewWebhookHandler(
			webhookEvents,
			contacts,
			identities,
			conversations,
			messages,
			zernioClient,
			cfg.ZernioWebhookSecret,
			sseHub,
		)

		// Wire agent after-message hook
		webhookHandler.SetAfterMessage(agentWH.AfterMessageReceived)
	}

	// Analisis de pedidos con IA local (Fase 19/20).
	//
	// Se arma FUERA del if de pool para que el servidor no muera si falta la
	// DB: /api/insights responde 503 y el inbox sigue andando. Es la misma
	// politica que el resto del sistema.
	var insightSvc *insight.Service
	var insightHandler *handlers.InsightHandler
	if pool != nil && cfg.InsightEnabled {
		insightRepo := insight.NewRepository(pool)
		catalogRepo := insight.NewCatalogRepository(insightRepo)

		llm := insight.NewOllamaClient(cfg.OllamaBaseURL, cfg.OllamaModel, cfg.OllamaTimeout, log)
		asrClient := asr.NewClient(cfg.ASRBaseURL, cfg.WhisperModel, cfg.ASRTimeout, log)
		mediaFetcher := asr.NewFetcher(cfg.ZernioAPIKey, cfg.ASRDownloadTime)

		insightSvc = insight.NewService(insight.ServiceDeps{
			Repo:        insightRepo,
			Catalog:     catalogRepo,
			LLM:         llm,
			ASR:         asrClient,
			Fetcher:     mediaFetcher,
			SSE:         sseHub,
			Logger:      log,
			TextWorkers: cfg.TextWorkers,
		})
		insightSvc.Start(ctx)
		// El orden importa: Stop() ESPERA a que los workers terminen, y los
		// workers solo terminan cuando ven ctx.Done(). Con dos defers sueltos,
		// el LIFO ejecuta Stop() antes que cancel() y el apagado se queda
		// colgado para siempre. Un solo defer, en el orden correcto.
		defer func() {
			cancel()
			insightSvc.Stop()
		}()

		insightHandler = handlers.NewInsightHandler(insightSvc, insightRepo, catalogRepo, log)
		inboxHandler.SetInsightRepo(insightRepo)
		// El handler de insights necesita el hub para avisar cuando un humano
		// aplica o reabre un pedido. Sin esto la bandeja se entera del cambio
		// solo en el proximo refetch.
		insightHandler.SetSSE(sseHub)

		// El analisis se encola, nunca se ejecuta aca: el webhook tiene 5
		// segundos para devolver 2xx y un POST a Ollama puede tardar 30.
		webhookHandler.SetAfterPersist(insightSvc.EnqueueAfterMessage)

		log.Info("analisis de pedidos habilitado",
			"ollama", cfg.OllamaBaseURL, "model", cfg.OllamaModel,
			"asr", cfg.ASRBaseURL, "whisper", cfg.WhisperModel,
			"text_workers", cfg.TextWorkers)
	} else if !cfg.InsightEnabled {
		log.Info("analisis de pedidos DESHABILITADO por config (INSIGHT_ENABLED=false)")
	}

	// Echo setup
	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// Health check
	e.GET("/health", func(c echo.Context) error {
		if pool == nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{
				"status": "error",
				"error":  "database not connected",
			})
		}
		if err := pool.Ping(ctx); err != nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{
				"status": "error",
				"error":  "database unreachable",
			})
		}
		return c.JSON(http.StatusOK, map[string]string{
			"status":  "ok",
			"version": appVersion,
		})
	})

	// Webhook endpoint (always registered)
	e.POST("/webhook/zernio", func(c echo.Context) error {
		if webhookHandler == nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "database not connected"})
		}
		return webhookHandler.HandleWebhook(c)
	})

	// SSE endpoint for real-time updates (moved to protected group below)
	// File upload + serve handlers (MinIO)
	uploadHandler := handlers.NewUploadHandler()
	filesHandler := handlers.NewFilesHandler()

	// Auth service + handler (works with nil pool: handlers respond 503)
	userService := auth.NewUserService(pool)
	authHandler := handlers.NewAuthHandler(userService)

	if os.Getenv("AUTH_JWT_SECRET") == "" {
		log.Warn("AUTH_JWT_SECRET no configurada: el login va a estar deshabilitado")
	}

	// Public auth endpoints
	e.POST("/api/auth/login", authHandler.Login)
	e.POST("/api/auth/logout", authHandler.Logout)

	// Protected API group: everything else under /api requires a session
	protected := e.Group("/api", auth.RequireAuth())

	protected.GET("/auth/me", authHandler.Me)

	protected.GET("/events", sseHub.ServeHTTP)
	protected.POST("/upload", uploadHandler.Upload)
	protected.GET("/files/*", filesHandler.Serve)

	// Media proxy: fetches Zernio media URLs and serves them to the frontend
	protected.GET("/media", func(c echo.Context) error {
		mediaURL := c.QueryParam("url")
		if mediaURL == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "missing url param"})
		}
		if cfg.ZernioAPIKey == "" {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "ZERNIO_API_KEY not configured"})
		}

		req, err := http.NewRequest("GET", mediaURL, nil)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid url"})
		}
		req.Header.Set("Authorization", "Bearer "+cfg.ZernioAPIKey)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return c.JSON(http.StatusBadGateway, map[string]string{"error": "failed to fetch media"})
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return c.JSON(http.StatusBadGateway, map[string]string{"error": "upstream returned non-200"})
		}

		contentType := resp.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		c.Response().Header().Set("Content-Type", contentType)
		c.Response().Header().Set("Cache-Control", "public, max-age=86400")
		return c.Stream(http.StatusOK, contentType, resp.Body)
	})

	// Inbox API (for frontend)
	if inboxHandler != nil && sendHandler != nil {
		inbox := protected.Group("/inbox")
		inbox.GET("/conversations", inboxHandler.ListConversations)
		inbox.GET("/conversations/:id", inboxHandler.GetConversation)
		inbox.PATCH("/conversations/:id", inboxHandler.UpdateConversation)
		inbox.POST("/conversations/:id/messages", sendHandler.SendMessage)
		inbox.PATCH("/contacts/:id", inboxHandler.UpdateContactNotes)
		inbox.GET("/unread", inboxHandler.UnreadFeed)
		inbox.GET("/search", inboxHandler.Search)
		whatsapp := protected.Group("/whatsapp")
		whatsapp.GET("/templates", sendHandler.ListWhatsAppTemplates)
		whatsapp.POST("/templates", sendHandler.CreateWhatsAppTemplate)
	}

	// Agent config API
	if pool != nil {
		agentsHandler := handlers.NewAgentsHandler(pool)
		protected.GET("/agents", agentsHandler.ListAgents)
		protected.PATCH("/agents/:channel", agentsHandler.UpdateAgent)

		templateRepo := repository.NewTemplateRepository(pool)
		templatesHandler := handlers.NewTemplatesHandler(templateRepo)
		protected.GET("/templates", templatesHandler.ListTemplates)
		protected.POST("/templates", templatesHandler.CreateTemplate)
		protected.PUT("/templates/:id", templatesHandler.UpdateTemplate)
		protected.DELETE("/templates/:id", templatesHandler.DeleteTemplate)

		statsHandler := handlers.NewStatsHandler(pool)
		protected.GET("/stats/overview", statsHandler.Overview)

		reportsHandler := handlers.NewReportsHandler(pool)
		protected.GET("/stats/reports", reportsHandler.Report)

		channelsHandler := handlers.NewChannelsHandler(pool)
		protected.GET("/channels/status", channelsHandler.Status)
	}

	// Analisis de pedidos, catalogo y pedidos consolidados
	if insightHandler != nil {
		insights := protected.Group("/insights")
		insights.GET("/status", insightHandler.Status)
		insights.GET("/messages/:id", insightHandler.GetMessageAnalysis)
		insights.GET("/conversation/:id", insightHandler.GetConversation)
		insights.PATCH("/settings/:key", insightHandler.UpdateSettings)
		insights.PATCH("/config/:channel", insightHandler.UpdateChannelConfig)
		insights.POST("/backfill", insightHandler.Backfill)

		orders := protected.Group("/orders")
		orders.GET("", insightHandler.ListOrders)
		orders.GET("/:id", insightHandler.GetOrder)
		orders.PATCH("/:id", insightHandler.UpdateOrder)
		orders.PATCH("/:id/status", insightHandler.SetOrderStatus)
		// Aplicar y reabrir son POST y no PATCH porque no editan un recurso:
		// ejecutan una transicion (crear la revision siguiente / sacar el sello de
		// edicion). Un PATCH sin cuerpo seria ambiguo con el PATCH de correccion.
		orders.POST("/:id/accept-pending", insightHandler.AcceptPending)
		orders.POST("/:id/reopen", insightHandler.ReopenOrder)

		catalog := protected.Group("/catalog")
		catalog.GET("", insightHandler.ListCatalog)
		catalog.POST("", insightHandler.CreateCatalogItem)
		catalog.PATCH("/:id", insightHandler.UpdateCatalogItem)
		catalog.DELETE("/:id", insightHandler.DeleteCatalogItem)
	}

	// Users management API (admin only)
	users := protected.Group("/users", auth.RequireRole("admin"))
	users.GET("", authHandler.ListUsers)
	users.POST("", authHandler.CreateUser)
	users.PUT("/:id", authHandler.UpdateUser)
	users.DELETE("/:id", authHandler.DeleteUser)

	// System info (admin only)
	handlers.AppVersion = appVersion
	protected.GET("/system/info", handlers.SystemInfoHandler(pool), auth.RequireRole("admin"))

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Info("apagando el servidor")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := e.Shutdown(shutdownCtx); err != nil {
			log.Error("apagado con error", "err", err)
		}
	}()

	addr := ":" + cfg.Port
	log.Info("servidor escuchando", "addr", addr, "version", appVersion)
	if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
		log.Error("el servidor no pudo arrancar", "err", err)
		os.Exit(1)
	}
}
