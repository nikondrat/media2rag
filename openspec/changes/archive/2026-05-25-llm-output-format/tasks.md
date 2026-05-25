## 1. TypedBlock Model (`internal/model/`)

- [x] 1.1 Add `TypedBlock` struct: Type, Params map[string]string, Content string

## 2. Output Parser (`internal/llm/parse.go`)

- [x] 2.1 Implement `ParseOutput(text string) ([]TypedBlock, error)`
- [x] 2.2 Handle `> type(params)\ncontent\n<` pattern
- [x] 2.3 Handle params parsing: `key=value, key2=value2`
- [x] 2.4 Handle multiple blocks in single response
- [x] 2.5 Handle plain text fallback (no markers → single "text" block)
- [x] 2.6 Handle missing `<` closing marker
- [x] 2.7 Handle `>` in content without confusing with marker

## 3. LLM Client Integration (`internal/llm/client.go`)

- [x] 3.1 Add `ChatAndParse(ctx, prompt) ([]TypedBlock, error)` to interface
- [x] 3.2 Implement `ChatAndParse` for OllamaClient
- [x] 3.3 Implement `ChatAndParse` for OpenRouterClient
- [x] 3.4 Add `StreamAndParse` to interface
- [x] 3.5 Implement `StreamAndParse` for OllamaClient (stream tokens, parse on done)
- [x] 3.6 Implement `StreamAndParse` for OpenRouterClient

## 4. Tests

- [x] 4.1 Test `ParseOutput` with single block
- [x] 4.2 Test `ParseOutput` with params
- [x] 4.3 Test `ParseOutput` with multiple blocks
- [x] 4.4 Test `ParseOutput` plain text fallback
- [x] 4.5 Test `ParseOutput` missing closing marker
- [x] 4.6 Test `ParseOutput` with `>` in content
- [x] 4.7 Test `ChatAndParse` with mock LLM

## 5. Integration

- [x] 5.1 `go build ./...` succeeds
- [x] 5.2 `go test ./internal/llm/...` passes
