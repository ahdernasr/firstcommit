package service

import (
	"context"
	"time"

	"ai-in-action/internal/github"
	"ai-in-action/internal/models"
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
	FindByID(ctx context.Context, repoID string) (models.Repo, error)
	GetTopContextChunks(ctx context.Context, repoID string, queryVec []float32, k int) ([]string, error)
}

// ---- Service implementation ------------------------------------------------

// GuideService generates or retrieves an AI guide for a GitHub issue.
type GuideService interface {
	GetGuide(ctx context.Context, issueID string) (models.Guide, error)
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
	// 1. Check cache.
	guide, err := s.guideRepo.FindByIssueID(ctx, issueID)
	if err == nil && guide.ID != "" {
		return guide, nil
	}

	// 2. Fetch issue info from GitHub.
	owner, repo, num := parseIssueID(issueID) // helper splits "owner/repo#123"
	issue, err := s.gh.GetIssue(owner, repo, num)
	if err != nil {
		return models.Guide{}, err
	}

	// 3. Embed the issue title/body.
	query := issue.Title + "\n\n" + issue.Body
	qVec, err := s.embedder.Embed(query)
	if err != nil {
		return models.Guide{}, err
	}

	// 4. Retrieve top‑k context chunks (code, README) from Mongo vector index.
	repoDoc, err := s.repoRepo.FindByID(ctx, repo)
	if err != nil {
		return models.Guide{}, err
	}
	chunks, err := s.repoRepo.GetTopContextChunks(ctx, repoDoc.ID.Hex(), qVec, 20)
	if err != nil {
		return models.Guide{}, err
	}

	// 5. Run local LLM with RAG prompt.
	answer, err := s.llm.GenerateGuide(issue, chunks)
	if err != nil {
		return models.Guide{}, err
	}

	// 6. Persist guide.
	guide = models.Guide{
		ID:        issueID,
		Answer:    answer,
		Issue:     issue,
		CreatedAt: time.Now(),
	}
	if err := s.guideRepo.Upsert(ctx, guide); err != nil {
		return guide, err // guide still has value
	}

	return guide, nil
}

// ---- Helpers & local interfaces -------------------------------------------

// parseIssueID converts "owner/repo#123" → ("owner","repo",123)
func parseIssueID(id string) (owner, repo string, number int) {
	// NOTE: Implement robust parsing; simplified placeholder:
	// Expect format owner_repo_number (e.g., "torvalds_linux_123")
	// TODO: implement real parsing logic
	return "", "", 0
}

// EmbeddingClient abstracts your local embedding model.
type EmbeddingClient interface {
	Embed(text string) ([]float32, error)
}

// LLMClient abstracts the local LLM you’ll plug in.
type LLMClient interface {
	GenerateGuide(issue models.Issue, context []string) (string, error)
}
