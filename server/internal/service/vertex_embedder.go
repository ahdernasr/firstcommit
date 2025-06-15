package service

import (
	"context"
	"fmt"
	"os"

	aiplatform "cloud.google.com/go/aiplatform/apiv1"
	"cloud.google.com/go/aiplatform/apiv1/aiplatformpb"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/types/known/structpb"
)

// VertexEmbedder uses Google's text-embedding-005 model to generate embeddings
type VertexEmbedder struct {
	client    *aiplatform.PredictionClient
	modelName string
}

// GeminiEmbedder uses Google's gemini-embedding-001 model to generate embeddings
type GeminiEmbedder struct {
	client    *aiplatform.PredictionClient
	modelName string
}

// NewVertexEmbedder creates a new embedder using the service account credentials
func NewVertexEmbedder() (*VertexEmbedder, error) {
	ctx := context.Background()

	// Initialize Vertex AI client
	client, err := aiplatform.NewPredictionClient(ctx, option.WithCredentialsFile("server-key.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to create Vertex AI client: %w", err)
	}

	// Construct model name
	projectID := os.Getenv("GCP_PROJECT_ID")
	location := os.Getenv("GCP_LOCATION")
	if location == "" {
		location = "us-central1"
	}
	modelName := fmt.Sprintf("projects/%s/locations/%s/publishers/google/models/text-embedding-005", projectID, location)

	return &VertexEmbedder{
		client:    client,
		modelName: modelName,
	}, nil
}

// NewGeminiEmbedder creates a new embedder using the Gemini model
func NewGeminiEmbedder() (*GeminiEmbedder, error) {
	ctx := context.Background()

	client, err := aiplatform.NewPredictionClient(ctx, option.WithCredentialsFile("server-key.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to create Vertex AI client: %w", err)
	}

	projectID := os.Getenv("GCP_PROJECT_ID")
	location := os.Getenv("GCP_LOCATION")
	if location == "" {
		location = "us-central1"
	}
	modelName := fmt.Sprintf("projects/%s/locations/%s/publishers/google/models/gemini-embedding-001", projectID, location)

	return &GeminiEmbedder{
		client:    client,
		modelName: modelName,
	}, nil
}

func embedBatch(ctx context.Context, client *aiplatform.PredictionClient, modelName string, texts []string) ([][]float32, error) {
	instances := make([]*structpb.Value, 0, len(texts))
	for _, text := range texts {
		if len(text) < 20 {
			continue
		}
		if len(text) > 2000 {
			text = text[:2000]
		}
		instance, err := structpb.NewStruct(map[string]interface{}{
			"content":   text,
			"task_type": "RETRIEVAL_DOCUMENT",
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create instance: %w", err)
		}
		instances = append(instances, structpb.NewStructValue(instance))
	}

	if len(instances) == 0 {
		return nil, fmt.Errorf("no valid texts to embed")
	}

	req := &aiplatformpb.PredictRequest{
		Endpoint:  modelName,
		Instances: instances,
	}

	resp, err := client.Predict(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get prediction: %w", err)
	}

	if len(resp.Predictions) == 0 {
		return nil, fmt.Errorf("no predictions returned")
	}

	embeddingsBatch := make([][]float32, len(resp.Predictions))
	for i, predictionValue := range resp.Predictions {
		prediction := predictionValue.GetStructValue()
		embeddings := prediction.GetFields()["embeddings"].GetStructValue()
		values := embeddings.GetFields()["values"].GetListValue().GetValues()

		result := make([]float32, len(values))
		for j, v := range values {
			result[j] = float32(v.GetNumberValue())
		}
		embeddingsBatch[i] = result
	}

	return embeddingsBatch, nil
}

// EmbedBatch generates embedding vectors for multiple input texts using VertexEmbedder
func (v *VertexEmbedder) EmbedBatch(texts []string) ([][]float32, error) {
	ctx := context.Background()
	const maxBatch = 5
	var allEmbeddings [][]float32

	for i := 0; i < len(texts); i += maxBatch {
		end := i + maxBatch
		if end > len(texts) {
			end = len(texts)
		}
		chunk := texts[i:end]
		embeddings, err := embedBatch(ctx, v.client, v.modelName, chunk)
		if err != nil {
			return nil, err
		}
		allEmbeddings = append(allEmbeddings, embeddings...)
	}
	return allEmbeddings, nil
}

// EmbedBatch generates embedding vectors for multiple input texts using GeminiEmbedder
func (g *GeminiEmbedder) EmbedBatch(texts []string) ([][]float32, error) {
	ctx := context.Background()
	const maxBatch = 5
	var allEmbeddings [][]float32

	for i := 0; i < len(texts); i += maxBatch {
		end := i + maxBatch
		if end > len(texts) {
			end = len(texts)
		}
		chunk := texts[i:end]
		embeddings, err := embedBatch(ctx, g.client, g.modelName, chunk)
		if err != nil {
			return nil, err
		}
		allEmbeddings = append(allEmbeddings, embeddings...)
	}
	return allEmbeddings, nil
}

// Embed generates an embedding vector for a single input text using VertexEmbedder
func (v *VertexEmbedder) Embed(text string) ([]float32, error) {
	embeddings, err := v.EmbedBatch([]string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned for text")
	}
	return embeddings[0], nil
}

// Embed generates an embedding vector for a single input text using GeminiEmbedder
func (g *GeminiEmbedder) Embed(text string) ([]float32, error) {
	embeddings, err := g.EmbedBatch([]string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned for text")
	}
	return embeddings[0], nil
}

// Close releases the Vertex AI client resources
func (v *VertexEmbedder) Close() error {
	return v.client.Close()
}

// Close releases the Gemini AI client resources
func (g *GeminiEmbedder) Close() error {
	return g.client.Close()
}
