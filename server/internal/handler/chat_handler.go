package handler

import (
	"ai-in-action/internal/models"
	"ai-in-action/internal/service"

	"github.com/gofiber/fiber/v2"
)

// ChatHandler wires HTTP → ChatService.
type ChatHandler struct {
	svc service.ChatService
}

// NewChatHandler returns a struct pointer so you can call Register on it.
func NewChatHandler(svc service.ChatService) *ChatHandler {
	return &ChatHandler{svc: svc}
}

// Register mounts the /chat endpoint on the supplied router group.
func (h *ChatHandler) Register(r fiber.Router) {
	r.Post("/chat", h.chat)
}

// chat handles POST /chat  { "question": "...", "context_id": "..." }
func (h *ChatHandler) chat(c *fiber.Ctx) error {
	var req models.ChatRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	if req.Question == "" {
		return fiber.NewError(fiber.StatusBadRequest, "question is required")
	}

	// Delegate to service layer.
	answer, err := h.svc.Ask(c.UserContext(), req.ContextID, req.Question)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(fiber.Map{
		"answer":     answer,
		"context_id": req.ContextID,
	})
}
