package service

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"strings"
)

// LocalEmbedder uses local models to generate embeddings
type LocalEmbedder struct {
	modelType string // "metadata" or "code"
}

// NewLocalEmbedder creates a new embedder using local models
func NewLocalEmbedder(modelType string) (*LocalEmbedder, error) {
	if modelType != "metadata" && modelType != "code" {
		return nil, fmt.Errorf("invalid model type: %s", modelType)
	}
	return &LocalEmbedder{modelType: modelType}, nil
}

// Embed generates an embedding vector for a single input text
func (l *LocalEmbedder) Embed(text string) ([]float32, error) {
	// Log the input
	log.Printf("Generating embedding for text (first 100 chars): %s...", text[:min(100, len(text))])
	log.Printf("Using model type: %s", l.modelType)

	// Properly escape the text for Python
	escapedText := strings.ReplaceAll(text, "'", "\\'")
	escapedText = strings.ReplaceAll(escapedText, "\n", "\\n")
	escapedText = strings.ReplaceAll(escapedText, "\r", "\\r")

	// Prepare Python script
	pythonScript := fmt.Sprintf(`
import sys
from sentence_transformers import SentenceTransformer

model_name = 'all-mpnet-base-v2' if '%s' == 'metadata' else 'intfloat/multilingual-e5-large'
print(f"DEBUG: Using model: {model_name}", file=sys.stderr)
model = SentenceTransformer(model_name)
print(f"DEBUG: Model loaded successfully", file=sys.stderr)
embedding = model.encode('%s', normalize_embeddings=True)
print(f"DEBUG: Generated embedding of length: {len(embedding.tolist())}", file=sys.stderr)
print(','.join(map(str, embedding.tolist())))
`, l.modelType, escapedText)

	// Log the command we're about to run
	log.Printf("Executing Python script with model type: %s", l.modelType)

	// Call Python script to generate embedding
	cmd := exec.Command("python3", "-c", pythonScript)

	// Capture both stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		log.Printf("Python script error: %v", err)
		log.Printf("Python stderr: %s", stderr.String())
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Log successful execution
	log.Printf("Python stdout: %s", stdout.String())
	log.Printf("Python stderr: %s", stderr.String())

	// Parse the comma-separated output into float32 slice
	values := strings.Split(strings.TrimSpace(stdout.String()), ",")
	result := make([]float32, len(values))
	for i, v := range values {
		var f float32
		_, err := fmt.Sscanf(v, "%f", &f)
		if err != nil {
			log.Printf("Failed to parse value '%s': %v", v, err)
			return nil, fmt.Errorf("failed to parse embedding value: %w", err)
		}
		result[i] = f
	}

	log.Printf("Successfully generated embedding of length: %d", len(result))
	return result, nil
}

// Close is a no-op for local embedder
func (l *LocalEmbedder) Close() error {
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
