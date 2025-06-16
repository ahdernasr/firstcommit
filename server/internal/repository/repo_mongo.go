package repository

import (
	"context"
	"fmt"
	"log"

	"github.com/ahmednasr/ai-in-action/server/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// RepoMongo implements the repository interface for MongoDB.
type RepoMongo struct {
	metaColl          *mongo.Collection // repos_meta collection from primary DB (for repository embeddings)
	codeColl          *mongo.Collection // repos_code collection from primary DB (for code chunks)
	federatedMetaColl *mongo.Collection // repos collection from federated DB (for full metadata)
}

// NewRepoRepository creates a new MongoDB repository instance.
func NewRepoRepository(primaryDB, federatedDB *mongo.Database) (*RepoMongo, error) {
	// Verify repos_meta collection exists in primaryDB
	collections, err := primaryDB.ListCollectionNames(context.Background(), bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to list collections in primaryDB: %w", err)
	}

	hasPrimaryMeta := false
	for _, coll := range collections {
		if coll == "repos_meta" {
			hasPrimaryMeta = true
		}
	}

	if !hasPrimaryMeta {
		log.Printf("Warning: repos_meta collection not found in primaryDB. Vector search may not work.")
	}

	// Verify repos_code collection exists in primaryDB
	collections, err = primaryDB.ListCollectionNames(context.Background(), bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to list collections in primaryDB: %w", err)
	}

	hasPrimaryCode := false
	for _, coll := range collections {
		if coll == "repos_code" {
			hasPrimaryCode = true
		}
	}

	if !hasPrimaryCode {
		log.Printf("Warning: repos_code collection not found in primaryDB. Code search may not work.")
	}

	// Verify repos collection exists in federatedDB
	collections, err = federatedDB.ListCollectionNames(context.Background(), bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to list collections in federatedDB: %w", err)
	}

	hasFederatedRepos := false
	for _, coll := range collections {
		if coll == "repos_meta" {
			hasFederatedRepos = true
		}
	}

	if !hasFederatedRepos {
		log.Printf("Warning: repos_meta collection not found in federatedDB. Full repository details may not be available.")
	}

	return &RepoMongo{
		metaColl:          primaryDB.Collection("repos_meta"),
		codeColl:          primaryDB.Collection("repos_code"),
		federatedMetaColl: federatedDB.Collection("repos_meta"),
	}, nil
}

// FindByID retrieves a repository by its ID.
func (r *RepoMongo) FindByID(ctx context.Context, id string) (*models.Repo, error) {
	filter := bson.M{"full_name": "vuejs/" + id}
	var repo models.Repo
	err := r.federatedMetaColl.FindOne(ctx, filter).Decode(&repo)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("repository with full_name '%s' not found", id)
		}
		return nil, fmt.Errorf("failed to find repository by full_name: %w", err)
	}
	return &repo, nil
}

// FindByName retrieves a single repository from the federated database by its name.
func (r *RepoMongo) FindByName(ctx context.Context, name string) (*models.Repo, error) {
	filter := bson.M{"name": name} // Search by 'name' field
	var repo models.Repo
	err := r.federatedMetaColl.FindOne(ctx, filter).Decode(&repo)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("repository with name '%s' not found", name)
		}
		return nil, fmt.Errorf("failed to find repository by name: %w", err)
	}
	return &repo, nil
}

// VectorSearch performs a vector similarity search on the repository embeddings.
func (r *RepoMongo) VectorSearch(ctx context.Context, queryVector []float32, k int) ([]models.Repo, error) {
	log.Printf("Building vector search pipeline with query vector length: %d", len(queryVector))

	// First, let's check what's in the primary meta collection (repos_meta)
	count, err := r.metaColl.CountDocuments(ctx, bson.M{})
	if err != nil {
		log.Printf("Error counting documents in primary meta collection: %v", err)
	} else {
		log.Printf("Found %d documents in primary meta collection", count)
	}

	// Sample a document to verify structure
	var sampleDoc struct {
		ID        string    `bson:"_id"`
		Embedding []float32 `bson:"embedding"`
	}
	err = r.metaColl.FindOne(ctx, bson.M{}).Decode(&sampleDoc)
	if err != nil {
		log.Printf("Error sampling document from primary meta collection: %v", err)
	} else {
		log.Printf("Sample document from primary meta collection: ID (Full Name)=%s, Embedding length=%d",
			sampleDoc.ID, len(sampleDoc.Embedding))
	}

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
				"_id":   1, // Project only _id
				"score": bson.M{"$meta": "vectorSearchScore"},
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

	var results []struct {
		ID    string  `bson:"_id"` // Decode _id
		Score float64 `bson:"score"`
	}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("vector search failed: failed to decode results: %w", err)
	}

	log.Printf("Vector search returned %d initial results", len(results))
	if len(results) > 0 {
		log.Printf("First result: ID (Full Name)=%s, Score=%f",
			results[0].ID, results[0].Score)
	}

	// Now, for each result, fetch the full repository metadata from federated DB
	var enrichedResults []models.Repo
	for _, result := range results {
		log.Printf("Looking up metadata for full_name: %s", result.ID)

		// Use ID (which is the full_name) to fetch full metadata from federated DB
		fullRepo, err := r.FindByID(ctx, result.ID)
		if err != nil {
			log.Printf("Warning: Could not find full metadata for repo %s from federated DB: %v", result.ID, err)
			continue // Skip if full metadata not found
		}
		log.Printf("Found metadata for repo: %s (full_name: %s)", fullRepo.Name, fullRepo.FullName)
		fullRepo.Score = result.Score // Preserve the score from vector search
		enrichedResults = append(enrichedResults, *fullRepo)
	}

	log.Printf("Vector search returned %d enriched results", len(enrichedResults))

	if len(enrichedResults) > 0 {
		log.Printf("First enriched result score: %v", enrichedResults[0].Score)
		log.Printf("First enriched result name: %s", enrichedResults[0].Name)
	}
	return enrichedResults, nil
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
