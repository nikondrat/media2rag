package store

import (
	"context"

	qdrant "github.com/qdrant/go-client/qdrant"
)

type VectorStore interface {
	InitCollections(ctx context.Context, dim uint64) error
	UpsertPoints(ctx context.Context, collection string, points []*qdrant.PointStruct) error
	SearchPoints(ctx context.Context, collection string, vector []float32, topK uint64) ([]SearchResult, error)
	DeletePoints(ctx context.Context, collection, documentID string) error
	ListCollections(ctx context.Context) ([]string, error)
	GetPointsByID(ctx context.Context, collection string, ids []string) ([]SearchResult, error)
	ScrollByFilter(ctx context.Context, collection, field, value string) ([]SearchResult, error)
	Close() error
}
