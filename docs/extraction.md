# Extraction — v1 Design

## Philosophy

Любой входной формат превращается в **Markdown**. CTG Pipeline никогда не видит сырой PDF, аудио или видео — только Markdown. Это единый формат для всего пайплайна.

```
Input ──► Extractor ──► Markdown ──► Pipeline
  ↑                      ↑
  разные форматы          единый формат
```

**Ключевое открытие:** `npx rdrr` ([github.com/fkonovalov/rdrr](https://github.com/fkonovalov/rdrr)) конвертирует **любой URL** в чистый Markdown. Веб-страницы, YouTube, GitHub issues, X/Twitter, PDF по ссылке — всё это rdrr превращает в MD. Нет отдельного экстрактора для YouTube или для веба — есть один URL экстрактор.

## Extractor Interface

```go
type Extractor interface {
    // Detect возвращает true, если этот экстрактор может обработать путь/URL.
    Detect(path string) bool

    // Extract возвращает Markdown.
    Extract(ctx context.Context, path string) (string, error)
}
```

**Extract возвращает только string (Markdown).** Metadata (title, source, type) извлекаются на этапе Compress в Pipeline.

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

### 1. URLExtractor (npx rdrr)

**Detect:** Начинается с `http://` или `https://`

**Extract:**
```bash
npx rdrr <url> --json
```

Флаг `--json` даёт структурированный вывод с metadata:
```json
{
    "type": "youtube" | "webpage" | "github" | "pdf" | "x-profile",
    "title": "...",
    "content": "clean markdown...",
    "description": "...",
    "siteName": "...",
    "wordCount": 2847,
    "published": "2024-01-01"
}
```

```go
type URLExtractor struct{}

func (e *URLExtractor) Detect(path string) bool {
    return strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://")
}

func (e *URLExtractor) Extract(ctx context.Context, url string) (string, error) {
    cmd := exec.CommandContext(ctx, "npx", "rdrr", url, "--json")
    out, err := cmd.Output()
    if err != nil {
        return "", fmt.Errorf("rdrr failed: %w", err)
    }

    var result rdrrResult
    if err := json.Unmarshal(out, &result); err != nil {
        // fallback: rdrr без --json
        return string(out), nil
    }

    // Сохраняем metadata в контекст для pipeline
    ctx = context.WithValue(ctx, ctxKeyMetadata, result.Metadata())
    return result.Content, nil
}
```

**Что rdrr умеет:**
- Веб-страницы: `rdrr https://react.dev/learn`
- YouTube транскрипты: `rdrr https://www.youtube.com/watch?v=...`
- GitHub issues: `rdrr https://github.com/.../issues/1`
- X/Twitter: `rdrr https://x.com/user`
- PDF по ссылке: `rdrr https://example.com/doc.pdf`

### 2. LocalFileExtractor (Go-native)

**Detect:** Любой локальный путь (не URL)

**Extract:**
- Если `.md` — passthrough (с парсингом frontmatter)
- Если другой формат — в v1 возвращаем ошибку "unsupported format"

```go
type LocalFileExtractor struct{}

func (e *LocalFileExtractor) Detect(path string) bool {
    return !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://")
}

func (e *LocalFileExtractor) Extract(ctx context.Context, path string) (string, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return "", err
    }

    ext := strings.ToLower(filepath.Ext(path))
    switch ext {
    case ".md", ".markdown":
        return parseMarkdown(data)
    default:
        return "", fmt.Errorf("unsupported file format: %s", ext)
    }
}

func parseMarkdown(data []byte) (string, error) {
    content := string(data)
    // Парсим YAML frontmatter если есть
    if fm, rest, ok := parseFrontmatter(content); ok {
        // metadata доступен через контекст
        return rest, nil
    }
    return content, nil
}
```

### 3. Экстракторы v2+

| Формат | Механизм | Статус |
|--------|----------|--------|
| PDF (локальный) | unipdf / Python subprocess | v2 |
| EPUB (локальный) | go-epub | v2 |
| Аудио (локальное) | whisper subprocess | v2 |
| Изображение | Ollama vision HTTP | v3 |

## Как это работает вместе

```
process "https://youtube.com/..."
  → Detect: starts with http:// → URLExtractor
  → npx rdrr <url> --json → Markdown + metadata

process "https://habr.com/articles/..."
  → Detect: starts with http:// → URLExtractor
  → npx rdrr <url> --json → Markdown + metadata

process "./notes.md"
  → Detect: local path → LocalFileExtractor
  → os.ReadFile → Markdown

process "./doc.pdf"
  → Detect: local path → LocalFileExtractor
  → Error: unsupported format (v1)
```

Только два экстрактора на v1. Всё URL → rdrr. Всё локальное → LocalFileExtractor (только `.md`).

## Future Ideas (на подумать)

### Всё через rdrr + временный HTTP сервер

Для локальных файлов, которые rdrr умеет обрабатывать (PDF, HTML), можно поднять временный HTTP сервер в Go и скормить URL rdrr:

```
./document.pdf
  │
  ▼
Go: запускает http.Server на localhost:0
  │
  ▼
npx rdrr http://localhost:XXXX/document.pdf
  │
  ▼
Markdown
```

**Когда пригодится:** когда понадобится обрабатывать PDF-файлы без Python.

### Whisper для аудио/видео

rdrr не умеет транскрибировать аудио и видео. Для YouTube он берёт готовые субтитры, но для локальных MP3/MP4 нужен Whisper.

**Варианты:**
- `whisper` subprocess (Python) — как сейчас
- `whisper.cpp` CGo — быстрее, без Python
- `whisper-rs` — Rust биндинги через CGo

### EPUB

Для EPUB нужен парсер контейнера. rdrr не поддерживает. Варианты:
- `go-epub` — Go-native библиотека
- Python subprocess через ebooklib

### Telegram каналы

rdrr умеет отдельные посты, но не умеет скачивать историю канала. Для batch-загрузки каналов:
- `gotd` (Go-native Telegram клиент)
- Telethon subprocess (Python)

```
~/.media2rag/workspace/
  ├── <source-hash>/
  │   ├── source.md         # raw markdown from extractor
  │   ├── output/
  │   │   └── final.md      # результат CTG pipeline
  │   └── .media2rag.yaml   # metadata (source, type, title, timestamps)
  └── ...
```
