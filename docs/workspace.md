# Workspace — v1 Design

## Структура

```
~/.media2rag/workspace/
├── <source-hash>/
│   ├── source.md              # сырой markdown от экстрактора
│   ├── .media2rag.yaml        # metadata
│   ├── versions/
│   │   ├── v1/
│   │   │   └── final.md       # результат CTG pipeline v1
│   │   ├── v2/
│   │   │   └── final.md       # результат повторной обработки
│   │   └── latest → v2        # symlink на последнюю версию
│   └── chunks/                # чанки для map-reduce (внутренние)
│       ├── chunk_001.md
│       ├── chunk_002.md
│       └── ...
└── ...
```

**source-hash** — SHA-256 от источника (URL или абсолютный путь):
```go
func SourceHash(source string) string {
    h := sha256.Sum256([]byte(source))
    return hex.EncodeToString(h[:8]) // первые 8 символов
}
```

## Metadata (.media2rag.yaml)

```yaml
source: "https://youtube.com/watch?v=abc123"
source_type: "url"
title: "Как масштабировать бизнес"
doc_type: "video"
language: "ru"
backend: "ollama"
model: "qwen3.5:27b"
word_count: 12400
created_at: 2025-05-25T10:00:00Z
updated_at: 2025-05-25T10:30:00Z
versions:
  - version: 1
    created_at: 2025-05-25T10:30:00Z
    backend: "ollama"
    model: "qwen3.5:27b"
  - version: 2
    created_at: 2025-05-25T11:00:00Z
    backend: "openai"
    model: "gpt-4o"
```

## Версионирование

При повторной обработке:
1. Находим папку по source-hash
2. Определяем следующую версию (max + 1)
3. Создаём `versions/vN/final.md`
4. Обновляем symlink `latest → vN`
5. Обновляем `.media2rag.yaml`

```bash
# Показать все версии
media2rag documents show <hash> --versions

# Показать конкретную версию
media2rag documents show <hash> --version 1

# Удалить версию
media2rag documents delete <hash> --version 1
```

## Очистка

```bash
# Удалить документ полностью
media2rag documents delete <hash> --all

# Удалить старые версии (оставить последние N)
media2rag documents cleanup --keep 3

# Удалить все документы старше N дней
media2rag documents cleanup --older-than 30d
```

## Qdrant связь

Каждая точка в Qdrant содержит `document_id` = source-hash. При удалении документа:
1. Удаляем папку из workspace
2. Удаляем точки из Qdrant по `document_id`
3. Обновляем `.media2rag.yaml`

## Workspace API (internal)

```go
type Workspace struct {
    root string // ~/.media2rag/workspace
}

func (w *Workspace) CreateDocument(source string) (*Document, error)
func (w *Workspace) GetDocument(hash string) (*Document, error)
func (w *Workspace) ListDocuments() ([]DocumentInfo, error)
func (w *Workspace) DeleteDocument(hash string, version int) error
func (w *Workspace) SaveVersion(hash string, content string) (int, error)
func (w *Workspace) GetLatestVersion(hash string) (*Version, error)
```

## Конфиг

```yaml
workspace:
  directory: ~/.media2rag/workspace
  max_versions: 5          # максимум версий на документ
  auto_cleanup: true       # автоудаление старых версий
  cleanup_after_days: 30   # удалять документы старше N дней
```
