package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"ai-in-action/internal/models"
)

// Client is a minimal wrapper around GitHub's REST API v3.
// It is intentionally light—just the endpoints our services require.
type Client struct {
	http  *http.Client
	token string
}

// NewClient returns a ready-to-use GitHub API client.
// token may be an empty string, but you will be subject to very low rate‑limits.
func NewClient(token string) *Client {
	return &Client{
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
		token: token,
	}
}

// ListRepoIssues fetches issues for a repo (excludes pull‑requests by default).
//
//	owner – repository owner (e.g., "torvalds")
//	repo  – repository name  (e.g., "linux")
//	state – "open" | "closed" | "all"
//	perPage – max items per page (1–100)
func (c *Client) ListRepoIssues(owner, repo, state string, perPage int) ([]models.Issue, error) {
	u := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues", url.PathEscape(owner), url.PathEscape(repo))

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	if state != "" {
		q.Set("state", state)
	}
	if perPage > 0 {
		q.Set("per_page", fmt.Sprint(perPage))
	}
	// Exclude pull requests
	q.Set("filter", "all")
	req.URL.RawQuery = q.Encode()

	c.addHeaders(req)

	var issues []models.Issue
	if err := c.do(req, &issues); err != nil {
		return nil, err
	}
	return issues, nil
}

// GetIssue retrieves a single issue by number.
func (c *Client) GetIssue(owner, repo string, number int) (models.Issue, error) {
	u := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d",
		url.PathEscape(owner), url.PathEscape(repo), number)

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return models.Issue{}, err
	}

	c.addHeaders(req)

	var issue models.Issue
	if err := c.do(req, &issue); err != nil {
		return models.Issue{}, err
	}
	return issue, nil
}

// addHeaders sets authentication and Accept headers.
func (c *Client) addHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("User-Agent", "ai-in-action-api")
}

// do executes the HTTP request and decodes JSON into v.
func (c *Client) do(req *http.Request, v interface{}) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("github: unexpected status %s", resp.Status)
	}

	return json.NewDecoder(resp.Body).Decode(v)
}
