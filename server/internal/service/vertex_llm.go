package service

import (
	"context"
	"fmt"
	"os"
	"strings"

	"cloud.google.com/go/vertexai/genai"
	"github.com/ahmednasr/ai-in-action/server/internal/models"
	"google.golang.org/api/option"
)

// VertexLLM implements the LLM interface using Google's Vertex AI
type VertexLLM struct {
	client *genai.Client
	model  *genai.GenerativeModel
}

// NewVertexLLM creates a new Vertex AI LLM client
func NewVertexLLM() (*VertexLLM, error) {
	ctx := context.Background()

	// Get credentials from environment or service account file
	var opts []option.ClientOption
	if creds := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); creds != "" {
		opts = append(opts, option.WithCredentialsFile(creds))
	}

	client, err := genai.NewClient(ctx, "ai-in-action-461204", "us-central1", opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vertex AI client: %w", err)
	}

	model := client.GenerativeModel("gemini-2.0-flash-lite-001")
	model.SetTemperature(0.7)
	model.SetTopP(0.8)
	model.SetTopK(40)

	return &VertexLLM{
		client: client,
		model:  model,
	}, nil
}

// GenerateResponse generates a response using the Vertex AI model
func (l *VertexLLM) GenerateResponse(ctx context.Context, prompt string) (string, error) {
	resp, err := l.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("failed to generate response: %w", err)
	}

	if len(resp.Candidates) == 0 {
		return "", fmt.Errorf("no response generated")
	}

	// Convert the response to string
	text, ok := resp.Candidates[0].Content.Parts[0].(genai.Text)
	if !ok {
		return "", fmt.Errorf("unexpected response type")
	}
	return string(text), nil
}

// GenerateGuide generates a guide using the Vertex AI model
func (l *VertexLLM) GenerateGuide(issue models.Issue, snippets []string) (string, error) {
	prompt := fmt.Sprintf(`Based on this GitHub issue and relevant code snippets, provide a detailed guide:

Issue Title: %s
Issue Description: %s

Relevant Code Snippets:
%s

Please provide a comprehensive guide that addresses the issue.`,
		issue.Title,
		issue.Body,
		strings.Join(snippets, "\n\n"))

	return l.GenerateResponse(context.Background(), prompt)
}

// Close closes the Vertex AI client
func (l *VertexLLM) Close() error {
	return l.client.Close()
}
