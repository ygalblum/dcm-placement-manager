package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/dcm-project/placement-manager/internal/apiserver"
	"github.com/dcm-project/placement-manager/internal/config"
	"github.com/dcm-project/placement-manager/internal/handlers"
	"github.com/dcm-project/placement-manager/internal/policy"
	"github.com/dcm-project/placement-manager/internal/service"
	"github.com/dcm-project/placement-manager/internal/sprm"
	"github.com/dcm-project/placement-manager/internal/store"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	db, err := store.InitDB(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Initialize store
	dataStore := store.NewStore(db)
	defer dataStore.Close()

	// Initialize policy client
	policyClient, err := policy.NewClient(cfg.Policy.URL)
	if err != nil {
		log.Fatalf("Failed to initialize policy client: %v", err)
	}
	log.Printf("Policy client initialized with URL: %s", cfg.Policy.URL)

	// Initialize SPRM client
	sprmClient, err := sprm.NewClient(cfg.SPRM.URL)
	if err != nil {
		log.Fatalf("Failed to initialize SPRM client: %v", err)
	}
	log.Printf("SPRM client initialized with URL: %s", cfg.SPRM.URL)

	// Initialize service
	placementService := service.NewPlacementService(dataStore, policyClient, sprmClient)

	// Create TCP listener
	listener, err := net.Listen("tcp", cfg.Service.Address)
	if err != nil {
		log.Fatalf("Failed to create listener: %v", err)
	}

	// Initialize handler
	handler := handlers.NewHandler(placementService)

	// Create API server
	srv := apiserver.New(cfg, listener, handler)

	// Setup graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.Printf("Starting Placement Manager API server on %s", listener.Addr().String())
	if err := srv.Run(ctx); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
