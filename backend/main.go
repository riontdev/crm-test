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
	"github.com/riont/crm/backend/internal/config"
	"github.com/riont/crm/backend/internal/database"
	"github.com/riont/crm/backend/internal/handlers"
	"github.com/riont/crm/backend/internal/repository"
	"github.com/riont/crm/backend/internal/sse"
	"github.com/riont/crm/backend/internal/zernio"
)

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
			"status": "ok",
		})
	})

	// Webhook endpoint (always registered)
	e.POST("/webhook/zernio", func(c echo.Context) error {
		if webhookHandler == nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "database not connected"})
		}
		return webhookHandler.HandleWebhook(c)
	})

	// SSE endpoint for real-time updates
	e.GET("/api/events", sseHub.ServeHTTP)

	// File upload endpoint
	uploadHandler := handlers.NewUploadHandler()
	e.POST("/api/upload", uploadHandler.Upload)

	// Media proxy: fetches Zernio media URLs and serves them to the frontend
	e.GET("/api/media", func(c echo.Context) error {
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
		api := e.Group("/api")
		inbox := api.Group("/inbox")
		inbox.GET("/conversations", inboxHandler.ListConversations)
		inbox.GET("/conversations/:id", inboxHandler.GetConversation)
		inbox.POST("/conversations/:id/messages", sendHandler.SendMessage)
	}

	// Agent config API
	api := e.Group("/api")
	api.GET("/agents", func(c echo.Context) error {
		if pool == nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "database not connected"})
		}
		configs, err := agent.NewConfigRepository(pool).GetAll(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list agent configs"})
		}
		return c.JSON(http.StatusOK, map[string]interface{}{"data": configs})
	})

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
