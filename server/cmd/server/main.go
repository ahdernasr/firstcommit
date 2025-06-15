package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"go.mongodb.org/mongo-driver/bson"

	"ai-in-action/internal/config"
	"ai-in-action/internal/database"
	"ai-in-action/internal/github"
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
	log.Printf("Using main database: %s", cfg.DBName)

	federatedDB := federatedClient.Database("reposdb") // Use the correct federated database name
	log.Printf("Using federated database: reposdb")

	repoRepo, err := repository.NewRepoRepository(mainDB, federatedDB)
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

	// Initialize local embedders
	metadataEmbedder, err := service.NewLocalEmbedder("metadata")
	if err != nil {
		log.Fatalf("Failed to initialize metadata embedder: %v", err)
	}
	defer metadataEmbedder.Close()

	codeEmbedder, err := service.NewLocalEmbedder("code")
	if err != nil {
		log.Fatalf("Failed to initialize code embedder: %v", err)
	}
	defer codeEmbedder.Close()

	// Initialize GitHub client
	ghClient := github.NewClient(cfg.GitHubToken)
	log.Printf("Initialized GitHub client")

	// Initialize services
	searchSvc := service.NewSearchService(repoRepo, metadataEmbedder)
	repoSvc := service.NewRepoService(repoRepo, ghClient)
	guideSvc := service.NewGuideService(guideRepo, ghClient, repoRepo, metadataEmbedder, service.NewDummyLLM())
	chatSvc := service.NewChatService(guideSvc)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	// Add middleware
	app.Use(middleware.Logging())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*", // Allow all origins for development
		AllowHeaders: "Origin, Content-Type, Accept",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
	}))

	// Register routes
	handler.RegisterRoutes(app, searchSvc, repoSvc, guideSvc, chatSvc, repoRepo, metadataEmbedder, codeEmbedder)

	// Add health check
	healthHandler := handler.NewHealthHandler(mainClient, federatedClient)
	healthHandler.Register(app)

	// Start server
	log.Printf("Server starting on port %s", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
