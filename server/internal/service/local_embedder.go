package service

import (
	"fmt"
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
	// Call Python script to generate embedding
	cmd := exec.Command("python3", "-c", fmt.Sprintf(`
import sys
from sentence_transformers import SentenceTransformer

model_name = 'all-mpnet-base-v2' if '%s' == 'metadata' else 'microsoft/codebert-base'
print(f"DEBUG: Python script using model: {model_name}", file=sys.stderr)
model = SentenceTransformer(model_name)
embedding = model.encode('%s', normalize_embeddings=True)
print(f"DEBUG: Python script generated embedding of length: {len(embedding.tolist())}", file=sys.stderr)
print(','.join(map(str, embedding.tolist())))
`, l.modelType, strings.ReplaceAll(text, "'", "\\'")))

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Parse the comma-separated output into float32 slice
	values := strings.Split(strings.TrimSpace(string(output)), ",")
	result := make([]float32, len(values))
	for i, v := range values {
		var f float32
		_, err := fmt.Sscanf(v, "%f", &f)
		if err != nil {
			return nil, fmt.Errorf("failed to parse embedding value: %w", err)
		}
		result[i] = f
	}

	return result, nil
}

// Close is a no-op for local embedder
func (l *LocalEmbedder) Close() error {
	return nil
}
