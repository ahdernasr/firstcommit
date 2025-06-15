package models

// Repo represents a GitHub repository with its metadata and vector embedding.
type Repo struct {
	ID              string    `bson:"_id" json:"id"`      // Repository full name (e.g. "facebook/react")
	Owner           string    `bson:"owner" json:"owner"` // GitHub username
	Name            string    `bson:"name" json:"name"`   // Repository name
	FullName        string    `bson:"full_name" json:"full_name"`
	Description     string    `bson:"description" json:"description"`
	StargazersCount int       `bson:"stargazers_count" json:"stargazers_count"` // Renamed and re-tagged
	WatchersCount   int       `bson:"watchers_count" json:"watchers_count"`
	ForksCount      int       `bson:"forks_count" json:"forks_count"`
	OpenIssuesCount int       `bson:"open_issues_count" json:"open_issues_count"`
	License         string    `bson:"license" json:"license"`
	Homepage        string    `bson:"homepage" json:"homepage"`
	DefaultBranch   string    `bson:"default_branch" json:"default_branch"`
	CreatedAt       string    `bson:"created_at" json:"created_at"`
	PushedAt        string    `bson:"pushed_at" json:"pushed_at"`
	Size            int       `bson:"size" json:"size"`
	Visibility      string    `bson:"visibility" json:"visibility"`
	Archived        bool      `bson:"archived" json:"archived"`
	AllowForking    bool      `bson:"allow_forking" json:"allow_forking"`
	IsTemplate      bool      `bson:"is_template" json:"is_template"`
	Topics          []string  `bson:"topics" json:"topics"`
	Languages       []string  `bson:"languages" json:"languages"`
	ImageURL        string    `bson:"image_url" json:"image_url"`
	Readme          string    `bson:"readme,omitempty" json:"readme,omitempty"`
	Embedding       []float32 `bson:"embedding" json:"-"`
	Score           float64   `bson:"score" json:"score"`
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
