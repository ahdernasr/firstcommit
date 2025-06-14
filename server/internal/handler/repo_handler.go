package handler

import (
	"github.com/gofiber/fiber/v2"

	"ai-in-action/internal/service"
)

// RepoHandler wires HTTP → RepoService.
type RepoHandler struct {
	svc service.RepoService
}

// NewRepoHandler creates a new RepoHandler.
func NewRepoHandler(svc service.RepoService) *RepoHandler {
	return &RepoHandler{svc: svc}
}

// Register mounts GET /repos/:id on the supplied router group.
func (h *RepoHandler) Register(r fiber.Router) {
	r.Get("/repos/:id", h.getRepo)
}

// getRepo handles GET /repos/:id
func (h *RepoHandler) getRepo(c *fiber.Ctx) error {
	repoID := c.Params("id")
	if repoID == "" {
		return fiber.NewError(fiber.StatusBadRequest, "repo id is required")
	}

	detail, err := h.svc.GetRepo(c.UserContext(), repoID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(detail)
}
