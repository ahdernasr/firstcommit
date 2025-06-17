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
				"limit":         5,
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
• A GitHub issue describing a bug or feature request.
• A list of relevant files extracted from the codebase.

Write a clear, actionable, and beginner-friendly guide to help a developer confidently address this specific issue—even if it's their first time in the repository.

⸻

Output Requirements
• Write in pure Markdown. Do not wrap the entire guide in %s or any fenced code block.
• The guide must focus only on solving the issue described—not on general contribution practices.
• Tone should be clear, direct, and confidence-building.
• Avoid conversational or overly friendly language.
• Do not include PR submission instructions.
• Keep total length between 400–700 words.
• Use 2 to 3 code snippets (in fenced code blocks using triple backticks, not indented).

⸻

Formatting Rules
• Use level 2 headers (##) for top-level sections.
• Use level 3 headers (###) for optional sub-sections if needed.
• Use bullet points or numbered steps for procedures.
• Use fenced code blocks (%[1]s) for code snippets.
• Use block quotes (>) **only** for file paths. Do not include any description inside the quote. The description must be on the next, next line (2 lines after) with no formatting (not italicized, bold, or quoted).
• **All bullets and numbered steps must place their description on the same line**. Example: 1. Run the test not 1.\nRun the tests.
• You must not break to a new line after 1. or •. The description must follow immediately on the same line.

⸻

Required Section Structure

Use the following exact headers and order (do not add or rename):

## Purpose of This Contribution

Clearly explain what this contribution aims to fix, improve, or introduce in direct relation to the GitHub issue. Frame it in terms of developer clarity, performance, maintainability, or correctness.

## Context

Summarize the relevant background from the issue—prior behavior, technical gaps, or what problem the current implementation poses.

## Files to Review

For each file provided (make sure you include each source provided):
> file/path/example.go  


Explain what the file does in the context of the project. Describe how it relates to the issue or implementation. Mention important functions, components, or logic to focus on.

Do not use bullet points or numbers to list the file paths. Only use block quotes for the path and unformatted text underneath for its description. This is achieved by making sure there is a blank next line between the two. 

## How to Fix
• Outline where and how to make the required changes.
• Reference specific file paths and sections if available.
• Use bullet points or numbered steps.
• Assume beginner familiarity with the codebase.

## How to Test
• Describe how to verify the changes are working correctly.
• Include any commands, scripts, or test steps.
• Mention what successful behavior looks like.

## Example

(Optional) Include 1–2 relevant code snippets, logs, or output examples showing the fix in action or an expected result.

## Notes
• List any extra considerations like edge cases, performance implications, or future improvements.
• If applicable, include known limitations or tradeoffs.

GitHub Issue: %[2]s

Relevant Files:
%[3]s

Write a guide that helps a junior developer contribute confidently without prior repo experience.`,
		"```", req.Query, formatSources(resp.Sources))

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
