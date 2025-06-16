package service

import (
	"fmt"

	"github.com/ahmednasr/ai-in-action/server/internal/models"
)

type dummyEmbedder struct{}

func (d dummyEmbedder) Embed(text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("empty text provided")
	}
	// Return a fixed-size embedding vector with some non-zero values
	embedding := make([]float32, 768)
	for i := range embedding {
		embedding[i] = 0.1 // Small non-zero value
	}
	return embedding, nil
}

func NewDummyEmbedder() EmbeddingClient {
	return dummyEmbedder{}
}

type dummyLLM struct{}

func (d dummyLLM) GenerateGuide(issue models.Issue, ctx []string) (string, error) {
	return "<placeholder answer>", nil
}

func NewDummyLLM() LLMClient {
	return dummyLLM{}
}
