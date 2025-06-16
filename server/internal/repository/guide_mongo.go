package repository

import (
	"context"

	"github.com/ahmednasr/ai-in-action/server/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GuideRepository provides Mongo-backed persistence for AI-generated guides.
type GuideRepository struct {
	col *mongo.Collection
}

// NewGuideRepository returns a GuideRepository that operates on the "guides" collection.
func NewGuideRepository(db *mongo.Database) *GuideRepository {
	return &GuideRepository{
		col: db.Collection("guides"),
	}
}

// FindByIssueID returns a guide by its issueID ("owner/repo#123").
// When the document is not found, it returns an empty Guide and a nil error
// so callers can decide to regenerate the guide.
func (r *GuideRepository) FindByIssueID(ctx context.Context, issueID string) (models.Guide, error) {
	var g models.Guide
	err := r.col.FindOne(ctx, bson.M{"_id": issueID}).Decode(&g)
	if err == mongo.ErrNoDocuments {
		return models.Guide{}, nil
	}
	return g, err
}

// Upsert inserts or replaces the guide with the same _id.
func (r *GuideRepository) Upsert(ctx context.Context, g models.Guide) error {
	_, err := r.col.ReplaceOne(
		ctx,
		bson.M{"_id": g.ID},
		g,
		options.Replace().SetUpsert(true),
	)
	return err
}
