package store

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"

	qdrant "github.com/qdrant/go-client/qdrant"
)

var (
	ErrStoreUnavailable  = errors.New("qdrant store unavailable")
	ErrCollectionNotFound = errors.New("collection not found")
)

type SearchResult struct {
	ID      string
	Score   float64
	Payload map[string]string
}

type Store struct {
	client *qdrant.Client
}

func New(host string, port int) (*Store, error) {
	client, err := qdrant.NewClient(&qdrant.Config{
		Host:                   host,
		Port:                   port,
		SkipCompatibilityCheck: true,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrStoreUnavailable, err)
	}
	return &Store{client: client}, nil
}

func (s *Store) Close() error {
	return s.client.Close()
}

func (s *Store) InitCollections(ctx context.Context, dim uint64) error {
	collections := []string{"documents", "memories"}
	for _, name := range collections {
		exists, err := s.client.CollectionExists(ctx, name)
		if err != nil {
			return fmt.Errorf("check %q: %w", name, err)
		}
		if exists {
			continue
		}
		err = s.client.CreateCollection(ctx, &qdrant.CreateCollection{
			CollectionName: name,
			VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
				Size:    dim,
				Distance: qdrant.Distance_Cosine,
			}),
		})
		if err != nil {
			return fmt.Errorf("create %q: %w", name, err)
		}
	}
	return nil
}

func (s *Store) UpsertPoints(ctx context.Context, collection string, points []*qdrant.PointStruct) error {
	_, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: collection,
		Points:         points,
	})
	if err != nil {
		return fmt.Errorf("upsert: %w", err)
	}
	return nil
}

func (s *Store) SearchPoints(ctx context.Context, collection string, vector []float32, topK uint64) ([]SearchResult, error) {
	results, err := s.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: collection,
		Query:          qdrant.NewQuery(vector...),
		Limit:          &topK,
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	return scoredToResults(results), nil
}

func (s *Store) DeletePoints(ctx context.Context, collection, documentID string) error {
	_, err := s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: collection,
		Points: qdrant.NewPointsSelectorFilter(&qdrant.Filter{
			Must: []*qdrant.Condition{qdrant.NewMatchKeyword("document_id", documentID)},
		}),
	})
	if err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	return nil
}

func (s *Store) ListCollections(ctx context.Context) ([]string, error) {
	return s.client.ListCollections(ctx)
}

func (s *Store) GetPointsByID(ctx context.Context, collection string, ids []string) ([]SearchResult, error) {
	pointIDs := make([]*qdrant.PointId, len(ids))
	for i, id := range ids {
		pointIDs[i] = IDFromString(id)
	}
	results, err := s.client.Get(ctx, &qdrant.GetPoints{
		CollectionName: collection,
		Ids:            pointIDs,
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, fmt.Errorf("get: %w", err)
	}
	return retrievedToResults(results), nil
}

func (s *Store) ScrollByFilter(ctx context.Context, collection, field, value string) ([]SearchResult, error) {
	limit := uint32(100)
	results, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: collection,
		Filter:         &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewMatchKeyword(field, value)}},
		WithPayload:    qdrant.NewWithPayload(true),
		Limit:          &limit,
	})
	if err != nil {
		return nil, fmt.Errorf("scroll: %w", err)
	}
	return retrievedToResults(results), nil
}

func NewPoint(id uint64, vector []float32, payload map[string]string) *qdrant.PointStruct {
	m := make(map[string]*qdrant.Value, len(payload))
	for k, v := range payload {
		m[k] = &qdrant.Value{Kind: &qdrant.Value_StringValue{StringValue: v}}
	}
	return &qdrant.PointStruct{
		Id:      qdrant.NewIDNum(id),
		Vectors: qdrant.NewVectors(vector...),
		Payload: m,
	}
}

func NewPointStr(id string, vector []float32, payload map[string]string) *qdrant.PointStruct {
	num, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		h := fnv.New64a()
		h.Write([]byte(id))
		num = h.Sum64()
	}
	return NewPoint(num, vector, payload)
}

func IDFromString(s string) *qdrant.PointId {
	num, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return qdrant.NewIDNum(idToNum(s))
	}
	return qdrant.NewIDNum(num)
}

func idToNum(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

func extractPayload(payload map[string]*qdrant.Value) map[string]string {
	out := make(map[string]string, len(payload))
	for k, v := range payload {
		switch val := v.GetKind().(type) {
		case *qdrant.Value_StringValue:
			out[k] = val.StringValue
		case *qdrant.Value_DoubleValue:
			out[k] = strconv.FormatFloat(val.DoubleValue, 'f', -1, 64)
		case *qdrant.Value_IntegerValue:
			out[k] = strconv.FormatInt(val.IntegerValue, 10)
		case *qdrant.Value_BoolValue:
			out[k] = strconv.FormatBool(val.BoolValue)
		}
	}
	return out
}

func extractPointID(id *qdrant.PointId) string {
	if id == nil {
		return ""
	}
	if uuid := id.GetUuid(); uuid != "" {
		return uuid
	}
	return strconv.FormatUint(id.GetNum(), 10)
}

func scoredToResults(points []*qdrant.ScoredPoint) []SearchResult {
	out := make([]SearchResult, 0, len(points))
	for _, r := range points {
		out = append(out, SearchResult{
			ID:      extractPointID(r.GetId()),
			Score:   float64(r.GetScore()),
			Payload: extractPayload(r.GetPayload()),
		})
	}
	return out
}

func retrievedToResults(points []*qdrant.RetrievedPoint) []SearchResult {
	out := make([]SearchResult, 0, len(points))
	for _, r := range points {
		out = append(out, SearchResult{
			ID:      extractPointID(r.GetId()),
			Payload: extractPayload(r.GetPayload()),
		})
	}
	return out
}

func KeywordOverlapSearch(query string, results []SearchResult) []SearchResult {
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return results
	}

	type scored struct {
		result SearchResult
		score  float64
	}
	scoredList := make([]scored, 0, len(results))
	for _, r := range results {
		content := strings.ToLower(r.Payload["content"])
		count := 0
		for _, t := range terms {
			if strings.Contains(content, t) {
				count++
			}
		}
		if count > 0 {
			scoredList = append(scoredList, scored{r, float64(count) / float64(len(terms))})
		}
	}
	ranked := make([]SearchResult, len(scoredList))
	for i, s := range scoredList {
		s.result.Score = s.score
		ranked[i] = s.result
	}
	return ranked
}

func RRF(dense []SearchResult, sparse []SearchResult, k float64) []SearchResult {
	type entry struct {
		result SearchResult
		score  float64
	}
	seen := map[string]*entry{}
	for i, r := range dense {
		score := 1.0 / (k + float64(i))
		e := &entry{result: r, score: score}
		seen[r.ID] = e
	}
	for i, r := range sparse {
		if e, ok := seen[r.ID]; ok {
			e.score += 1.0 / (k + float64(i))
		} else {
			e := &entry{result: r, score: 1.0 / (k + float64(i))}
			seen[r.ID] = e
		}
	}

	results := make([]SearchResult, 0, len(seen))
	for _, e := range seen {
		e.result.Score = e.score
		results = append(results, e.result)
	}
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
	return results
}

func TopK(results []SearchResult, k int) []SearchResult {
	if len(results) <= k {
		return results
	}
	return results[:k]
}
