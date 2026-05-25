# LLM Clients — v1 Design

## Philosophy

LLM — это просто HTTP. Никаких SDK. Go шлёт JSON и получает JSON обратно.

Два типа провайдеров:
- **Ollama** — локальный (свой API `/api/chat`, `/api/embed`)
- **OpenAI-compatible** — любой API в формате OpenAI (OpenRouter, OpenAI, Groq, Together, etc.)

## Interface

```go
type LLMClient interface {
    Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
    StreamChat(ctx context.Context, req ChatRequest) (<-chan StreamDelta, error)
    Embed(ctx context.Context, text string) ([]float32, error)
}

type ChatRequest struct {
    Model     string
    Messages  []Message
    Stream    bool
    Reasoning bool
    Images    []string // base64
}

type Message struct {
    Role    string // system, user, assistant
    Content string
}

type ChatResponse struct {
    Content   string
    Reasoning string
    Model     string
    Usage     Usage
}

type StreamDelta struct {
    Content   string
    Reasoning string
}
```

## Реализации

### OllamaClient

```go
type OllamaClient struct {
    baseURL    string // http://localhost:11434
    http       *http.Client
}

// POST /api/chat — chat completion
// POST /api/embed — embeddings
// GET /api/tags — список моделей
```

### OpenAIClient (единый для всех OpenAI-compatible)

```go
type OpenAIClient struct {
    baseURL string // openrouter.ai, api.openai.com, api.groq.com, etc.
    apiKey  string
    http    *http.Client
}

// POST /chat/completions — chat completion
// POST /embeddings — embeddings
```

Один класс для любого OpenAI-совместимого API. Разница только в `baseURL` и `apiKey`.

## Конфигурация

В `~/.media2rag/config.yaml`:

```yaml
llm:
  backend: ollama          # ollama | openai
  model: qwen3.5:27b       # модель по умолчанию

  ollama:
    base_url: http://localhost:11434

  openai:
    base_url: https://openrouter.ai/api/v1
    api_key: ${OPENROUTER_API_KEY}  # или sk-..., или из env
    model: gpt-4o
```

CLI флаги переопределяют config:

```bash
# Ollama по дефолту
media2rag process ./notes.md

# OpenRouter
media2rag process ./notes.md --backend openai --model gpt-4o

# OpenAI
media2rag process ./notes.md --backend openai --base-url https://api.openai.com/v1

# Groq
media2rag process ./notes.md --backend openai --base-url https://api.groq.com/openai/v1
```

## Fallback

Если не указан `--backend` — Ollama. Если Ollama не отвечает (connection refused) — ошибка, не фоллбэк. Фоллбэк на другой провайдер только если явно настроен в конфиге.

## Provider Selection

```go
func NewLLMClient(cfg LLMConfig) LLMClient {
    switch cfg.Backend {
    case "ollama":
        return &OllamaClient{
            baseURL: cfg.Ollama.BaseURL,
        }
    case "openai":
        return &OpenAIClient{
            baseURL: cfg.OpenAI.BaseURL,
            apiKey:  cfg.OpenAI.APIKey,
        }
    default:
        return &OllamaClient{...} // fallback
    }
}
```

## Таймауты

| Операция | Таймаут |
|----------|---------|
| Chat (non-streaming) | 60s |
| Chat (streaming) | 300s |
| Embed | 30s |
| Первый токен (streaming) | 15s |
