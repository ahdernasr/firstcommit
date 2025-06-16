package service

// Embedder defines the interface for text embedding services
type Embedder interface {
	// Embed converts a text string into a vector embedding
	Embed(text string) ([]float32, error)
}
