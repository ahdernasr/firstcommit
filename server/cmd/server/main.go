package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"

	"ai-in-action/internal/config"
	"ai-in-action/internal/database"
	"ai-in-action/internal/handler"
	"ai-in-action/internal/middleware"
	"ai-in-action/internal/repository"
	"ai-in-action/internal/service"
)

// main is the single entry‑point for the REST API.
func main() {
	// Load configuration
	cfg := config.Load()
	log.Printf("Configuration loaded:")
	log.Printf("  - Database: %s", cfg.DBName)
	log.Printf("  - MongoDB URI: %s", cfg.MongoURI)
	log.Printf("  - Federated MongoDB URI: %s", cfg.FederatedMongoURI)

	// Connect to main MongoDB (for embeddings)
	mainClient, mainCtx, mainCancel, err := database.NewMongo(cfg.MongoURI)
	if err != nil {
		log.Fatalf("Failed to connect to main MongoDB: %v", err)
	}
	defer mainCancel()
	defer mainClient.Disconnect(mainCtx)
	log.Printf("Connected to main MongoDB")

	// Connect to federated MongoDB (for code access)
	federatedClient, fedCtx, fedCancel, err := database.NewMongo(cfg.FederatedMongoURI)
	if err != nil {
		log.Fatalf("Failed to connect to federated MongoDB: %v", err)
	}
	defer fedCancel()
	defer federatedClient.Disconnect(fedCtx)
	log.Printf("Connected to federated MongoDB")

	// Initialize repositories
	mainDB := mainClient.Database(cfg.DBName)
	log.Printf("Using database: %s", cfg.DBName)

	repoRepo, err := repository.NewRepoRepository(mainDB)
	if err != nil {
		log.Fatalf("Failed to initialize repository repository: %v", err)
	}

	guideRepo := repository.NewGuideRepository(mainDB)

	// List collections to verify access
	collections, err := mainDB.ListCollectionNames(mainCtx, bson.M{})
	if err != nil {
		log.Printf("Warning: Failed to list collections: %v", err)
	} else {
		log.Printf("Available collections: %v", collections)
	}

	// Initialize Vertex AI embedder
	embedder, err := service.NewVertexEmbedder()
	if err != nil {
		log.Fatalf("Failed to initialize Vertex AI embedder: %v", err)
	}
	defer embedder.Close()

	// Initialize Gemini embedder for repos_meta
	geminiEmbedder, err := service.NewGeminiEmbedder()
	if err != nil {
		log.Fatalf("Failed to initialize Gemini embedder: %v", err)
	}
	defer geminiEmbedder.Close()

	// Initialize services
	searchSvc := service.NewSearchService(repoRepo, geminiEmbedder)
	repoSvc := service.NewRepoService(repoRepo, nil) // TODO: Add GitHub client
	guideSvc := service.NewGuideService(guideRepo, nil, repoRepo, geminiEmbedder, service.NewDummyLLM())
	chatSvc := service.NewChatService(guideSvc)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	// Add middleware
	app.Use(middleware.Logging())

	// Register routes
	handler.RegisterRoutes(app, searchSvc, repoSvc, guideSvc, chatSvc)

	// Add health check
	healthHandler := handler.NewHealthHandler(mainClient, federatedClient)
	healthHandler.Register(app)

	// Start server
	log.Printf("Server starting on port %s", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
