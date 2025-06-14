package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"

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
	// 1. Load ENV / CLI flags → Config struct.
	cfg := config.Load()

	// 2. Connect to MongoDB Atlas.
	client, ctx, cancel, err := database.NewMongo(cfg.MongoURI)
	if err != nil {
		log.Fatalf("mongo connect: %v", err)
	}
	defer cancel()
	defer client.Disconnect(ctx)
	db := client.Database(cfg.DBName)

	// 3. GitHub REST API wrapper.
	gh := github.NewClient(cfg.GitHubToken)

	// 4. Build repositories (pure data‑access).
	repoRepo := repository.NewRepoRepository(db)
	guideRepo := repository.NewGuideRepository(db)

	// 5. Build AI adapters.
	emb := service.NewDummyEmbedder() // TODO: replace with real embedder
	llm := service.NewDummyLLM()      // TODO: replace with real LLM

	// 6. Build services (business logic).
	searchSvc := service.NewSearchService(repoRepo, emb)
	repoSvc := service.NewRepoService(repoRepo, gh)
	guideSvc := service.NewGuideService(guideRepo, gh, repoRepo, emb, llm)
	chatSvc := service.NewChatService(guideSvc)

	// 7. HTTP server.
	app := fiber.New()
	app.Use(middleware.Logging())

	// 8. Register REST routes.
	handler.RegisterRoutes(app, searchSvc, repoSvc, guideSvc, chatSvc)

	// 9. Run + graceful shutdown.
	go func() {
		if err := app.Listen(":" + cfg.Port); err != nil {
			log.Fatalf("fiber listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}
