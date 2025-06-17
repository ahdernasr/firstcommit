package repository

import (
	"context"
	"fmt"
	"io"
	"log"
	"sort"
	"strings"
	"sync"

	"cloud.google.com/go/storage"
	"github.com/ahmednasr/ai-in-action/server/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type vectorSearchResult struct {
	ID              string   `bson:"_id"`
	Name            string   `bson:"name"`
	Description     string   `bson:"description"`
	StargazersCount int      `bson:"stargazers_count"`
	ForksCount      int      `bson:"forks_count"`
	Topics          []string `bson:"topics"`
	Languages       []string `bson:"languages"`
	Score           float64  `bson:"score"`
	RelevanceScore  float64  `bson:"relevance_score"`
}

// RepoMongo implements the repository interface for MongoDB.
type RepoMongo struct {
	metaColl          *mongo.Collection // repos_meta collection from primary DB (for repository embeddings)
	codeColl          *mongo.Collection // repos_code collection from primary DB (for code chunks)
	federatedMetaColl *mongo.Collection // repos collection from federated DB (for full metadata)
	storageClient     *storage.Client
}

// NewRepoRepository creates a new MongoDB repository instance.
func NewRepoRepository(primaryDB, federatedDB *mongo.Database, storageClient *storage.Client) (*RepoMongo, error) {
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
		storageClient:     storageClient,
	}, nil
}

// FindByID retrieves a repository by its ID.
func (r *RepoMongo) FindByID(ctx context.Context, id string) (*models.Repo, error) {
	filter := bson.M{"full_name": id}
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

	// Enhanced pipeline with hybrid search capabilities
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
				"_id":              1,
				"name":             1,
				"description":      1,
				"stargazers_count": 1,
				"forks_count":      1,
				"topics":           1,
				"languages":        1,
				"score":            bson.M{"$meta": "vectorSearchScore"},
				// Add relevance score calculation
				"relevance_score": bson.M{
					"$add": []interface{}{
						bson.M{"$multiply": []interface{}{bson.M{"$meta": "vectorSearchScore"}, 0.7}},
						bson.M{"$multiply": []interface{}{bson.M{"$divide": []interface{}{"$stargazers_count", 1000}}, 0.2}},
						bson.M{"$multiply": []interface{}{bson.M{"$divide": []interface{}{"$forks_count", 100}}, 0.1}},
					},
				},
			}},
		},
		{
			{"$sort", bson.M{"relevance_score": -1}},
		},
	}

	log.Printf("Executing vector search pipeline")
	cursor, err := r.metaColl.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}
	defer cursor.Close(ctx)

	var results []vectorSearchResult
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("vector search failed: failed to decode results: %w", err)
	}

	log.Printf("Vector search returned %d initial results", len(results))
	if len(results) > 0 {
		log.Printf("First result: ID (Full Name)=%s, Score=%f, Relevance Score=%f",
			results[0].ID, results[0].Score, results[0].RelevanceScore)
	}

	type repoWithIndex struct {
		index int
		repo  models.Repo
	}
	var (
		enriched  []repoWithIndex
		mu        sync.Mutex
		wg        sync.WaitGroup
		semaphore = make(chan struct{}, 10)
	)

	for i, result := range results {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(i int, result vectorSearchResult) {
			defer wg.Done()
			defer func() { <-semaphore }()

			log.Printf("Looking up metadata for full_name: %s", result.ID)
			fullRepo, err := r.FindByID(ctx, result.ID)
			if err != nil {
				log.Printf("Warning: Could not find full metadata for repo %s from federated DB: %v", result.ID, err)
				return
			}
			fullRepo.Score = result.Score

			mu.Lock()
			enriched = append(enriched, repoWithIndex{i, *fullRepo})
			mu.Unlock()

			log.Printf("Found metadata for repo: %s (full_name: %s)", fullRepo.Name, fullRepo.FullName)
		}(i, result)
	}

	wg.Wait()

	sort.Slice(enriched, func(i, j int) bool {
		return enriched[i].repo.Score > enriched[j].repo.Score
	})

	finalResults := make([]models.Repo, len(enriched))
	for i, r := range enriched {
		finalResults[i] = r.repo
	}

	log.Printf("Vector search returned %d enriched results", len(finalResults))
	if len(finalResults) > 0 {
		log.Printf("First enriched result score: %v", finalResults[0].Score)
		log.Printf("First enriched result name: %s", finalResults[0].Name)
	}

	// Log all results with their scores
	for i, repo := range finalResults {
		log.Printf("Result #%d: %s (score: %.4f)", i+1, repo.Name, repo.Score)
	}

	return finalResults, nil
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

	log.Printf("Code vector search returned %d initial results for repo %s", len(results), repoID)

	type chunkWithIndex struct {
		index int
		chunk models.CodeChunk
	}
	var (
		enriched  []chunkWithIndex
		mu        sync.Mutex
		wg        sync.WaitGroup
		semaphore = make(chan struct{}, 10)
	)

	for i, result := range results {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(i int, chunk models.CodeChunk) {
			defer wg.Done()
			defer func() { <-semaphore }()

			mu.Lock()
			enriched = append(enriched, chunkWithIndex{i, chunk})
			mu.Unlock()
		}(i, result)
	}

	wg.Wait()

	sort.Slice(enriched, func(i, j int) bool {
		return enriched[i].chunk.Score > enriched[j].chunk.Score
	})

	finalResults := make([]models.CodeChunk, len(enriched))
	for i, c := range enriched {
		finalResults[i] = c.chunk
	}

	log.Printf("Code vector search returned %d enriched results for repo %s", len(finalResults), repoID)
	if len(finalResults) > 0 {
		log.Printf("First result score: %.4f", finalResults[0].Score)
	}

	// Log all results with their scores
	for i, chunk := range finalResults {
		log.Printf("Code Result #%d: %s (score: %.4f)", i+1, chunk.File, chunk.Score)
	}

	return finalResults, nil
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

