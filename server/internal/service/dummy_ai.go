package service

import "ai-in-action/internal/models"

type dummyEmbedder struct{}

func (d dummyEmbedder) Embed(string) ([]float32, error) {
	return make([]float32, 768), nil
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
