package handler

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/mongo"
)

type HealthHandler struct {
	mainDB      *mongo.Client
	federatedDB *mongo.Client
}

func NewHealthHandler(mainDB, federatedDB *mongo.Client) *HealthHandler {
	return &HealthHandler{
		mainDB:      mainDB,
		federatedDB: federatedDB,
	}
}

func (h *HealthHandler) Register(r fiber.Router) {
	r.Get("/health", h.health)
}

func (h *HealthHandler) health(c *fiber.Ctx) error {
	status := fiber.Map{
		"status": "ok",
		"dbs": fiber.Map{
			"main":      h.checkDB(h.mainDB),
			"federated": h.checkDB(h.federatedDB),
		},
	}

	return c.JSON(status)
}

func (h *HealthHandler) checkDB(client *mongo.Client) string {
	if client == nil {
		return "not_configured"
	}

	ctx := context.Background()
	if err := client.Ping(ctx, nil); err != nil {
		return "error"
	}
	return "connected"
}
