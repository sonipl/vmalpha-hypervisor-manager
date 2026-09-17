package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/novasphere/novasphere/internal/api/routes"
	"github.com/novasphere/novasphere/internal/config"
	"github.com/novasphere/novasphere/internal/database"
	"github.com/novasphere/novasphere/internal/websocket"
	"github.com/sirupsen/logrus"
)

// @title NovaSphere Hypervisor API
// @version 1.0.0
// @description AI-powered KVM + KubeVirt + Ceph virtualization platform
// @host localhost:8080
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Set log level
	level, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = logrus.InfoLevel
	}
	log.SetLevel(level)
	log.Info("Starting NovaSphere Hypervisor API")

	// Initialize database
	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	log.Info("Database connected")

	// Run migrations
	if err := database.Migrate(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	log.Info("Database migrations complete")

	if err := database.SeedBootstrap(db); err != nil {
		log.Fatalf("Failed to seed bootstrap admin: %v", err)
	}
	log.Info("Administrator bootstrap check complete")

	// Initialize WebSocket hub
	hub := websocket.NewHub(log)
	go hub.Run()

	// Setup router
	router := routes.Setup(cfg, db, hub, log)

	// Create HTTP server
	srv := &http.Server{
		Addr:         net.JoinHostPort(cfg.ListenAddress, fmt.Sprint(cfg.Port)),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Infof("API server listening on :%d", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Info("Server exited")
}
