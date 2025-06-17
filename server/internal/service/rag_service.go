package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ahmednasr/ai-in-action/server/internal/models"
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
	guideSvc     GuideService
}

func NewRAGService(codeColl, metadataColl *mongo.Collection, embedder Embedder, llm LLM, guideSvc GuideService) *RAGService {
	return &RAGService{
		codeColl:     codeColl,
		metadataColl: metadataColl,
		embedder:     embedder,
		llm:          llm,
		guideSvc:     guideSvc,
	}
}

type RAGRequest struct {
	Query       string `json:"query"`
	RepoID      string `json:"repo_id,omitempty"`
	IssueNumber string `json:"issue_number,omitempty"` // GitHub issue number (e.g., "51878")
	MaxResults  int    `json:"max_results,omitempty"`
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

	// 6. Get the issue details and guide
	var guide models.Guide
	var issueDetails string
	if req.IssueNumber != "" {
		issueID := req.RepoID + "#" + req.IssueNumber
		guide, err = s.guideSvc.GetGuide(ctx, issueID)
		if err != nil {
			log.Printf("Warning: Failed to get guide for issue %s: %v", issueID, err)
		} else if guide.Issue.Title != "" && guide.Issue.Body != "" {
			// Use cached issue details
			issueDetails = fmt.Sprintf("Title: %s\n\nDescription:\n%s", guide.Issue.Title, guide.Issue.Body)
		} else {
			// Fallback to GitHub API
			log.Printf("Guide is missing issue details. Fetching from GitHub API...")
			url := fmt.Sprintf("https://api.github.com/repos/%s/issues/%s", req.RepoID, req.IssueNumber)
			reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			type ghIssue struct {
				Title string `json:"title"`
				Body  string `json:"body"`
			}

			httpReq, err := http.NewRequestWithContext(reqCtx, "GET", url, nil)
			if err != nil {
				log.Printf("Failed to create GitHub request: %v", err)
			} else {
				httpReq.Header.Set("Accept", "application/vnd.github+json")
				client := &http.Client{}
				resp, err := client.Do(httpReq)
				if err != nil {
					log.Printf("Failed to fetch GitHub issue: %v", err)
				} else {
					defer resp.Body.Close()
					var gh ghIssue
					if err := json.NewDecoder(resp.Body).Decode(&gh); err != nil {
						log.Printf("Failed to decode GitHub issue response: %v", err)
					} else {
						issueDetails = fmt.Sprintf("Title: %s\n\nDescription:\n%s", gh.Title, gh.Body)
					}
				}
			}
		}
	}

	// 7. Generate answer using Vertex AI with enhanced prompt
	prompt := fmt.Sprintf(`You are an AI assistant helping a developer understand and work on a GitHub issue. Use the following context to answer the user's question:

Issue Details:
%s

First-Time Contributor Guide:
%s

Relevant Code Snippets:
%s

User's Question: %s

Please provide a clear and helpful answer that:
1. Directly addresses the user's question
2. References specific parts of the code when relevant
3. Uses markdown links in the format [filename](filepath) when referencing files
• If a file path has more than 6 segments (e.g., a/b/c/d/e/f/g), truncate the middle using `+"`...`"+` like a/b/c/.../e/f/g for display, but keep the full filepath in the markdown link.
4. Maintains a professional and technical tone
5. Focuses on helping the user understand and solve the issue
6. Remember that most if not all questions have the goal or the need of solving the issue.  IMPORTANT!

IMPORTANT NOTE: 
You will always be given code snippets. Sometimes the users response will not require new snippets, and you will not have to use them in your response. 
Sometimes they will ask about the snippets in the first-time contributor guide which you will have to respond to. 

Formatting Rules
• Use level 2 headers (##) for top-level sections.
• Use level 3 headers (###) for optional sub-sections if needed.
• Use bullet points or numbered steps for procedures.
• Use fenced code blocks(%[1]s) for code snippets.
• Use markdown links for file references: [filename](filepath)
• If a file path has more than 6 segments (e.g., a/b/c/d/e/f/g), truncate the middle using `+"`...`"+` like a/b/c/.../e/f/g for display, but keep the full filepath in the markdown link.
• Do not use conventional number a number should be followed by ) in a numbered list, such as 1) 2) 3)
• **All bullets and numbered steps must place their description on the same line**. Example: 1) Run the test not 1)\nRun the tests. Make sure no formatting glitch causes this to happen.
• You must not break to a new line after 1) or •. The description must follow immediately on the same line. 
• If a break after a numbered step or a bullet is done then the output is considered invalid. 

Failure to follow any rules will deem the response invalid. 

Your response should be in markdown format and should not include any meta-commentary or disclaimers.`,
		issueDetails, // Formatted issue details
		guide.Answer, // Guide content
		formatSources(sources),
		req.Query) // User's question

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
	log.Printf("[Guide Generation] Starting guide generation for repo: %s, issue: %s", req.RepoID, req.IssueNumber)

	// Validate required fields
	if req.IssueNumber == "" {
		log.Printf("[Guide Generation] Missing issue number in request")
		return nil, fmt.Errorf("issue number is required")
	}

	// Check cache first
	issueID := req.RepoID + "#" + req.IssueNumber
	guide, err := s.guideSvc.GetGuide(ctx, issueID)
	if err == nil && guide.ID != "" {
		log.Printf("[Guide Generation] Found cached guide for issue: %s", issueID)
		return &RAGResponse{
			Guide: guide.Answer,
		}, nil
	}
	log.Printf("[Guide Generation] No cached guide found, generating new guide for issue: %s", issueID)

	// Generate new guide using RAG
	resp, err := s.GenerateResponse(ctx, req)
	if err != nil {
		log.Printf("[Guide Generation] Error generating initial response: %v", err)
		return nil, fmt.Errorf("failed to generate guide: %w", err)
	}
	log.Printf("[Guide Generation] Successfully generated initial response")

	// Update the prompt to generate a guide
	guidePrompt := fmt.Sprintf(`

IMPORTANT: When generating the guide below:
- DO NOT put the content of any step on a new line after 1), 2), etc.
- Do NOT format numbered steps or bullets with * or ** or other characters that cause indentation or list parsing.
- DO NOT indent or break lines between the number and the description.
- Every bullet point or step must stay on the SAME line as its description. If you break after 1), your output will be considered INVALID. Now follow the instructions below:

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
• When referencing files, use markdown links in the format [filename](filepath). For example, if you want to reference a file at src/main.go, write it as [main.go](src/main.go).
• If a file path has more than 6 segments (e.g., a/b/c/d/e/f/g), truncate the middle using `+"`...`"+` like a/b/c/.../e/f/g for display, but keep the full filepath in the markdown link.

⸻

Formatting Rules
• Use level 2 headers (##) for top-level sections.
• Use level 3 headers (###) for optional sub-sections if needed.
• Use bullet points or numbered steps for procedures.
• Use fenced code blocks (%[1]s) for code snippets.
• Use markdown links for file references: [filename](filepath)
• If a file path has more than 6 segments (e.g., a/b/c/d/e/f/g), truncate the middle using `+"`...`"+` like a/b/c/.../e/f/g for display, but keep the full filepath in the markdown link.
• Do not use convential number a number should be followed by ) in a numbered list, such as 1) 2) 3)
• **All bullets and numbered steps must place their description on the same line**. Example: 1) Run the test not 1)\nRun the tests. Make sure no formatting glitch causes this to happen.
• You must not break to a new line after 1) or •. The description must follow immediately on the same line. 
• If a break after a numbered step or a bullet is done then the output is considered invalid. 

⸻

Required Section Structure

Use the following exact headers and order (do not add or rename):

## Purpose of This Contribution

Clearly explain what this contribution aims to fix, improve, or introduce in direct relation to the GitHub issue. Frame it in terms of developer clarity, performance, maintainability, or correctness.

## Context

Summarize the relevant background from the issue—prior behavior, technical gaps, or what problem the current implementation poses.

## Files to Review

For each file provided (make sure you include each source provided), use markdown links to reference them. it should always be the full filename and full filepath never cut them down. Always break a line between the repo link and its description.

> [filename](filepath)

Explain what the file does in the context of the project. Describe how it relates to the issue or implementation. Mention important functions, components, or logic to focus on.

Do not use bullet points or numbers to list the file paths. Only use block quotes for the path and unformatted text underneath for its description. This is achieved by making sure there is a blank next line between the two. 

## How to Fix
• Outline where and how to make the required changes.
• Reference specific file paths using markdown links, it should always be the full filename and full filepath never cut them down: [filename](filepath). 
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
		"```markdown, do not wrap the code in ```. If you do either, your answer is invalid.", req.Query, formatSources(resp.Sources))

	guideContent, err := s.llm.GenerateResponse(ctx, guidePrompt)
	if err != nil {
		log.Printf("[Guide Generation] Error generating guide content: %v", err)
		return nil, fmt.Errorf("failed to generate guide: %w", err)
	}
	log.Printf("[Guide Generation] Successfully generated guide content")

	// Create a guide model and cache it
	guideModel := models.Guide{
		ID:        issueID,
		Answer:    guideContent,
		CreatedAt: time.Now(),
	}

	// Cache the guide in MongoDB
	log.Printf("[Guide Generation] Attempting to cache guide for issue: %s", issueID)
	if err := s.guideSvc.Upsert(ctx, guideModel); err != nil {
		log.Printf("[Guide Generation] Failed to cache guide for issue %s: %v", issueID, err)
	} else {
		log.Printf("[Guide Generation] Successfully cached guide for issue: %s", issueID)
	}

	return &RAGResponse{
		Answer:     resp.Answer,
		Sources:    resp.Sources,
		Confidence: resp.Confidence,
		Guide:      guideContent,
	}, nil
}

func formatSources(sources []Source) string {
	var sb strings.Builder
	for _, s := range sources {
		truncatedPath := truncateFilePath(s.FilePath)
		sb.WriteString(fmt.Sprintf("File: [%s](%s)\n", truncatedPath, s.FilePath))
		sb.WriteString("Content:\n```\n")
		sb.WriteString(s.Content)
		sb.WriteString("\n```\n\n")
	}
	return sb.String()
}

func truncateFilePath(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 6 {
		return strings.Join(parts[:3], "/") + "/.../" + strings.Join(parts[len(parts)-3:], "/")
	}
	return path
}
