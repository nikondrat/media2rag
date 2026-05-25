# Vector Store — Qdrant Schema

## Коллекции

**Две коллекции:**

| Коллекция | Размер чанка | Назначение |
|-----------|-------------|------------|
| `parents` | ~512 токенов | Хранение контекста для LLM |
| `children` | ~128 токенов | Точный поиск по запросу |

**Как работает parent-child:**
1. Индексируем мелкие чанки (children) — они точнее матчатся на запрос
2. После поиска → достаём parent по `parent_id`
3. Отдаём LLM parent-чанки — они дают полный контекст

```
Запрос → поиск в children → parent_id → lookup в parents → контекст для LLM
```

## Embedding модель

**Не привязаны к конкретной модели.** Размерность вектора задаётся в конфиге:

```yaml
llm:
  embedding_model: qwen3-embedding:0.6b  # или nomic-embed-text, text-embedding-3-small, etc.
  embedding_dim: 1024                     # размерность под модель
```

Qdrant коллекция создаётся с размерностью, соответствующей модели. Если модель меняется — нужна новая коллекция (либо реиндексация).

## Parent collection: `parents`

```go
type ParentPoint struct {
    ID      string  // UUIDv4
    Vector  []float32 // dense embedding

    Payload struct {
        DocumentID  string `json:"document_id"`
        DocType     string `json:"doc_type"`     // transcript, article, pdf, markdown
        Source      string `json:"source"`       // URL или путь
        Title       string `json:"title"`
        Content     string `json:"content"`        // текст чанка
        ChunkIndex  int    `json:"chunk_index"`    // порядковый номер
        Section     string `json:"section,omitempty"`
        Language    string `json:"language,omitempty"`
        ContentHash string `json:"content_hash"`   // SHA-256 для дедупликации
        CreatedAt   int64  `json:"created_at"`
    }
}
```

**Индексы (создаются ДО вставки):**

| Field | Type |
|-------|------|
| `document_id` | keyword |
| `doc_type` | keyword |
| `content_hash` | keyword |
| `created_at` | integer |

## Children collection: `children`

```go
type ChildPoint struct {
    ID      string  // UUIDv4
    Vector  []float32 // dense embedding

    Payload struct {
        DocumentID  string `json:"document_id"`
        ParentID    string `json:"parent_id"`      // ссылка на parent
        Content     string `json:"content"`        // текст чанка
        ChunkIndex  int    `json:"chunk_index"`
        ContentHash string `json:"content_hash"`
        CreatedAt   int64  `json:"created_at"`
    }
}
```

**Индексы:**

| Field | Type |
|-------|------|
| `document_id` | keyword |
| `parent_id` | keyword |
| `content_hash` | keyword |
| `created_at` | integer |

## Sparse vectors (BM25)

Обе коллекции содержат sparse vector для гибридного поиска:

```go
// Qdrant поддерживает sparse vectors из коробки (v1.10+)
// Строятся автоматически из текста при индексации
type SparseVectorParams struct {
    Index *SparseIndexConfig{
        FullScanThreshold: 10000, // переключение на full scan при < 10K точек
    }
}
```

Гибридный поиск: dense (семантический) + sparse (BM25) через RRF fusion.

## Hybrid Search

```go
func (s *Store) HybridSearch(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
    // Параллельные prefetch: dense + sparse
    // RRF fusion через Qdrant
    results := s.client.Query(ctx, &qdrant.QueryPoints{
        Collection: "children",
        Prefetch: []*PrefetchQuery{
            {Query: denseVector, Using: "dense", Limit: req.TopK * 2},
            {Query: queryText,  Using: "sparse", Limit: req.TopK * 2},
        },
        Query:  fusionQuery, // RRF
        Filter: buildFilter(req),
        Limit:  req.TopK,
        WithPayload: true,
    })
}
```

## Parent lookup

```go
func (s *Store) GetParents(ctx context.Context, results []SearchResult) ([]ParentPoint, error) {
    parentIDs := uniqueParentIDs(results)
    // lookup в parents по parent_id
    points := s.client.Get(ctx, &qdrant.GetPoints{
        Collection: "parents",
        IDs:        parentIDs,
    })
    // маппинг: child → parent
}
```

## Создание коллекций

```go
func (s *Store) InitCollections(ctx context.Context, dim int) error {
    for _, name := range []string{"parents", "children"} {
        s.client.CreateCollection(ctx, &qdrant.CreateCollection{
            CollectionName: name,
            VectorsConfig: VectorsConfig{
                Size: dim,  // из config.yaml
                Distance: Distance_Cosine,
            },
            SparseVectorsConfig: SparseVectorConfig{
                Sparse: &SparseVectorParams{},
            },
        })
        // payload индексы
    }
}
```

## Конфиг

```yaml
qdrant:
  url: localhost:6334  # gRPC порт
  timeout: 30s

llm:
  embedding_model: qwen3-embedding:0.6b
  embedding_dim: 1024          # размерность под модель
  embedding_url: http://localhost:11434  # Ollama для эмбеддингов
```
