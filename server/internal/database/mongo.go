package database

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// NewMongo establishes a new MongoDB client with a 10‑second connection timeout.
//
// It returns:
//   - *mongo.Client  – the connected client
//   - context.Context / context.CancelFunc – so callers can cleanly cancel work
//   - error          – non‑nil when the connection attempt fails
//
// Typical usage:
//
//	client, ctx, cancel, err := database.NewMongo(cfg.MongoURI)
//	if err != nil { … }
//	defer cancel()
//	defer client.Disconnect(ctx)
func NewMongo(uri string) (*mongo.Client, context.Context, context.CancelFunc, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	clientOpts := options.Client().
		ApplyURI(uri).
		SetServerSelectionTimeout(5 * time.Second)

	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, ctx, cancel, err
	}

	// Verify the connection with a ping.
	if err := client.Ping(ctx, nil); err != nil {
		// Disconnect in case of ping failure to avoid leaking sockets.
		_ = client.Disconnect(ctx)
		return nil, ctx, cancel, err
	}

	return client, ctx, cancel, nil
}
