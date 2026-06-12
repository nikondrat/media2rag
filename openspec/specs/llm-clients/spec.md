## ADDED Requirements

### Requirement: LLMClient interface
The system SHALL define an `LLMClient` interface with `Chat`, `StreamChat`, and `Embed` methods.

#### Scenario: Interface compilation
- **WHEN** a struct implements `LLMClient` interface
- **THEN** it can be used wherever `LLLMClient` is expected

#### Scenario: Chat returns response
- **WHEN** `Chat(ctx, req)` is called with valid request
- **THEN** it returns `*ChatResponse` with content or error

#### Scenario: StreamChat returns channel
- **WHEN** `StreamChat(ctx, req)` is called with `Stream: true`
- **THEN** it returns `<-chan StreamDelta` that yields tokens

#### Scenario: Embed returns vector
- **WHEN** `Embed(ctx, "hello world")` is called
- **THEN** it returns `[]float32` embedding vector or error

### Requirement: Ollama client implementation
The `OllamaClient` SHALL communicate with Ollama via HTTP at `localhost:11434` using the Ollama API.

#### Scenario: Chat with Ollama
- **WHEN** `OllamaClient.Chat()` is called
- **THEN** POST request is sent to `http://localhost:11434/api/chat`

#### Scenario: Embed with Ollama
- **WHEN** `OllamaClient.Embed()` is called
- **THEN** POST request is sent to `http://localhost:11434/api/embed`

#### Scenario: Ollama unavailable
- **WHEN** Ollama is not running on localhost:11434
- **THEN** `Chat` returns `ErrLLMUnavailable` error

### Requirement: OpenRouter client implementation
The `OpenRouterClient` SHALL communicate with OpenAI-compatible API using Bearer token authentication.

#### Scenario: Chat with OpenRouter
- **WHEN** `OpenRouterClient.Chat()` is called
- **THEN** POST request is sent to configured URL with `Authorization: Bearer <key>`

#### Scenario: OpenRouter returns error
- **WHEN** API returns 401
- **THEN** client returns structured error with message

### Requirement: LLM client fallback
The system SHALL support fallback from Ollama to OpenRouter when Ollama is unavailable.

#### Scenario: Ollama fails, fallback succeeds
- **WHEN** Ollama returns error and OpenRouter is configured
- **THEN** request is retried with OpenRouter client
