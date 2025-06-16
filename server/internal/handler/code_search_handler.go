package handler

import (
	"github.com/ahmednasr/ai-in-action/server/internal/service"

	"github.com/gofiber/fiber/v2"
)

type CodeSearchHandler struct {
	repoRepo service.RepoRepository
	embedder service.EmbeddingClient
}

func NewCodeSearchHandler(repoRepo service.RepoRepository, embedder service.EmbeddingClient) *CodeSearchHandler {
	return &CodeSearchHandler{repoRepo: repoRepo, embedder: embedder}
}

func (h *CodeSearchHandler) Register(r fiber.Router) {
	r.Post("/code_search", h.codeSearch)
}

type codeSearchRequest struct {
	RepoID string `json:"repo_id"`
	Query  string `json:"query"`
}

func (h *CodeSearchHandler) codeSearch(c *fiber.Ctx) error {
	var req codeSearchRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	if req.RepoID == "" || req.Query == "" {
		return fiber.NewError(fiber.StatusBadRequest, "repo_id and query are required")
	}

	embedding, err := h.embedder.Embed(req.Query)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "embedding failed: "+err.Error())
	}

	chunks, err := h.repoRepo.CodeVectorSearch(c.UserContext(), req.RepoID, embedding, 5)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "vector search failed: "+err.Error())
	}

	return c.JSON(chunks)
}
