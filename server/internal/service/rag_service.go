package service

import (
	"context"
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// LLM defines the interface for language model interactions
type LLM interface {
	GenerateResponse(ctx context.Context, prompt string) (string, error)
}

type RAGService struct {
	codeColl     *mongo.Collection
	metadataColl *mongo.Collection
	embedder     Embedder
	llm          LLM
}

func NewRAGService(codeColl, metadataColl *mongo.Collection, embedder Embedder, llm LLM) *RAGService {
	return &RAGService{
		codeColl:     codeColl,
		metadataColl: metadataColl,
		embedder:     embedder,
		llm:          llm,
	}
}

type RAGRequest struct {
	Query      string `json:"query"`
	RepoID     string `json:"repo_id,omitempty"`
	MaxResults int    `json:"max_results,omitempty"`
}

type RAGResponse struct {
	Answer     string   `json:"answer"`
	Sources    []Source `json:"sources"`
	Confidence float64  `json:"confidence"`
}

type Source struct {
	RepoID    string  `json:"repo_id"`
	FilePath  string  `json:"file_path"`
	Content   string  `json:"content"`
	Relevance float64 `json:"relevance"`
}

func (s *RAGService) GenerateResponse(ctx context.Context, req RAGRequest) (*RAGResponse, error) {
	// Validate request
	if strings.TrimSpace(req.Query) == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	// 1. Get query embedding
	queryEmbedding, err := s.embedder.Embed(req.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	// 2. Build search pipeline
	pipeline := mongo.Pipeline{
		{
			{"$vectorSearch", bson.M{
				"index":         "vector_index",
				"path":          "embedding",
				"queryVector":   queryEmbedding,
				"numCandidates": 100,
				"limit":         10,
				"similarity":    "cosine",
				"filter":        bson.M{"repo_id": "vue"},
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

	// 3. Execute search
	cursor, err := s.codeColl.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to execute vector search: %w", err)
	}
	defer cursor.Close(ctx)

	// 4. Process results
	var results []struct {
		ID     string  `bson:"_id"`
		RepoID string  `bson:"repo_id"`
		File   string  `bson:"file"`
		Text   string  `bson:"text"`
		Score  float64 `bson:"score"`
	}

	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode search results: %w", err)
	}

	if len(results) == 0 {
		return &RAGResponse{
			Answer:     "I couldn't find any relevant code snippets to answer your question. Please try rephrasing your question or ask about a different aspect of the codebase.",
			Sources:    []Source{},
			Confidence: 0.0,
		}, nil
	}

	// 5. Format sources
	sources := make([]Source, len(results))
	for i, r := range results {
		sources[i] = Source{
			RepoID:    r.RepoID,
			FilePath:  r.File,
			Content:   r.Text,
			Relevance: r.Score,
		}
	}

	// 6. Generate answer using Vertex AI
	prompt := fmt.Sprintf(`Based on this GitHub issue and relevant code snippets, provide a detailed guide:

Issue Title: %s
Issue Description: %s

Relevant Code Snippets:
%s

Please provide a comprehensive guide that addresses the issue.`,
		req.Query,
		req.Query,
		formatSources(sources))

	answer, err := s.llm.GenerateResponse(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate answer: %w", err)
	}

	return &RAGResponse{
		Answer:     answer,
		Sources:    sources,
		Confidence: results[0].Score,
	}, nil
}

func formatSources(sources []Source) string {
	var sb strings.Builder
	for i, source := range sources {
		sb.WriteString(fmt.Sprintf("%d. In %s/%s:\n", i+1, source.RepoID, source.FilePath))
		sb.WriteString("```\n")
		sb.WriteString(source.Content)
		sb.WriteString("\n```\n\n")
	}
	return sb.String()
}
