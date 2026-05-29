package rag

import (
	"context"
	"sort"

	"media2rag/internal/store"
)

type ParentLookup struct {
	st store.VectorStore
}

func NewParentLookup(st store.VectorStore) *ParentLookup {
	return &ParentLookup{st: st}
}

func (pl *ParentLookup) Lookup(ctx context.Context, results []store.SearchResult) ([]store.SearchResult, error) {
	parentIDs := make(map[string]int)
	for _, r := range results {
		if pid := r.Payload["parent_id"]; pid != "" {
			parentIDs[pid]++
		}
	}

	if len(parentIDs) == 0 {
		return results, nil
	}

	ids := make([]string, 0, len(parentIDs))
	for pid := range parentIDs {
		ids = append(ids, pid)
	}

	parents, err := pl.st.GetPointsByID(ctx, "documents", ids)
	if err != nil {
		return nil, err
	}

	ranked := make([]store.SearchResult, len(parents))
	for i, p := range parents {
		p.Score = float64(parentIDs[p.ID])
		ranked[i] = p
	}

	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Score > ranked[j].Score
	})

	return ranked, nil
}
