package service

import (
	"context"
	"fmt"
	"log"

	"github.com/ahmednasr/ai-in-action/server/internal/models"
)

// ---- Repository contract ---------------------------------------------------

// SearchRepoRepository exposes vector search over the repo embeddings.
type SearchRepoRepository interface {
	// VectorSearch returns the top‑k repositories whose stored embedding is
	// most similar to queryVec. The implementation typically uses
	// MongoDB Atlas Vector Search.
	VectorSearch(ctx context.Context, queryVec []float32, k int) ([]models.Repo, error)
	GetAllRepos(ctx context.Context) ([]models.Repo, error)
}

// ---- Service interface + implementation ------------------------------------

// SearchService converts natural‑language queries into embeddings and performs
// K‑NN searches through the repository vector index.
type SearchService interface {
	Search(query string) ([]models.Repo, error)
	GetAllRepos() ([]models.Repo, error)
}

type searchService struct {
	repo     SearchRepoRepository
	embedder EmbeddingClient
}

// NewSearchService wires the repository and embedder.
func NewSearchService(repo SearchRepoRepository, embedder EmbeddingClient) SearchService {
	return &searchService{
		repo:     repo,
		embedder: embedder,
	}
}

// Search embeds the query string and calls the repository's VectorSearch method.
func (s *searchService) Search(query string) ([]models.Repo, error) {
	ctx := context.Background()
	log.Printf("Starting search for query: %q", query)

	// Generate embedding
	log.Printf("Generating embedding for query...")
	vec, err := s.embedder.Embed(query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}
	log.Printf("Generated embedding vector of length %d", len(vec))
	log.Printf("First few values of embedding: %v", vec[:5])

	// Search repositories
	log.Printf("Performing vector search with k=30...")
	repos, err := s.repo.VectorSearch(ctx, vec, 30)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}
	log.Printf("Vector search returned %d results", len(repos))

	if len(repos) == 0 {
		log.Printf("No repositories found for query: %q", query)
		return []models.Repo{}, nil
	}

	// Log results for debugging
	for i, repo := range repos {
		log.Printf("Result #%d: %s (score: %.4f)", i+1, repo.ID, repo.Score)
	}

	return repos, nil
}

// GetAllRepos retrieves all repositories from the federated database.
func (s *searchService) GetAllRepos() ([]models.Repo, error) {
	ctx := context.Background()
	repos, err := s.repo.GetAllRepos(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get all repos: %w", err)
	}
	return repos, nil
}
