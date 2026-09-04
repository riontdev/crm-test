package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/riont/crm/backend/internal/agent"
	"github.com/riont/crm/backend/internal/auth"
	"github.com/riont/crm/backend/internal/config"
	"github.com/riont/crm/backend/internal/database"
	"github.com/riont/crm/backend/internal/handlers"
	"github.com/riont/crm/backend/internal/repository"
	"github.com/riont/crm/backend/internal/sse"
	"github.com/riont/crm/backend/internal/zernio"
)

// appVersion identifies the deployed build; bump to force/verify deploys.
const appVersion = "1.4.0"

func main() {
	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run migrations (non-fatal: server starts even if DB is down)
	if err := database.Migrate(cfg.DatabaseURL); err != nil {
		log.Printf("WARNING: Migration failed: %v (server will start anyway)", err)
	} else {
		log.Println("Migrations applied successfully")
	}

	// Create connection pool
	pool, err := database.NewPool(ctx)
	if err != nil {
		log.Printf("WARNING: Failed to connect to database: %v (server will start anyway)", err)
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
	// File upload handler
	uploadHandler := handlers.NewUploadHandler()

	// Auth service + handler (works with nil pool: handlers respond 503)
	userService := auth.NewUserService(pool)
	authHandler := handlers.NewAuthHandler(userService)

	if os.Getenv("AUTH_JWT_SECRET") == "" {
		log.Printf("WARN: AUTH_JWT_SECRET no configurada — el login estará deshabilitado")
	}

	// Public auth endpoints
	e.POST("/api/auth/login", authHandler.Login)
	e.POST("/api/auth/logout", authHandler.Logout)

	// Protected API group: everything else under /api requires a session
	protected := e.Group("/api", auth.RequireAuth())

	protected.GET("/auth/me", authHandler.Me)

	protected.GET("/events", sseHub.ServeHTTP)
	protected.POST("/upload", uploadHandler.Upload)

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
		fmt.Println("Shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := e.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}()

	addr := ":" + cfg.Port
	fmt.Printf("Server starting on %s\n", addr)
	if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
