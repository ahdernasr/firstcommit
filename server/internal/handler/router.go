package handler

import (
	"github.com/ahmednasr/ai-in-action/server/internal/service"

	"github.com/gofiber/fiber/v2"
)

func RegisterRoutes(app *fiber.App,
	searchSvc service.SearchService,
	repoSvc service.RepoService,
	guideSvc service.GuideService,
	chatSvc service.ChatService,
	repoRepository service.RepoRepository,
	metadataEmbedder service.EmbeddingClient,
	codeEmbedder service.EmbeddingClient,
	codeSvc service.CodeService,
) {

	v1 := app.Group("/api/v1")
	NewSearchHandler(searchSvc).Register(v1)
	NewRepoHandler(repoSvc).Register(v1)
	NewGuideHandler(guideSvc).Register(v1)
	NewChatHandler(chatSvc).Register(v1)
	NewCodeSearchHandler(repoRepository, codeEmbedder, codeSvc).Register(v1)
}
