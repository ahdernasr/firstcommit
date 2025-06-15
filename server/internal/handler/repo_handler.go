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

// Register mounts GET /repos/:id and GET /repos/:owner/:name/issues on the supplied router group.
func (h *RepoHandler) Register(r fiber.Router) {
	r.Get("/repos/:id", h.getRepo)
	r.Get("/repos/:owner/:name/issues", h.getIssues)
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

// getIssues handles GET /repos/:owner/:name/issues
func (h *RepoHandler) getIssues(c *fiber.Ctx) error {
	owner := c.Params("owner")
	repoName := c.Params("name")

	if owner == "" || repoName == "" {
		return fiber.NewError(fiber.StatusBadRequest, "owner and repository name are required")
	}

	issues, err := h.svc.ListRepoIssues(c.UserContext(), owner, repoName, "open", 100) // Default to open issues, 100 per page
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(issues)
}
