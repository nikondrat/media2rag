package llm

import (
	"context"
	"testing"

	"media2rag/internal/model"
)

func eq(t *testing.T, got, want []model.TypedBlock) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len: got %d, want %d\n  got:  %+v\n  want: %+v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i].Type != want[i].Type {
			t.Fatalf("[%d] Type: got %q, want %q", i, got[i].Type, want[i].Type)
		}
		if got[i].Content != want[i].Content {
			t.Fatalf("[%d] Content: got %q, want %q", i, got[i].Content, want[i].Content)
		}
		if len(got[i].Params) != len(want[i].Params) {
			t.Fatalf("[%d] Params len: got %d, want %d\n  got:  %+v\n  want: %+v", i, len(got[i].Params), len(want[i].Params), got[i].Params, want[i].Params)
		}
		for k, v := range want[i].Params {
			if got[i].Params[k] != v {
				t.Fatalf("[%d] Params[%q]: got %q, want %q", i, k, got[i].Params[k], v)
			}
		}
	}
}

func TestParseOutput_SingleBlock(t *testing.T) {
	blocks, err := ParseOutput("> memory\nПользователя зовут Никита\n<")
	if err != nil {
		t.Fatal(err)
	}
	eq(t, blocks, []model.TypedBlock{
		{Type: "memory", Content: "Пользователя зовут Никита"},
	})
}

func TestParseOutput_WithParams(t *testing.T) {
	blocks, err := ParseOutput("> topic: chunk=3, lang=ru\nHNSW, векторный поиск\n<")
	if err != nil {
		t.Fatal(err)
	}
	eq(t, blocks, []model.TypedBlock{
		{Type: "topic", Params: map[string]string{"chunk": "3", "lang": "ru"}, Content: "HNSW, векторный поиск"},
	})
}

func TestParseOutput_MultipleBlocks(t *testing.T) {
	input := "> topic\ntopic1\n<\n> summary\nsummary text\n<"
	blocks, err := ParseOutput(input)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, blocks, []model.TypedBlock{
		{Type: "topic", Content: "topic1"},
		{Type: "summary", Content: "summary text"},
	})
}

func TestParseOutput_PlainTextFallback(t *testing.T) {
	input := "Hello, this is plain text without markers"
	blocks, err := ParseOutput(input)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, blocks, []model.TypedBlock{
		{Type: "text", Content: "Hello, this is plain text without markers"},
	})
}

func TestParseOutput_MissingClosingMarker(t *testing.T) {
	input := "> memory\nThis block has no closing marker"
	blocks, err := ParseOutput(input)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, blocks, []model.TypedBlock{
		{Type: "memory", Content: "This block has no closing marker"},
	})
}

func TestParseOutput_WithGreaterThanInContent(t *testing.T) {
	input := "> quote\nLife is > everything\n<"
	blocks, err := ParseOutput(input)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, blocks, []model.TypedBlock{
		{Type: "quote", Content: "Life is > everything"},
	})
}

type mockLLMClient struct {
	content string
}

func (m *mockLLMClient) Chat(ctx context.Context, req model.ChatRequest) (*model.ChatResponse, error) {
	return &model.ChatResponse{
		Message: model.Message{Content: m.content, Role: "assistant"},
		Done:    true,
	}, nil
}

func (m *mockLLMClient) ChatAndParse(ctx context.Context, req model.ChatRequest) ([]model.TypedBlock, error) {
	resp, err := m.Chat(ctx, req)
	if err != nil {
		return nil, err
	}
	return ParseOutput(resp.Message.Content)
}
func (m *mockLLMClient) StreamChat(ctx context.Context, req model.ChatRequest) (<-chan model.StreamDelta, error) { return nil, nil }
func (m *mockLLMClient) StreamAndParse(ctx context.Context, req model.ChatRequest) (<-chan model.StreamDelta, chan []model.TypedBlock, error) { return nil, nil, nil }
func (m *mockLLMClient) Embed(ctx context.Context, text string) ([]float32, error) { return nil, nil }
func (m *mockLLMClient) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) { return nil, nil }

func TestChatAndParse_Mock(t *testing.T) {
	mock := &mockLLMClient{content: "> memory\nuser=nikita\n<"}
	blocks, err := mock.ChatAndParse(context.Background(), model.ChatRequest{
		Messages: []model.Message{{Role: "user", Content: "remember"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	eq(t, blocks, []model.TypedBlock{
		{Type: "memory", Content: "user=nikita"},
	})
}
