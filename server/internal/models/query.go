package models

import "time"

// SearchRequest is the payload for GET /search (query parameters) or POST /search.
type SearchRequest struct {
	Query string `json:"q"   query:"q"` // full‑text query
	TopK  int    `json:"k"   query:"k"` // optional; default handled in handler
}

// ChatRequest is the payload for POST /chat follow‑up questions.
type ChatRequest struct {
	ContextID string `json:"context_id"` // ID returned from a guide or prior chat
	Question  string `json:"question"`   // user’s natural‑language question
}

// Guide represents an AI‑generated troubleshooting guide for a GitHub issue.
type Guide struct {
	ID        string    `bson:"_id,omitempty" json:"id"` // same as "owner/repo#number"
	Issue     Issue     `bson:"issue"          json:"issue"`
	Answer    string    `bson:"answer"         json:"answer"`
	CreatedAt time.Time `bson:"created_at"     json:"created_at"`
}
