package rag

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	qdrant "github.com/qdrant/go-client/qdrant"

	"media2rag/internal/llm"
	"media2rag/internal/store"
)

const (
	ParentTokenSize = 512
	ChildTokenSize  = 128
)

type Indexer struct {
	store  *store.Store
	client llm.LLMClient
}

func NewIndexer(s *store.Store, client llm.LLMClient) *Indexer {
	return &Indexer{store: s, client: client}
}

type Chunk struct {
	ID          string
	Content     string
	DocumentID  string
	ParentID    string
	ContentHash string
	ChunkType   string
}

func (idx *Indexer) IndexDocument(ctx context.Context, documentID, content string) error {
	parentChunks := splitText(content, ParentTokenSize)
	var points []*qdrant.PointStruct

	for pi, parent := range parentChunks {
		parentID := fmt.Sprintf("%s_p%d", documentID, pi)
		parentHash := contentHash(parent)
		parentEmbedding, err := idx.embed(ctx, parent)
		if err != nil {
			return fmt.Errorf("embed parent %d: %w", pi, err)
		}

		points = append(points, store.NewPointStr(parentID, parentEmbedding, map[string]string{
			"content":      parent,
			"document_id":  documentID,
			"parent_id":    "",
			"content_hash": parentHash,
			"chunk_type":   "parent",
			"chunk_id":     parentID,
		}))

		childChunks := splitText(parent, ChildTokenSize)
		for ci, child := range childChunks {
			childID := fmt.Sprintf("%s_p%d_c%d", documentID, pi, ci)
			childHash := contentHash(child)
			childEmbedding, err := idx.embed(ctx, child)
			if err != nil {
				return fmt.Errorf("embed child %d_%d: %w", pi, ci, err)
			}

			points = append(points, store.NewPointStr(childID, childEmbedding, map[string]string{
				"content":      child,
				"document_id":  documentID,
				"parent_id":    parentID,
				"content_hash": childHash,
				"chunk_type":   "child",
				"chunk_id":     childID,
			}))
		}
	}

	return idx.store.UpsertPoints(ctx, "documents", points)
}

func (idx *Indexer) embed(ctx context.Context, text string) ([]float32, error) {
	return idx.client.Embed(ctx, text)
}

func splitText(text string, tokenSize int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var chunks []string
	for i := 0; i < len(words); i += tokenSize {
		end := i + tokenSize
		if end > len(words) {
			end = len(words)
		}
		chunks = append(chunks, strings.Join(words[i:end], " "))
	}
	return chunks
}

func contentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h)
}


