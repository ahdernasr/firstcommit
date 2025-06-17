package service

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/ahmednasr/ai-in-action/server/internal/github"
	"github.com/ahmednasr/ai-in-action/server/internal/models"
)

// ---- Repository layer contracts -------------------------------------------

// GuideRepository handles persistence of AI‑generated guides & chat history.
type GuideRepository interface {
	FindByIssueID(ctx context.Context, issueID string) (models.Guide, error)
	Upsert(ctx context.Context, g models.Guide) error
}

// ---- Repository contract ---------------------------------------------------

// RepoRepository lets us pull README / code snippets vectors for RAG.
type RepoRepository interface {
	FindByID(ctx context.Context, repoID string) (*models.Repo, error)
	GetTopContextChunks(ctx context.Context, repoID string, k int) ([]models.CodeChunk, error)
	CodeVectorSearch(ctx context.Context, repoID string, queryVec []float32, k int) ([]models.CodeChunk, error)
	GetFileContent(ctx context.Context, repoID string, filePath string) (string, error)
}

// ---- Service implementation ------------------------------------------------

// GuideService generates or retrieves an AI guide for a GitHub issue.
type GuideService interface {
	GetGuide(ctx context.Context, issueID string) (models.Guide, error)
	Upsert(ctx context.Context, guide models.Guide) error
}

type guideService struct {
	guideRepo GuideRepository
	repoRepo  RepoRepository
	gh        *github.Client
	embedder  EmbeddingClient // local model for generating embeddings
	llm       LLMClient       // local LLM for generation
}

// NewGuideService wires dependencies.
func NewGuideService(
	guideRepo GuideRepository,
	gh *github.Client,
	repoRepo RepoRepository,
	embedder EmbeddingClient,
	llm LLMClient,
) GuideService {
	return &guideService{
		guideRepo: guideRepo,
		repoRepo:  repoRepo,
		gh:        gh,
		embedder:  embedder,
		llm:       llm,
	}
}

// GetGuide returns a cached guide or generates a new one via RAG.
func (s *guideService) GetGuide(ctx context.Context, issueID string) (models.Guide, error) {
	log.Printf("[Guide Service] Getting guide for issue: %s", issueID)

	// Split the issue ID into repo and number parts
	parts := strings.Split(issueID, "#")
	if len(parts) != 2 {
		log.Printf("[Guide Service] Invalid issue ID format (expected owner/repo#number): %s", issueID)
		return models.Guide{}, fmt.Errorf("invalid issue ID format")
	}

	repoPart := parts[0]
	numberPart := parts[1]

	// Create the cache key using the repo and issue number
	cacheKey := fmt.Sprintf("%s#%s", repoPart, numberPart)
	log.Printf("[Guide Service] Looking up guide with cache key: %s", cacheKey)

	// 1. Check cache.
	guide, err := s.guideRepo.FindByIssueID(ctx, cacheKey)
	if err == nil && guide.ID != "" {
		log.Printf("[Guide Service] Found cached guide for issue: %s", cacheKey)
		return guide, nil
	}
	log.Printf("[Guide Service] No cached guide found for issue: %s", cacheKey)

	// 2. Fetch issue info from GitHub.
	repoParts := strings.Split(repoPart, "/")
	if len(repoParts) != 2 {
		log.Printf("[Guide Service] Invalid repo format in ID %s: %s", issueID, repoPart)
		return models.Guide{}, fmt.Errorf("invalid repo format")
	}

	owner, repo := repoParts[0], repoParts[1]
	num, err := strconv.Atoi(numberPart)
	if err != nil {
		log.Printf("[Guide Service] Invalid issue number in ID %s: %v", issueID, err)
		return models.Guide{}, fmt.Errorf("invalid issue number: %w", err)
	}

	log.Printf("[Guide Service] Fetching issue info from GitHub: owner=%s, repo=%s, number=%d", owner, repo, num)
	issue, err := s.gh.GetIssue(owner, repo, num)
	if err != nil {
		log.Printf("[Guide Service] Error fetching issue from GitHub: %v", err)
		return models.Guide{}, err
	}
	log.Printf("[Guide Service] Successfully fetched issue from GitHub")

	// 3. Retrieve top‑k context chunks (code, README) from Mongo vector index.
	repoDoc, err := s.repoRepo.FindByID(ctx, repo)
	if err != nil {
		log.Printf("[Guide Service] Error finding repo document: %v", err)
		return models.Guide{}, err
	}
	log.Printf("[Guide Service] Found repo document: %s", repoDoc.ID)

	chunks, err := s.repoRepo.GetTopContextChunks(ctx, repoDoc.ID, 20)
	if err != nil {
		log.Printf("[Guide Service] Error getting context chunks: %v", err)
		return models.Guide{}, err
	}
	log.Printf("[Guide Service] Retrieved %d context chunks", len(chunks))

	// Convert CodeChunks to strings for the LLM
	chunkTexts := make([]string, len(chunks))
	for i, chunk := range chunks {
		chunkTexts[i] = chunk.Text
	}

	// 4. Run local LLM with RAG prompt.
	log.Printf("[Guide Service] Generating guide using LLM")
	answer, err := s.llm.GenerateGuide(issue, chunkTexts)
	if err != nil {
		log.Printf("[Guide Service] Error generating guide with LLM: %v", err)
		return models.Guide{}, err
	}
	log.Printf("[Guide Service] Successfully generated guide with LLM")
	log.Printf("[Guide Service] Generated guide length: %d", len(answer))

	// 5. Persist guide.
	guide = models.Guide{
		ID:        issueID,
		Answer:    answer,
		Issue:     issue,
		CreatedAt: time.Now(),
	}
	log.Printf("[Guide Service] Attempting to persist guide to MongoDB")
	log.Printf("[Guide Service] Guide ID: %s", guide.ID)
	log.Printf("[Guide Service] Guide content length: %d", len(guide.Answer))

	if err := s.guideRepo.Upsert(ctx, guide); err != nil {
		log.Printf("[Guide Service] Error persisting guide to MongoDB: %v", err)
		return guide, err // guide still has value
	}
	log.Printf("[Guide Service] Successfully persisted guide to MongoDB")

	return guide, nil
}

// Upsert inserts or replaces a guide in the repository.
func (s *guideService) Upsert(ctx context.Context, guide models.Guide) error {
	log.Printf("[Guide Service] Upserting guide for issue: %s", guide.ID)
	return s.guideRepo.Upsert(ctx, guide)
}

// ---- Helpers & local interfaces -------------------------------------------

// EmbeddingClient abstracts your local embedding model.
type EmbeddingClient interface {
	Embed(text string) ([]float32, error)
}

// LLMClient abstracts the local LLM you'll plug in.
type LLMClient interface {
	GenerateGuide(issue models.Issue, context []string) (string, error)
}
