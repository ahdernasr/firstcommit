package handler

import (
	"github.com/ahmednasr/ai-in-action/server/internal/service"
	"github.com/gofiber/fiber/v2"
)

// SearchHandler exposes the search API.
type SearchHandler struct {
	svc service.SearchService
}

// NewSearchHandler wires the service.
func NewSearchHandler(svc service.SearchService) *SearchHandler {
	return &SearchHandler{svc: svc}
}

// Register mounts the search routes.
func (h *SearchHandler) Register(r fiber.Router) {
	r.Get("/search", h.search)
	r.Get("/repos", h.getAllRepos)
}

// search handles GET /api/v1/search?q=query
func (h *SearchHandler) search(c *fiber.Ctx) error {
	query := c.Query("q")
	if query == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "missing query parameter 'q'",
		})
	}

	repos, err := h.svc.Search(query)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"repositories": repos,
	})
}

// getAllRepos handles GET /api/v1/repos
func (h *SearchHandler) getAllRepos(c *fiber.Ctx) error {
	repos, err := h.svc.GetAllRepos()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"repositories": repos,
	})
}
