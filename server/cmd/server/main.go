package main

import (
	"context"
	"log"

	"cloud.google.com/go/storage"
	"github.com/ahmednasr/ai-in-action/server/internal/config"
	"github.com/ahmednasr/ai-in-action/server/internal/database"
	"github.com/ahmednasr/ai-in-action/server/internal/github"
	"github.com/ahmednasr/ai-in-action/server/internal/handler"
	"github.com/ahmednasr/ai-in-action/server/internal/repository"
	"github.com/ahmednasr/ai-in-action/server/internal/service"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"go.mongodb.org/mongo-driver/bson"
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

	// Initialize GCS client
	storageClient, err := storage.NewClient(context.Background())
	if err != nil {
		log.Fatalf("Failed to create GCS client: %v", err)
	}
	defer storageClient.Close()
	log.Printf("Connected to Google Cloud Storage")

	// Initialize repositories
	mainDB := mainClient.Database(cfg.DBName)
	log.Printf("Using main database: %s", cfg.DBName)

	federatedDB := federatedClient.Database("reposdb") // Use the correct federated database name
	log.Printf("Using federated database: reposdb")

	repoRepo, err := repository.NewRepoRepository(mainDB, federatedDB, storageClient)
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
	codeSvc := service.NewCodeService(repoRepo)

	// Initialize Vertex AI LLM
	llm, err := service.NewVertexLLM()
	if err != nil {
		log.Fatalf("Failed to initialize Vertex AI LLM: %v", err)
	}
	defer llm.Close()

	guideSvc := service.NewGuideService(guideRepo, ghClient, repoRepo, metadataEmbedder, llm)
	chatSvc := service.NewChatService(guideSvc)

	// Use code embedder for RAG service
	ragService := service.NewRAGService(mainDB.Collection("repos_code"), mainDB.Collection("repos_meta"), codeEmbedder, llm, guideSvc)

	// Initialize handlers
	healthHandler := handler.NewHealthHandler(mainClient, federatedClient)
	ragHandler := handler.NewRAGHandler(ragService)
	codeSearchHandler := handler.NewCodeSearchHandler(repoRepo, codeEmbedder, codeSvc)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	// Add middleware
	app.Use(cors.New(cors.Config{
		AllowOrigins: "https://frontend-222198140851.us-central1.run.app,http://localhost:3000",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		MaxAge:       300, // Cache preflight requests for 5 minutes
	}))
	app.Use(logger.New())
	app.Use(recover.New())

	app.Options("/*", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	// Register routes
	handler.RegisterRoutes(app, searchSvc, repoSvc, guideSvc, chatSvc, repoRepo, metadataEmbedder, codeEmbedder, codeSvc)
	healthHandler.Register(app)
	ragHandler.RegisterRoutes(app)
	codeSearchHandler.Register(app)

	// Start server
	log.Printf("Server starting on port %s", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
