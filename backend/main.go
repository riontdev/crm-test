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
	"github.com/riont/crm/backend/internal/zernio"
)

func main() {
	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run migrations
	if err := database.Migrate(cfg.DatabaseURL); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	// Create connection pool
	pool, err := database.NewPool(ctx)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Initialize repositories
	webhookEvents := repository.NewWebhookEventRepository(pool)
	contacts := repository.NewContactRepository(pool)
	identities := repository.NewContactIdentityRepository(pool)
	conversations := repository.NewConversationRepository(pool)
	messages := repository.NewMessageRepository(pool)

	// Initialize Zernio client
	zernioClient := zernio.NewClient(cfg.ZernioAPIKey)

	// Initialize agent system
	agentConfigRepo := agent.NewConfigRepository(pool)
	openRouterClient := agent.NewOpenRouterClient()
	aiAgent := agent.NewAgent(agentConfigRepo, openRouterClient, pool)
	agentWH := agent.NewWebhookHandler(aiAgent, messages, conversations, contacts, zernioClient, pool)

	// Initialize inbox handler
	inboxHandler := handlers.NewInboxHandler(conversations, messages, contacts)

	// Initialize send handler
	sendHandler := handlers.NewSendHandler(messages, conversations, contacts, zernioClient)

	// Initialize webhook handler
	webhookHandler := handlers.NewWebhookHandler(
		webhookEvents,
		contacts,
		identities,
		conversations,
		messages,
		zernioClient,
		cfg.ZernioWebhookSecret,
	)

	// Wire agent after-message hook
	webhookHandler.SetAfterMessage(agentWH.AfterMessageReceived)

	// Echo setup
	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// Health check
	e.GET("/health", func(c echo.Context) error {
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

	// Webhook endpoint
	e.POST("/webhook/zernio", webhookHandler.HandleWebhook)

	// Inbox API (for frontend)
	api := e.Group("/api")
	inbox := api.Group("/inbox")
	inbox.GET("/conversations", inboxHandler.ListConversations)
	inbox.GET("/conversations/:id", inboxHandler.GetConversation)
	inbox.POST("/conversations/:id/messages", sendHandler.SendMessage)

	// Agent config API
	api.GET("/agents", func(c echo.Context) error {
		configs, err := agentConfigRepo.GetAll(c.Request().Context())
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
