package rag

import (
	"strings"

	"media2rag/internal/store"
)

func KeywordOverlapSearch(query string, results []store.SearchResult) []store.SearchResult {
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return results
	}

	type scored struct {
		result store.SearchResult
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
	ranked := make([]store.SearchResult, len(scoredList))
	for i, s := range scoredList {
		s.result.Score = s.score
		ranked[i] = s.result
	}
	return ranked
}

func RRF(dense []store.SearchResult, sparse []store.SearchResult, k float64) []store.SearchResult {
	type entry struct {
		result store.SearchResult
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

	results := make([]store.SearchResult, 0, len(seen))
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

func TopK(results []store.SearchResult, k int) []store.SearchResult {
	if len(results) <= k {
		return results
	}
	return results[:k]
}
