package repository

import (
	"context"
	"log"

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
	log.Printf("[Guide Repository] Finding guide by issue ID: %s", issueID)
	var g models.Guide
	err := r.col.FindOne(ctx, bson.M{"_id": issueID}).Decode(&g)
	if err == mongo.ErrNoDocuments {
		log.Printf("[Guide Repository] No guide found for issue ID: %s", issueID)
		return models.Guide{}, nil
	}
	if err != nil {
		log.Printf("[Guide Repository] Error finding guide by issue ID %s: %v", issueID, err)
		return models.Guide{}, err
	}
	log.Printf("[Guide Repository] Found guide for issue ID: %s", issueID)
	return g, err
}

// Upsert inserts or replaces the guide with the same _id.
func (r *GuideRepository) Upsert(ctx context.Context, g models.Guide) error {
	log.Printf("[Guide Repository] Upserting guide for issue ID: %s", g.ID)
	log.Printf("[Guide Repository] Guide content length: %d", len(g.Answer))

	// Log the MongoDB operation details
	log.Printf("[Guide Repository] Collection name: %s", r.col.Name())
	log.Printf("[Guide Repository] Database name: %s", r.col.Database().Name())

	_, err := r.col.ReplaceOne(
		ctx,
		bson.M{"_id": g.ID},
		g,
		options.Replace().SetUpsert(true),
	)
	if err != nil {
		log.Printf("[Guide Repository] Error upserting guide for issue ID %s: %v", g.ID, err)
		return err
	}
	log.Printf("[Guide Repository] Successfully upserted guide for issue ID: %s", g.ID)
	return err
}
