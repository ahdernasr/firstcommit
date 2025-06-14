package handler

import (
	"github.com/gofiber/fiber/v2"

	"ai-in-action/internal/service"
)

// GuideHandler wires HTTP → GuideService.
type GuideHandler struct {
	svc service.GuideService
}

// NewGuideHandler creates a GuideHandler instance.
func NewGuideHandler(svc service.GuideService) *GuideHandler {
	return &GuideHandler{svc: svc}
}

// Register mounts GET /issues/:id/guide on the given router group.
func (h *GuideHandler) Register(r fiber.Router) {
	r.Get("/issues/:id/guide", h.getGuide)
}

// getGuide handles GET /issues/:id/guide
func (h *GuideHandler) getGuide(c *fiber.Ctx) error {
	issueID := c.Params("id")
	if issueID == "" {
		return fiber.NewError(fiber.StatusBadRequest, "issue id is required")
	}

	guide, err := h.svc.GetGuide(c.UserContext(), issueID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(guide)
}
