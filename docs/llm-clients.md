# LLM Clients — v1 Design

## Philosophy

LLM — это просто HTTP. Никаких SDK, никаких обёрток. Go шлёт JSON в Ollama или OpenRouter и получает JSON обратно.

```
Go binary
  │
  ├── Ollama:      http://localhost:11434
  │     GET  /api/tags          → список моделей
  │     POST /api/chat          → chat completion
  │     POST /api/embed         → эмбеддинги
  │
  └── OpenRouter:  https://openrouter.ai/api/v1
        POST /chat/completions  → chat completion
        POST /embeddings        → эмбеддинги
```

## Interface

```go
// ProviderType — какой бэкенд используется
type ProviderType string

const (
    ProviderOllama     ProviderType = "ollama"
    ProviderOpenRouter ProviderType = "openrouter"
)

// LLMClient — единый интерфейс для любого бэкенда.
type LLMClient interface {
    // Chat — полный ответ (не streaming).
    Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
    // StreamChat — streaming ответ через канал.
    StreamChat(ctx context.Context, req ChatRequest) (<-chan StreamDelta, error)
    // Embed — получение эмбеддинга текста.
    Embed(ctx context.Context, text string) ([]float32, error)
}

// ChatRequest — универсальный запрос для любого бэкенда.
type ChatRequest struct {
    Model     string
    Messages  []Message
    Stream    bool
    Reasoning bool      // включать chain-of-thought
    Images    []string  // base64-encoded изображения
}

type Message struct {
    Role    string // "system", "user", "assistant"
    Content string
}

type ChatResponse struct {
    Content   string
    Reasoning string
    Model     string
    Usage     Usage
}

type Usage struct {
    PromptTokens     int
    CompletionTokens int
    TotalTokens      int
}

type StreamDelta struct {
    Content   string
    Reasoning string
}
```

## Конфигурация

```go
type LLMConfig struct {
    // Бэкенд по умолчанию
    DefaultBackend ProviderType // "ollama" или "openrouter"

    // Ollama
    OllamaURL  string // http://localhost:11434
    OllamaModel string

    // OpenRouter
    OpenRouterKey   string
    OpenRouterURL   string // https://openrouter.ai/api/v1
    OpenRouterModel string

    // Эмбеддинги (отдельная модель для экономии)
    EmbeddingURL   string // http://localhost:11435 (второй Ollama)
    EmbeddingModel string // nomic-embed-text / qwen3-embedding
}
```

## Реализации

### OllamaClient

```go
type OllamaClient struct {
    baseURL string
    model   string
    http    *http.Client
}

func (c *OllamaClient) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
    body := map[string]any{
        "model":    req.Model,
        "messages": req.Messages,
        "stream":   false,
    }
    if req.Reasoning {
        body["think"] = true
    }

    resp, err := c.post(ctx, "/api/chat", body)
    // парсим response.message.content
}

func (c *OllamaClient) StreamChat(ctx context.Context, req ChatRequest) (<-chan StreamDelta, error) {
    ch := make(chan StreamDelta)

    body := map[string]any{
        "model":    req.Model,
        "messages": req.Messages,
        "stream":   true,
    }

    // HTTP streaming: читаем построчно, парсим JSON
    // Каждая строка: {"message":{"content":"...","reasoning":"..."},"done":false}
    go func() {
        defer close(ch)
        scanner := bufio.NewScanner(resp.Body)
        for scanner.Scan() {
            var delta struct {
                Message struct {
                    Content   string `json:"content"`
                    Reasoning string `json:"reasoning"`
                } `json:"message"`
                Done bool `json:"done"`
            }
            json.Unmarshal(scanner.Bytes(), &delta)
            ch <- StreamDelta{
                Content:   delta.Message.Content,
                Reasoning: delta.Message.Reasoning,
            }
            if delta.Done {
                return
            }
        }
    }()

    return ch, nil
}

func (c *OllamaClient) Embed(ctx context.Context, text string) ([]float32, error) {
    body := map[string]any{
        "model":  c.embedModel, // отдельная модель
        "input":  text,
    }
    resp, err := c.post(ctx, "/api/embed", body)
    // парсим resp.embedding
}
```

### OpenRouterClient

```go
type OpenRouterClient struct {
    apiKey  string
    baseURL string
    model   string
    http    *http.Client
}

func (c *OpenRouterClient) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
    body := map[string]any{
        "model":    req.Model,
        "messages": req.Messages,
        "stream":   false,
    }
    if req.Reasoning {
        body["reasoning"] = map[string]any{"max_tokens": 2048}
    }

    resp, err := c.post(ctx, "/chat/completions", body)
    // парсим resp.choices[0].message.content
}

func (c *OpenRouterClient) StreamChat(ctx context.Context, req ChatRequest) (<-chan StreamDelta, error) {
    // SSE streaming: читаем "data: {...}" строки
    // Каждая строка: data: {"choices":[{"delta":{"content":"..."}}]}
}

func (c *OpenRouterClient) Embed(ctx context.Context, text string) ([]float32, error) {
    body := map[string]any{
        "model": "text-embedding-3-small",  // или другая
        "input": text,
    }
    resp, err := c.post(ctx, "/embeddings", body)
    // парсим resp.data[0].embedding
}
```

## Fallback Logic

Если Ollama недоступен (connection refused, timeout) — автоматический фоллбэк на OpenRouter:

```go
type FallbackClient struct {
    primary   LLMClient // Ollama
    secondary LLMClient // OpenRouter
}

func (c *FallbackClient) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
    resp, err := c.primary.Chat(ctx, req)
    if err == nil {
        return resp, nil
    }

    // Ollama не ответил — пробуем OpenRouter
    log.Printf("Ollama failed: %v, falling back to OpenRouter", err)
    return c.secondary.Chat(ctx, req)
}
```

## Retry Policy

```go
type RetryConfig struct {
    MaxAttempts int           // 3
    BaseDelay   time.Duration // 1 second
    MaxDelay    time.Duration // 10 seconds
}
```

- Ретраим только на `connection refused`, `timeout`, `5xx`
- Не ретраим на `4xx` (кроме `429` — rate limit)
- Exponential backoff + jitter

## Provider Selection

```go
// NewLLMClient создаёт клиента по типу бэкенда.
func NewLLMClient(provider ProviderType, cfg LLMConfig) LLMClient {
    switch provider {
    case ProviderOllama:
        return &OllamaClient{...}
    case ProviderOpenRouter:
        return &OpenRouterClient{...}
    default:
        return &FallbackClient{
            primary:   &OllamaClient{...},
            secondary: &OpenRouterClient{...},
        }
    }
}
```

При инициализации:
1. Если указан `--backend ollama` — только Ollama
2. Если указан `--backend openrouter` — только OpenRouter
3. Если не указан — FallbackClient (Ollama → OpenRouter)

## Таймауты

| Операция | Таймаут |
|----------|---------|
| Chat (non-streaming) | 60s |
| Chat (streaming) | 300s (5 min) |
| Embed | 30s |
| Первый байт (streaming) | 15s |
