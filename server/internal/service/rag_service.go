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
	Guide      string   `json:"guide,omitempty"`
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
				"filter":        bson.M{"repo_id": req.RepoID},
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

func (s *RAGService) GenerateGuide(ctx context.Context, req RAGRequest) (*RAGResponse, error) {
	// Just reuse the existing RAG functionality with a different prompt
	resp, err := s.GenerateResponse(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to generate guide: %w", err)
	}

	// Update the prompt to generate a guide
	guidePrompt := fmt.Sprintf(`

	You are generating a first-time contributor guide for a GitHub issue using retrieval-augmented context. You will be given:
	•	A GitHub issue describing a bug or feature request.
	•	A list of relevant files extracted from the codebase.

Write a clear, actionable, and beginner-friendly guide to help a developer contribute confidently—even if its their first time in the repo.

The guide should follow this structure:

⸻

Goal
Summarize the issue:
	•	What is the task (bug fix or feature)?
	•	Why does it matter to the project or users?

⸻

Files to Review
For each file provided:
	•	Explain what the file does in the context of the project.
	•	Describe how it relates to the issue or implementation.
	•	Mention important functions, components, or logic that the contributor should look at.

⸻

How to Fix or Implement It
Provide a step-by-step plan:
	•	Outline where and how to make the necessary changes.
	•	Reference specific functions or file sections where edits should occur.
	•	Assume beginner-level familiarity with the codebase.
	•	Be clear and precise.
	•	Use bullet points or numbered steps when appropriate.

⸻

How to Test It
Explain how to test the fix or feature:
	•	Mention relevant commands (e.g., npm run build, yarn test).
	•	If manual testing is needed, describe how to reproduce the bug or verify the new feature works as intended.
	•	Optionally mention any existing tests or testing folders if relevant.

⸻

Comments
If you know anything about the issue or the codebase, you can add quick comments to the guide.

⸻

Output Guidelines:
	•	Output should be in Markdown format.
	•	Total length should be 400–500 words.
	•	Do not greet the user.
	•	Avoid a conversational tone.
	•	Do not include instructions about submitting pull requests.
	•	Prioritize clarity, relevance, and a confidence-building tone.
	•	Do not use emojis.

Formatting Guidelines:
	•	Use level 2 headers (##) for top-level sections like ## Goal, ## Files to Review, etc.
	•	Use level 3 headers (###) for optional sub-sections.
	•	Use bullet points or numbered steps where appropriate.
	•	Use fenced code blocks (triple backticks) for code snippets.
	•	Use Markdown block quotes (>) for file paths.
	•	Use bold for important words.
	

GitHub Issue: %s

Relevant Files:
%s

Write a guide that helps a junior developer contribute confidently without prior repo experience.`,
		req.Query,
		formatSources(resp.Sources))

	guide, err := s.llm.GenerateResponse(ctx, guidePrompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate guide: %w", err)
	}

	return &RAGResponse{
		Answer:     resp.Answer,
		Sources:    resp.Sources,
		Confidence: resp.Confidence,
		Guide:      guide,
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
