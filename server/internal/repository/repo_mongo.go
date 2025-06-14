package repository

import (
	"context"
	"fmt"

	"ai-in-action/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// RepoMongo satisfies three different interfaces used across the service layer:
//   - RepoRepository     – FindByID()
//   - SearchRepoRepository – VectorSearch()
//   - RepoRepository (guide_service) – FindByID() + GetTopContextChunks()
type RepoMongo struct {
	metaCol   *mongo.Collection // "repos_meta" (one doc per repo)
	chunkCol  *mongo.Collection // "repos_code" (code / readme chunks with embeddings)
	vectorIdx string            // name of Atlas Vector Search index
}

// NewRepoRepository wires the collections.
//
// Expected schema:
//
//	repos_meta
//	  { _id: ObjectId, name, owner, full_name, stars, languages, image_url, vector: []float32 }
//
//	repos_code
//	  { _id: ObjectId, repo_id: ObjectId, text: string, vector: []float32 }
func NewRepoRepository(db *mongo.Database) *RepoMongo {
	return &RepoMongo{
		metaCol:   db.Collection("repos_meta"),
		chunkCol:  db.Collection("repos_code"),
		vectorIdx: "repo_embedding_index",
	}
}

// -------------------------- public API --------------------------------------

// FindByID fetches a repo document by its string ObjectID (hex form).
func (r *RepoMongo) FindByID(ctx context.Context, id string) (models.Repo, error) {
	var repo models.Repo
	err := r.metaCol.FindOne(ctx, bson.M{"_id": id}).Decode(&repo)
	return repo, err
}

// VectorSearch performs a K‑NN search across repo embeddings.
func (r *RepoMongo) VectorSearch(ctx context.Context, queryVec []float32, k int) ([]models.Repo, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$vectorSearch", Value: bson.D{
			{Key: "index", Value: r.vectorIdx},
			{Key: "queryVector", Value: queryVec},
			{Key: "path", Value: "vector"},
			{Key: "numCandidates", Value: k * 10},
			{Key: "limit", Value: k},
		}}},
		{{Key: "$project", Value: bson.D{
			{Key: "vector", Value: 0}, // omit heavy field
		}}},
	}

	cur, err := r.metaCol.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var repos []models.Repo
	if err := cur.All(ctx, &repos); err != nil {
		return nil, err
	}
	return repos, nil
}

// GetTopContextChunks grabs the most similar code / README chunks for RAG.
func (r *RepoMongo) GetTopContextChunks(ctx context.Context, repoID string, queryVec []float32, k int) ([]string, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"repo_id": repoID}}},
		{{Key: "$vectorSearch", Value: bson.D{
			{Key: "index", Value: "code_chunk_index"},
			{Key: "queryVector", Value: queryVec},
			{Key: "path", Value: "vector"},
			{Key: "numCandidates", Value: k * 10},
			{Key: "limit", Value: k},
		}}},
		{{Key: "$project", Value: bson.M{
			"text": 1,
		}}},
	}

	cur, err := r.chunkCol.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	type chunk struct {
		Text string `bson:"text"`
	}
	var out []chunk
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	chunks := make([]string, len(out))
	for i, c := range out {
		chunks[i] = c.Text
	}
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks found for repo %s", repoID)
	}
	return chunks, nil
}
