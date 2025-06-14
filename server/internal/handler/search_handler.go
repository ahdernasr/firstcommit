package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"ai-in-action/internal/models"
	"ai-in-action/internal/service"
)

// SearchHandler wires HTTP → SearchService.
type SearchHandler struct {
	svc service.SearchService
}

// NewSearchHandler returns a handler instance.
func NewSearchHandler(svc service.SearchService) *SearchHandler {
	return &SearchHandler{svc: svc}
}

// Register mounts GET /search on the given router group.
func (h *SearchHandler) Register(r fiber.Router) {
	r.Get("/search", h.search)
}

// search handles GET /search?q=some+text&k=10
func (h *SearchHandler) search(c *fiber.Ctx) error {
	q := c.Query("q")
	if q == "" {
		return fiber.NewError(fiber.StatusBadRequest, "q (query) parameter is required")
	}

	kParam := c.Query("k", "10")
	k, err := strconv.Atoi(kParam)
	if err != nil || k <= 0 {
		return fiber.NewError(fiber.StatusBadRequest, "k must be a positive integer")
	}

	req := models.SearchRequest{
		Query: q,
		TopK:  k,
	}

	results, err := h.svc.Search(c.UserContext(), req.Query, req.TopK)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(results)
}
