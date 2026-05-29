//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	qdrant "github.com/qdrant/go-client/qdrant"

	"media2rag/internal/config"
	"media2rag/internal/llm"
	"media2rag/internal/model"
	"media2rag/internal/rag"
	"media2rag/internal/service"
	"media2rag/internal/store"
)

var (
	cfg         *config.Config
	llmClient   llm.LLMClient
	embedClient llm.LLMClient
)

func TestMain(m *testing.M) {
	var err error
	cfg, err = config.Load("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "config load: %v\n", err)
		os.Exit(1)
	}

	llmClient, err = llm.NewClient(
		context.Background(),
		cfg.LLM.DefaultBackend,
		cfg.LLM.OllamaURL,
		cfg.LLM.Model,
		cfg.LLM.OpenRouterURL,
		cfg.LLM.OpenRouterKey,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "llm init: %v\n", err)
		os.Exit(1)
	}

	embedClient = llm.NewOllamaClient(cfg.LLM.OllamaURL, cfg.LLM.EmbedModel)

	os.Exit(m.Run())
}

func requireStore(t *testing.T) store.VectorStore {
	t.Helper()
	ctx := context.Background()

	qdrantSvc := service.NewQdrant(cfg.RAG.Qdrant)
	s, err := qdrantSvc.EnsureRunning(ctx, embedClient)
	if err != nil {
		t.Fatalf("Qdrant unavailable at %s:%d (auto-start=%v): %v",
			cfg.RAG.Qdrant.Host, cfg.RAG.Qdrant.Port, cfg.RAG.Qdrant.AutoStart, err)
	}

	cols, err := s.ListCollections(ctx)
	if err != nil {
		t.Fatalf("Qdrant unreachable: %v", err)
	}
	t.Logf("Qdrant collections before: %v", cols)

	t.Cleanup(func() {
		qdrantSvc.Stop(ctx)
	})
	return s
}

func requireOllamaChat(t *testing.T) llm.LLMClient {
	t.Helper()
	ctx := context.Background()
	_, err := llmClient.Chat(ctx, model.ChatRequest{
		Messages: []model.Message{{Role: "user", Content: "respond with OK"}},
	})
	if err != nil {
		t.Fatalf("Ollama chat (%s) unavailable: %v", cfg.LLM.Model, err)
	}
	return llmClient
}

func requireOllamaEmbed(t *testing.T) llm.LLMClient {
	t.Helper()
	ctx := context.Background()
	_, err := embedClient.Embed(ctx, "test")
	if err != nil {
		t.Fatalf("Ollama embed (%s) unavailable: %v", cfg.LLM.EmbedModel, err)
	}
	return embedClient
}

func TestE2E_StoreInitAndCollections(t *testing.T) {
	s := requireStore(t)
	ctx := context.Background()

	cols, err := s.ListCollections(ctx)
	if err != nil {
		t.Fatalf("ListCollections: %v", err)
	}
	t.Logf("Collections: %v", cols)

	found := map[string]bool{"documents": false, "memories": false}
	for _, c := range cols {
		found[c] = true
	}
	if !found["documents"] {
		t.Fatal("documents collection missing")
	}
	if !found["memories"] {
		t.Fatal("memories collection missing")
	}
}

