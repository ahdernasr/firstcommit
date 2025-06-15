package service

import (
	"context"
	"strings"

	"ai-in-action/internal/github"
	"ai-in-action/internal/models"
)

// ---- Return DTO ------------------------------------------------------------

// RepoDetail combines dataset metadata with live GitHub issues.
type RepoSDetail struct {
	Repo   models.Repo    `json:"repo"`
	Issues []models.Issue `json:"issues"`
}

// ---- Service interface + implementation ------------------------------------

// RepoService enriches repository data with live GitHub information.
type RepoService interface {
	GetRepo(ctx context.Context, repoID string) (RepoSDetail, error)
	ListRepoIssues(ctx context.Context, owner, repoName, state string, perPage int) ([]models.Issue, error)
}

type repoService struct {
	repoRepo RepoRepository
	gh       *github.Client
}

// NewRepoService returns a concrete implementation.
func NewRepoService(repoRepo RepoRepository, gh *github.Client) RepoService {
	return &repoService{repoRepo: repoRepo, gh: gh}
}

// GetRepo fetches repository metadata from Mongo, then pulls live issues from GitHub.
func (s *repoService) GetRepo(ctx context.Context, repoID string) (RepoSDetail, error) {
	// 1. Fetch metadata document.
	repoDoc, err := s.repoRepo.FindByID(ctx, repoID)
	if err != nil {
		return RepoSDetail{}, err
	}

	// 2. Parse owner/name. The dataset stores them separately, but fallback to FullName.
	owner, name := repoDoc.Owner, repoDoc.Name
	if owner == "" || name == "" {
		if parts := strings.Split(repoDoc.FullName, "/"); len(parts) == 2 {
			owner, name = parts[0], parts[1]
		}
	}

	// 3. Pull open issues (limit 20) from GitHub.
	issues, err := s.gh.ListRepoIssues(owner, name, "open", 20)
	if err != nil {
		// Non-fatal: still return repo metadata even if GitHub call fails.
		return RepoSDetail{Repo: *repoDoc}, nil
	}

	return RepoSDetail{
		Repo:   *repoDoc,
		Issues: issues,
	}, nil
}

// ListRepoIssues fetches issues for a repo from GitHub.
func (s *repoService) ListRepoIssues(ctx context.Context, owner, repoName, state string, perPage int) ([]models.Issue, error) {
	issues, err := s.gh.ListRepoIssues(owner, repoName, state, perPage)
	if err != nil {
		return nil, err
	}
	return issues, nil
}
