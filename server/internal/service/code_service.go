package service

import (
	"context"
)

// CodeService handles file content retrieval operations
type CodeService interface {
	GetFileContent(ctx context.Context, repoID string, filePath string) (string, error)
}

type codeService struct {
	repoRepo RepoRepository
}

// NewCodeService creates a new instance of CodeService
func NewCodeService(repoRepo RepoRepository) CodeService {
	return &codeService{
		repoRepo: repoRepo,
	}
}

// GetFileContent retrieves the content of a file from the repository
func (s *codeService) GetFileContent(ctx context.Context, repoID string, filePath string) (string, error) {
	return s.repoRepo.GetFileContent(ctx, repoID, filePath)
}