func TestE2E_IndexAndDenseSearch(t *testing.T) {
	s := requireStore(t)
	_ = requireOllamaEmbed(t)
	ctx := context.Background()

	content := `The Go programming language, often called Golang, was designed at Google by Robert Griesemer, Rob Pike, and Ken Thompson.
It is a statically typed, compiled language known for its simplicity, strong concurrency primitives (goroutines and channels),
fast compilation times, and built-in garbage collection. Go is widely used for cloud services, CLI tools, microservices,
and DevOps infrastructure. The language has a rich standard library and a growing ecosystem.`

	idx := rag.NewIndexer(s, embedClient)
	docID := fmt.Sprintf("e2e-test-%d", time.Now().UnixNano())

	err := idx.IndexDocument(ctx, docID, content)
	if err != nil {
		t.Fatalf("IndexDocument: %v", err)
	}
	t.Logf("Indexed doc: %s", docID)

	embedding, err := embedClient.Embed(ctx, "Go programming language concurrency")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(embedding) == 0 {
		t.Fatal("embedding is empty")
	}
	t.Logf("Embedding dim: %d", len(embedding))

	results, err := s.SearchPoints(ctx, "documents", embedding, 5)
	if err != nil {
		t.Fatalf("SearchPoints: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("dense search returned 0 results")
	}
	t.Logf("Dense search: %d results, top score=%.4f", len(results), results[0].Score)

	for _, r := range results {
		if r.Payload["document_id"] == docID {
			t.Logf("Found document in search results: id=%s score=%.4f", r.ID, r.Score)
			return
		}
	}
	t.Fatal("indexed document not found in search results")
}

func TestE2E_HybridSearchAndRRF(t *testing.T) {
	s := requireStore(t)
	_ = requireOllamaEmbed(t)
	ctx := context.Background()

	content := strings.Repeat("Go goroutines enable lightweight concurrent execution. ", 50)
	docID := fmt.Sprintf("e2e-hybrid-%d", time.Now().UnixNano())

	idx := rag.NewIndexer(s, embedClient)
	err := idx.IndexDocument(ctx, docID, content)
	if err != nil {
		t.Fatalf("IndexDocument: %v", err)
	}

	embedding, err := embedClient.Embed(ctx, "Go concurrency goroutines")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}

	searcher := rag.NewSearcher(s)
	results, err := searcher.HybridSearch(ctx, "Go goroutines concurrent", embedding, 5)
	if err != nil {
		t.Fatalf("HybridSearch: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("hybrid search returned 0 results")
	}
	t.Logf("Hybrid search: %d results, top score=%.4f", len(results), results[0].Score)

	found := false
	for _, r := range results {
		if r.Payload["document_id"] == docID {
			found = true
			t.Logf("Doc found via hybrid: id=%s score=%.4f type=%s", r.ID, r.Score, r.Payload["chunk_type"])
			break
		}
	}
	if !found {
		t.Fatal("indexed document missing from hybrid search results")
	}
}

func TestE2E_Rewriter(t *testing.T) {
	_ = requireOllamaChat(t)
	ctx := context.Background()

	rw := rag.NewRewriter(llmClient)

	format := rw.DetectFormat("как работает RAG?")
	if format != rag.FormatQuestion {
		t.Errorf("expected FormatQuestion, got %v", format)
	}

	format = rw.DetectFormat("напиши код")
	if format != rag.FormatCommand {
		t.Errorf("expected FormatCommand, got %v", format)
	}

	format = rw.DetectFormat("just")
	if format != rag.FormatFragment {
		t.Errorf("expected FormatFragment for single word, got %v", format)
	}

	rewritten, err := rw.Rewrite(ctx, "как работают горутины в Go?", rag.FormatQuestion)
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	if rewritten == "" {
		t.Fatal("rewrite returned empty string")
	}
	t.Logf("Rewrite result: %q", rewritten)

	expanded, err := rw.Expand(ctx, "Go concurrency model")
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(expanded) == 0 {
		t.Fatal("expand returned 0 queries")
	}
	t.Logf("Expanded queries: %v", expanded)
}

func TestE2E_Reranker(t *testing.T) {
	ctx := context.Background()

	rr := rag.NewReranker(cfg.LLM.OllamaURL, "pdurugyan/qwen3-reranker-0.6b-q8_0", true)
	results := []store.SearchResult{
		{ID: "1", Payload: map[string]string{"content": "Go is a compiled language designed at Google for systems programming"}},
		{ID: "2", Payload: map[string]string{"content": "Python is an interpreted language known for its readability"}},
		{ID: "3", Payload: map[string]string{"content": "Go goroutines are lightweight threads managed by the runtime"}},
		{ID: "4", Payload: map[string]string{"content": "Rust is a systems language focused on safety and performance"}},
		{ID: "5", Payload: map[string]string{"content": "Go channels enable safe communication between goroutines"}},
	}

	reranked, err := rr.Rerank(ctx, "Go goroutines and channels", results, 3)
	if err != nil {
		t.Fatalf("Rerank: %v", err)
	}
	if len(reranked) == 0 {
		t.Fatal("rerank returned 0 results")
	}
	t.Logf("Reranked %d results:", len(reranked))
	for i, r := range reranked {
		t.Logf("  %d. id=%s score=%.4f content=%q…", i+1, r.ID, r.Score, r.Payload["content"][:min(40, len(r.Payload["content"]))])
	}
}

func TestE2E_FullRAGPipeline(t *testing.T) {
	s := requireStore(t)
	_ = requireOllamaChat(t)
	_ = requireOllamaEmbed(t)
	ctx := context.Background()

	content := `Go is a statically typed, compiled programming language designed at Google.
It has goroutines for lightweight concurrency and channels for communication between them.
Go is known for its simplicity, fast compilation, and built-in testing support.
The standard library includes HTTP server, JSON encoding, and cryptography packages.
Go is widely used for cloud-native applications, microservices, and DevOps tools.`

	docID := fmt.Sprintf("e2e-full-%d", time.Now().UnixNano())

	idx := rag.NewIndexer(s, embedClient)
	err := idx.IndexDocument(ctx, docID, content)
	if err != nil {
		t.Fatalf("IndexDocument: %v", err)
	}

	engine := rag.NewEngine(rag.EngineConfig{
		Store:         s,
		LLM:           llmClient,
		EmbedClient:   embedClient,
		OllamaURL:     cfg.LLM.OllamaURL,
		EmbedModel:    cfg.LLM.EmbedModel,
		RerankModel:   cfg.RAG.RerankModel,
		RerankEnabled: true,
	})

	resp, err := engine.Query(ctx, rag.RAGQuery{
		Query: "What is Go and what are its main features?",
		TopK:  3,
	})
	if err != nil {
		t.Fatalf("Engine.Query: %v", err)
	}

	if resp.Answer == "" {
		t.Fatal("RAG response has empty Answer")
	}
	t.Logf("Answer:\n%s", resp.Answer)

	if len(resp.Sources) == 0 {
		t.Fatal("RAG response has 0 Sources")
	}
	t.Logf("Sources (%d):", len(resp.Sources))
	for _, src := range resp.Sources {
		t.Logf("  [%d] %s (%s)", src.Index, src.Title, src.Type)
	}
}

func TestE2E_ParentLookup(t *testing.T) {
	s := requireStore(t)
	_ = requireOllamaEmbed(t)
	ctx := context.Background()

	embedding, err := embedClient.Embed(ctx, "test")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}

	parentPoint := store.NewPointStr("e2e-parent-1", embedding, map[string]string{
		"content": "Parent chunk with full context about Go programming language and its concurrency model",
		"document_id": "e2e-parent-doc",
		"chunk_type":  "parent",
		"parent_id":   "",
	})
	err = s.UpsertPoints(ctx, "documents", []*qdrant.PointStruct{parentPoint})
	if err != nil {
		t.Fatalf("Upsert parent: %v", err)
	}

	childResults := []store.SearchResult{
		{ID: "e2e-child-1", Payload: map[string]string{
			"parent_id": "e2e-parent-1", "content": "child1", "document_id": "e2e-parent-doc",
		}},
		{ID: "e2e-child-2", Payload: map[string]string{
			"parent_id": "e2e-parent-1", "content": "child2", "document_id": "e2e-parent-doc",
		}},
	}

	pl := rag.NewParentLookup(s)
	parents, err := pl.Lookup(ctx, childResults)
	if err != nil {
		t.Fatalf("ParentLookup: %v", err)
	}
	if len(parents) == 0 {
		t.Fatal("ParentLookup returned 0 results")
	}
	t.Logf("ParentLookup: %d parents, top=%s score=%.4f", len(parents), parents[0].ID, parents[0].Score)
}

func TestE2E_DeletePoints(t *testing.T) {
	s := requireStore(t)
	ctx := context.Background()

	err := s.DeletePoints(ctx, "documents", "e2e-test-delete")
	if err != nil {
		t.Fatalf("DeletePoints: %v", err)
	}
	t.Log("DeletePoints succeeded")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
