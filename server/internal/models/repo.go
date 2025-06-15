package models

// Repo represents a GitHub repository with its metadata and vector embedding.
type Repo struct {
	ID          string    `bson:"_id" json:"id"`      // Repository full name (e.g. "facebook/react")
	Owner       string    `bson:"owner" json:"owner"` // GitHub username
	Name        string    `bson:"name" json:"name"`   // Repository name
	FullName    string    `bson:"full_name" json:"full_name"`
	Description string    `bson:"description" json:"description"`
	Stars       int       `bson:"stars" json:"stars"`
	Languages   []string  `bson:"languages" json:"languages"`
	ImageURL    string    `bson:"image_url" json:"image_url"`
	Embedding   []float32 `bson:"embedding" json:"-"` // Vector embedding (excluded from JSON)
	Score       float64   `bson:"score" json:"score"` // Vector search score
}

// CodeChunk represents a code snippet or documentation chunk from a repository.
type CodeChunk struct {
	ID     string  `bson:"_id" json:"id"`
	RepoID string  `bson:"repo_id" json:"repo_id"`
	Text   string  `bson:"text" json:"text"`
	File   string  `bson:"file" json:"file"`
	Score  float64 `bson:"score" json:"score"`
}

// Issue captures the minimal fields we care about from GitHub's REST API.
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
