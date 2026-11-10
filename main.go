package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"url-shortener/config"
	"url-shortener/internal/handler"
	"url-shortener/internal/repository"
	"url-shortener/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	log.Printf("Connecting to PostgreSQL at %s:%s...", cfg.Postgres.Host, cfg.Postgres.Port)
	dbPool, err := repository.InitDB(
		context.Background(),
		cfg.Postgres.DSN(),
		cfg.Postgres.MaxOpenConns,
		cfg.Postgres.MaxIdleConns,
		cfg.Postgres.ConnMaxLifetime,
	)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	log.Printf("Connected to PostgreSQL (database: %s)", cfg.Postgres.Database)
	defer dbPool.Close()

	// Run migrations
	if err := repository.RunMigrations(context.Background(), dbPool); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	log.Println("Database migrations applied")

	// Initialize Redis
	log.Printf("Connecting to Redis at %s:%s...", cfg.Redis.Host, cfg.Redis.Port)
	redisClient := repository.NewRedisClient(&cfg.Redis)
	defer redisClient.Close()

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Printf("Warning: Failed to connect to Redis: %v", err)
	} else {
		log.Println("Connected to Redis")
	}

	// Initialize repositories
	urlRepo := repository.NewPostgresRepository(dbPool)
	redisRepo := repository.NewRedisRepository(redisClient)

	// Initialize services
	cacheTTL := 24 * time.Hour
	urlService := service.NewURLService(urlRepo, redisRepo, cacheTTL)

	// Initialize handlers
	urlHandler := handler.NewURLHandler(urlService, getBaseURL())
	healthHandler := handler.NewHealthHandler(dbPool, redisClient)

	// Setup Gin router
	router := gin.Default()
	router.Use(gin.Recovery())

	// Health check endpoints
	router.GET("/healthz", healthHandler.Healthz)
	router.GET("/readyz", healthHandler.Readyz)

	// API routes
	api := router.Group("/api/v1")
	{
		api.POST("/urls", urlHandler.CreateShortURL)
		api.GET("/urls/:shortCode/stats", urlHandler.GetStats)
		api.DELETE("/urls/expired", urlHandler.DeleteExpired)
	}

	// Redirect endpoint (short code)
	router.GET("/:shortCode", urlHandler.Redirect)

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting server on port %s", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

func getBaseURL() string {
	// In production, this would come from config or be derived from the request
	// For now, use a default
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	return baseURL
}