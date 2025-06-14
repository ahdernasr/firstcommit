package models

import "go.mongodb.org/mongo-driver/bson/primitive"

// Repo represents a repository document stored in MongoDB.
// It also carries a few GitHub‑specific fields so handlers can echo them
// without an extra DTO layer.
type Repo struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Owner       string             `bson:"owner"          json:"owner"`     // e.g. "torvalds"
	Name        string             `bson:"name"           json:"name"`      // e.g. "linux"
	FullName    string             `bson:"full_name"      json:"full_name"` // e.g. "torvalds/linux"
	Description string             `bson:"description"    json:"description"`
	Stars       int                `bson:"stars"          json:"stars"`
	Languages   []string           `bson:"languages"      json:"languages"`
	ImageURL    string             `bson:"image_url"      json:"image_url"` // cached owner avatar or repo header img
	Vector      []float32          `bson:"vector,omitempty" json:"-"`       // embedding vector (not serialized to JSON)
}

// Issue captures the minimal fields we care about from GitHub’s REST API.
type Issue struct {
	ID        int    `json:"id"         bson:"id"`
	Number    int    `json:"number"     bson:"number"`
	Title     string `json:"title"      bson:"title"`
	Body      string `json:"body"       bson:"body"`
	State     string `json:"state"      bson:"state"`
	HTMLURL   string `json:"html_url"   bson:"html_url"`
	CreatedAt string `json:"created_at" bson:"created_at"`
	UpdatedAt string `json:"updated_at" bson:"updated_at"`
	User      struct {
		Login string `json:"login" bson:"login"`
	} `json:"user" bson:"user"`
}
