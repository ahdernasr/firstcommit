package handler

import (
	"fmt"
	"log"

	"github.com/ahmednasr/ai-in-action/server/internal/service"
	"github.com/gofiber/fiber/v2"
)

type RAGHandler struct {
	ragService *service.RAGService
}

func NewRAGHandler(ragService *service.RAGService) *RAGHandler {
	return &RAGHandler{
		ragService: ragService,
	}
}

func (h *RAGHandler) RegisterRoutes(app *fiber.App) {
	app.Post("/api/v1/rag", h.HandleRAG)
}

func (h *RAGHandler) HandleRAG(c *fiber.Ctx) error {
	var req service.RAGRequest
	if err := c.BodyParser(&req); err != nil {
		log.Printf("Failed to parse request body: %v", err)
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	log.Printf("Received RAG request: %+v", req)

	if req.Query == "" {
		log.Printf("Empty query received")
		return fiber.NewError(fiber.StatusBadRequest, "Query cannot be empty")
	}

	resp, err := h.ragService.GenerateResponse(c.Context(), req)
	if err != nil {
		log.Printf("Error generating response: %v", err)
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("Error generating response: %v", err))
	}

	log.Printf("Generated response: %+v", resp)
	return c.JSON(resp)
}
