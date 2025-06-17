package handler

import (
	"github.com/ahmednasr/ai-in-action/server/internal/service"

	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type CodeSearchHandler struct {
	repoRepo service.RepoRepository
	embedder service.EmbeddingClient
	codeSvc  service.CodeService
}

func NewCodeSearchHandler(repoRepo service.RepoRepository, embedder service.EmbeddingClient, codeSvc service.CodeService) *CodeSearchHandler {
	return &CodeSearchHandler{
		repoRepo: repoRepo,
		embedder: embedder,
		codeSvc:  codeSvc,
	}
}

func (h *CodeSearchHandler) Register(r fiber.Router) {
	r.Post("/code_search", h.codeSearch)
	r.Get("/file/:repo_id/*", h.getFile)
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

// getFile handles GET /file/:repo_id/*
func (h *CodeSearchHandler) getFile(c *fiber.Ctx) error {
	repoID := c.Params("repo_id")
	filePath := c.Params("*") // This captures everything after /file/:repo_id/

	log.Printf("Received file request - RepoID: %s, FilePath: %s", repoID, filePath)

	if repoID == "" || filePath == "" {
		log.Printf("Invalid request - missing repo_id or file path")
		return fiber.NewError(fiber.StatusBadRequest, "repo_id and file path are required")
	}

	// Remove any duplicate repo_id from the file path
	// The file path might contain the repo_id at the start (e.g., "vuejs/vue/path/to/file")
	// We want to remove it if it matches the repo_id
	parts := strings.Split(filePath, "/")
	if len(parts) >= 2 && parts[0]+"/"+parts[1] == repoID {
		filePath = strings.Join(parts[2:], "/")
		log.Printf("Removed duplicate repo_id from file path. New path: %s", filePath)
	}

	content, err := h.codeSvc.GetFileContent(c.UserContext(), repoID, filePath)
	if err != nil {
		log.Printf("Error fetching file content - RepoID: %s, FilePath: %s, Error: %v", repoID, filePath, err)
		return fiber.NewError(fiber.StatusInternalServerError, "failed to get file content: "+err.Error())
	}

	log.Printf("Successfully retrieved file content - RepoID: %s, FilePath: %s", repoID, filePath)
	return c.JSON(fiber.Map{
		"content": content,
		"repo_id": repoID,
		"file":    filePath,
	})
}
