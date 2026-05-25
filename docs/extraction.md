# Extraction — v1 Design

## Philosophy

Любой входной формат превращается в **Markdown**. CTG Pipeline никогда не видит сырой PDF, аудио или видео — только Markdown. Это единый формат для всего пайплайна.

```
File ──► Extractor ──► Markdown ──► Pipeline
  ↑                      ↑
  разные форматы          единый формат
```

## Extractor Interface

```go
type Extractor interface {
    // Detect возвращает true, если этот экстрактор может обработать путь
    Detect(path string) bool

    // Extract возвращает Markdown
    Extract(ctx context.Context, path string) (string, error)
}
```

**Extract возвращает только string (Markdown).** Никаких ExtractedContent структур — это избыточно. Metadata (title, source, type) парсятся из frontmatter там, где он есть, или извлекаются на этапе Compress в Pipeline.

## Registry

```go
type Registry struct {
    extractors []Extractor
}

func (r *Registry) Find(path string) (Extractor, error)
// Пробегает по экстракторам, вызывает Detect, возвращает первый подходящий.
```

Экстракторы регистрируются в порядке приоритета (самые специфичные — первые).

## Экстракторы v1

### 1. MarkdownExtractor (Go-native)

**Detect:** `.md` extension

**Extract:**
- Читает файл
- Парсит YAML frontmatter (если есть)
- Возвращает чистый Markdown (без frontmatter) + сохраняет metadata для pipeline

```go
type MarkdownExtractor struct{}

func (e *MarkdownExtractor) Extract(ctx context.Context, path string) (string, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return "", err
    }
    content := string(data)

    // Try parse frontmatter
    if fm, rest, ok := parseFrontmatter(content); ok {
        // fm доступен для pipeline через контекст
        ctx = context.WithValue(ctx, ctxKeyMetadata, fm)
        return rest, nil
    }

    return content, nil
}
```

### 2. YouTubeExtractor (npx rdrr subprocess)

**Detect:** YouTube URL (youtube.com, youtu.be)

**Extract:**
```
npx rdrr <url> → stdout (Markdown)
```

```go
type YouTubeExtractor struct{}

func (e *YouTubeExtractor) Extract(ctx context.Context, url string) (string, error) {
    cmd := exec.CommandContext(ctx, "npx", "rdrr", url)
    out, err := cmd.Output()
    if err != nil {
        return "", fmt.Errorf("rdrr failed: %w", err)
    }
    return string(out), nil
}
```

### 3. TelegramExtractor (v2)

**Detect:** Telegram channel/group URL

**Extract:** TBD (возможно telethon через subprocess, или gotd native)

## Поддержка URL в process команде

```bash
media2rag process "https://youtube.com/watch?v=..."
media2rag process ./path/to/file.md
media2rag process ./path/to/file.pdf  # v2+
```

Extractor Registry определяет по пути/URL какой экстрактор вызвать.

## Workspace Structure

Результат экстракции сохраняется во временный workspace перед pipeline:

```
~/.media2rag/workspace/
  ├── <source-hash>/
  │   ├── source.md         # raw markdown from extractor
  │   ├── output/
  │   │   └── final.md      # результат CTG pipeline
  │   └── .media2rag.yaml   # metadata (source, type, title, timestamps)
  └── ...
```

## План расширения

| Формат | v1 | v2 | v3 |
|--------|-----|----|----|
| Markdown | Go-native | — | — |
| YouTube (rdrr) | Go + npx subprocess | — | — |
| YouTube (native) | — | yt-dlp + whisper.cpp CGo | — |
| PDF | — | Python subprocess (PyMuPDF) | Go-native (unipdf) |
| EPUB | — | Python subprocess (ebooklib) | Go-native (go-epub) |
| Audio | — | whisper subprocess | whisper.cpp CGo |
| Image | — | Ollama vision HTTP | — |
| Telegram | — | gotd native | — |