// GetAllRepos retrieves all repositories from the federated database.
func (r *RepoMongo) GetAllRepos(ctx context.Context) ([]models.Repo, error) {
	cursor, err := r.federatedMetaColl.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to find repositories: %w", err)
	}
	defer cursor.Close(ctx)

	var repos []models.Repo
	if err := cursor.All(ctx, &repos); err != nil {
		return nil, fmt.Errorf("failed to decode repositories: %w", err)
	}
	return repos, nil
}

// GetFileContent retrieves the content of a file from the GCS bucket.
func (r *RepoMongo) GetFileContent(ctx context.Context, repoID string, filePath string) (string, error) {
	// Extract owner and repo name from the filePath
	parts := strings.SplitN(filePath, "/", 2)
	if len(parts) != 2 {
		log.Printf("Invalid file path format - FilePath: %s", filePath)
		return "", fmt.Errorf("invalid file path format: %s", filePath)
	}

	// Construct the normalized repoID (owner--repo)
	normalizedRepoID := fmt.Sprintf("%s--%s", repoID, parts[0])
	// Get the rest of the file path
	restOfPath := parts[1]

	// Construct the full GCS path
	fullPath := fmt.Sprintf("input/repos/%s/%s", normalizedRepoID, restOfPath)

	// Log the exact GCS path being accessed
	log.Printf("Accessing GCS bucket:\nBucket: ai-in-action-repo-bucket\nPath: %s", fullPath)

	// Get the object from GCS
	obj := r.storageClient.Bucket("ai-in-action-repo-bucket").Object(fullPath)
	reader, err := obj.NewReader(ctx)
	if err != nil {
		if err == storage.ErrObjectNotExist {
			log.Printf("File not found in GCS bucket - Path: %s", fullPath)
			return "", fmt.Errorf("file not found: %s in repo %s", filePath, repoID)
		}
		log.Printf("GCS error while reading file - Path: %s, Error: %v", fullPath, err)
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	defer reader.Close()

	// Read the content
	content, err := io.ReadAll(reader)
	if err != nil {
		log.Printf("Error reading file content - Path: %s, Error: %v", fullPath, err)
		return "", fmt.Errorf("failed to read file content: %w", err)
	}

	log.Printf("Successfully read file from GCS - Path: %s", fullPath)
	return string(content), nil
}
