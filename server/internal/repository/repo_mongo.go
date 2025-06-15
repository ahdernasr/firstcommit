package repository

import (
	"context"
	"fmt"
	"log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"ai-in-action/internal/models"
)

// RepoMongo implements the repository interface for MongoDB.
type RepoMongo struct {
	metaColl *mongo.Collection // repos_meta collection
	codeColl *mongo.Collection // repos_code collection
}

// NewRepoRepository creates a new MongoDB repository instance.
func NewRepoRepository(db *mongo.Database) (*RepoMongo, error) {
	// Verify collections exist
	collections, err := db.ListCollectionNames(context.Background(), bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}

	hasMeta := false
	hasCode := false
	for _, coll := range collections {
		if coll == "repos_meta" {
			hasMeta = true
		}
		if coll == "repos_code" {
			hasCode = true
		}
	}

	if !hasMeta {
		log.Printf("Warning: repos_meta collection not found")
	}
	if !hasCode {
		log.Printf("Warning: repos_code collection not found")
	}

	return &RepoMongo{
		metaColl: db.Collection("repos_meta"),
		codeColl: db.Collection("repos_code"),
	}, nil
}

// FindByID retrieves a repository by its ID.
func (r *RepoMongo) FindByID(ctx context.Context, id string) (*models.Repo, error) {
	var repo models.Repo
	err := r.metaColl.FindOne(ctx, bson.M{"_id": id}).Decode(&repo)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("repository not found: %s", id)
		}
		return nil, fmt.Errorf("failed to find repository: %w", err)
	}
	return &repo, nil
}

// VectorSearch performs a vector similarity search on the repository embeddings.
func (r *RepoMongo) VectorSearch(ctx context.Context, queryVector []float32, k int) ([]models.Repo, error) {
	log.Printf("Building vector search pipeline with query vector length: %d", len(queryVector))

	pipeline := mongo.Pipeline{
		{
			{"$vectorSearch", bson.M{
				"index":         "vector_index",
				"path":          "embedding",
				"queryVector":   queryVector,
				"numCandidates": k * 10,
				"limit":         k,
				"similarity":    "cosine",
			}},
		},
		{
			{"$project", bson.M{
				"_id":         1,
				"owner":       1,
				"name":        1,
				"full_name":   1,
				"description": 1,
				"stars":       1,
				"languages":   1,
				"image_url":   1,
				"score":       bson.M{"$meta": "vectorSearchScore"},
			}},
		},
		{
			{"$sort", bson.M{"score": -1}},
		},
	}

	log.Printf("Executing vector search pipeline")
	cursor, err := r.metaColl.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}
	defer cursor.Close(ctx)

	var results []models.Repo
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("vector search failed: failed to decode results: %w", err)
	}

	log.Printf("Vector search returned %d results", len(results))
	if len(results) > 0 {
		log.Printf("First result score: %v", results[0].Score)
	}
	return results, nil
}

// CodeVectorSearch performs a vector similarity search on code chunks.
func (r *RepoMongo) CodeVectorSearch(ctx context.Context, repoID string, queryVector []float32, k int) ([]models.CodeChunk, error) {
	log.Printf("Building code vector search pipeline for repo %s with query vector length: %d", repoID, len(queryVector))

	pipeline := mongo.Pipeline{
		{
			{"$vectorSearch", bson.M{
				"index":         "vector_index",
				"path":          "embedding",
				"queryVector":   queryVector,
				"numCandidates": k * 10,
				"limit":         k,
				"similarity":    "cosine",
				"filter":        bson.M{"repo_id": repoID},
			}},
		},
		{
			{"$project", bson.M{
				"_id":     1,
				"repo_id": 1,
				"text":    1,
				"file":    1,
				"score":   bson.M{"$meta": "vectorSearchScore"},
			}},
		},
		{
			{"$sort", bson.M{"score": -1}},
		},
	}

	log.Printf("Executing code vector search pipeline for repo %s", repoID)
	cursor, err := r.codeColl.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("code vector search failed: %w", err)
	}
	defer cursor.Close(ctx)

	var results []models.CodeChunk
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("code vector search failed: failed to decode results: %w", err)
	}

	log.Printf("Code vector search returned %d results for repo %s", len(results), repoID)
	return results, nil
}

// GetTopContextChunks retrieves the most relevant code chunks for a repository.
func (r *RepoMongo) GetTopContextChunks(ctx context.Context, repoID string, k int) ([]models.CodeChunk, error) {
	opts := options.Find().
		SetSort(bson.M{"score": -1}).
		SetLimit(int64(k))

	cursor, err := r.codeColl.Find(ctx, bson.M{"repo_id": repoID}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find code chunks: %w", err)
	}
	defer cursor.Close(ctx)

	var chunks []models.CodeChunk
	if err := cursor.All(ctx, &chunks); err != nil {
		return nil, fmt.Errorf("failed to decode code chunks: %w", err)
	}
	return chunks, nil
}
