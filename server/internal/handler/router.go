package handler

import (
	"ai-in-action/internal/service"

	"github.com/gofiber/fiber/v2"
)

func RegisterRoutes(app *fiber.App,
	searchSvc service.SearchService,
	repoSvc service.RepoService,
	guideSvc service.GuideService,
	chatSvc service.ChatService,
	repoRepository service.RepoRepository,
	vertexEmbedder service.EmbeddingClient,
) {

	v1 := app.Group("/api/v1")
	NewSearchHandler(searchSvc).Register(v1)
	NewRepoHandler(repoSvc).Register(v1)
	NewGuideHandler(guideSvc).Register(v1)
	NewChatHandler(chatSvc).Register(v1)
	NewCodeSearchHandler(repoRepository, vertexEmbedder).Register(v1)
}
