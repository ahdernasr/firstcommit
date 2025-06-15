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

// Embed generates a 768‑dimensional embedding vector for the input text
// using task_type = "RETRIEVAL_QUERY" so it aligns with document embeddings.
func (v *VertexEmbedder) Embed(text string) ([]float32, error) {
	ctx := context.Background()

	// Create instance with content and explicit task type for semantic search
	instance, err := structpb.NewStruct(map[string]interface{}{
		"content":   text,
		"task_type": "RETRIEVAL_QUERY",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create instance: %w", err)
	}

	// Create prediction request
	req := &aiplatformpb.PredictRequest{
		Endpoint:  v.modelName,
		Instances: []*structpb.Value{structpb.NewStructValue(instance)},
	}

	// Get prediction
	resp, err := v.client.Predict(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get prediction: %w", err)
	}

	// Extract embeddings from response
	if len(resp.Predictions) == 0 {
		return nil, fmt.Errorf("no predictions returned")
	}

	prediction := resp.Predictions[0].GetStructValue()
	embeddings := prediction.GetFields()["embeddings"].GetStructValue()
	values := embeddings.GetFields()["values"].GetListValue().GetValues()

	// Convert to float32 slice
	result := make([]float32, len(values))
	for i, v := range values {
		result[i] = float32(v.GetNumberValue())
	}

	return result, nil
}

// Embed generates an embedding vector for the input text using Gemini
func (g *GeminiEmbedder) Embed(text string) ([]float32, error) {
	ctx := context.Background()

	instance, err := structpb.NewStruct(map[string]interface{}{
		"content":   text,
		"task_type": "RETRIEVAL_QUERY",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create instance: %w", err)
	}

	req := &aiplatformpb.PredictRequest{
		Endpoint:  g.modelName,
		Instances: []*structpb.Value{structpb.NewStructValue(instance)},
	}

	resp, err := g.client.Predict(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get prediction: %w", err)
	}

	if len(resp.Predictions) == 0 {
		return nil, fmt.Errorf("no predictions returned")
	}

	prediction := resp.Predictions[0].GetStructValue()
	embeddings := prediction.GetFields()["embeddings"].GetStructValue()
	values := embeddings.GetFields()["values"].GetListValue().GetValues()

	result := make([]float32, len(values))
	for i, v := range values {
		result[i] = float32(v.GetNumberValue())
	}

	return result, nil
}

// Close releases the Vertex AI client resources
func (v *VertexEmbedder) Close() error {
	return v.client.Close()
}

// Close releases the Gemini AI client resources
func (g *GeminiEmbedder) Close() error {
	return g.client.Close()
}
