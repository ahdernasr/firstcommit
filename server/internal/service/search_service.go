package service

import (
	"context"

	"ai-in-action/internal/models"
)

// ---- Repository contract ---------------------------------------------------

// SearchRepoRepository exposes vector search over the repo embeddings.
type SearchRepoRepository interface {
	// VectorSearch returns the top‑k repositories whose stored embedding is
	// most similar to queryVec. The implementation typically uses
	// MongoDB Atlas Vector Search.
	VectorSearch(ctx context.Context, queryVec []float32, k int) ([]models.Repo, error)
}

// ---- Service interface + implementation ------------------------------------

// SearchService converts a natural‑language query into an embedding and then
// performs K‑NN search through the repository vector index.
type SearchService interface {
	Search(ctx context.Context, query string, k int) ([]models.Repo, error)
}

type searchService struct {
	repoRepo SearchRepoRepository
	embedder EmbeddingClient
}

// NewSearchService wires dependencies and returns SearchService.
func NewSearchService(repoRepo SearchRepoRepository, embedder EmbeddingClient) SearchService {
	return &searchService{
		repoRepo: repoRepo,
		embedder: embedder,
	}
}

// Search embeds the query string, performs a vector search, and returns results.
func (s *searchService) Search(ctx context.Context, query string, k int) ([]models.Repo, error) {
	vec, err := s.embedder.Embed(query)
	if err != nil {
		return nil, err
	}

	repos, err := s.repoRepo.VectorSearch(ctx, vec, k)
	if err != nil {
		return nil, err
	}

	return repos, nil
}
