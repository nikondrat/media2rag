# Vector Store — Qdrant Schema

## Collections

**Одна коллекция: `chunks`**

Parent и child чанки живут в одной коллекции. Различаются полем `parent_id`:
- Если `parent_id == ""` — это parent chunk (крупный, 512 токенов)
- Если `parent_id != ""` — это child chunk (мелкий, 128 токенов)

Это упрощает поиск: один запрос в одну коллекцию. Parent lookup через `parent_id` на клиенте (Go).

## Схема точек

```go
type Point struct {
    ID           string    // UUIDv4 (content_hash based)
    Vector       []float32 // dense embedding (768d for nomic-embed-text)
    SparseVector []SparseValue // sparse for BM25

    Payload struct {
        // Идентификация
        DocumentID  string `json:"document_id"`
        DocType     string `json:"doc_type"`     // transcript, article, pdf, markdown
        Source      string `json:"source"`       // URL или путь к файлу
        Title       string `json:"title"`

        // Чанк
        Content     string `json:"content"`        // текст чанка
        ChunkIndex  int    `json:"chunk_index"`    // порядковый номер
        ParentID    string `json:"parent_id"`      // пусто — значит сам parent
        ContentHash string `json:"content_hash"`   // SHA-256 для дедупликации

        // Метаданные
        Section     string `json:"section,omitempty"`
        Language    string `json:"language,omitempty"`
        Topics      string `json:"topics,omitempty"` // comma-separated
        CreatedAt   int64  `json:"created_at"`       // unix timestamp
    }
}
```

**Зачем ContentHash в payload:** дедупликация на клиенте при поиске (проверять `seenHashes`).

## Индексы (payload fields)

Создаются ДО вставки данных (Qdrant строит HNSW граф с учётом индексов):

| Field | Type | Зачем |
|-------|------|-------|
| `document_id` | keyword | Фильтр поиска по конкретному документу |
| `doc_type` | keyword | Фильтр по типу контента |
| `parent_id` | keyword | Поиск parent по id |
| `content_hash` | keyword | Для дедупликации на серверной стороне |
| `created_at` | integer | Сортировка/фильтр по дате |

**Не индексируем:** `content` (текст) — он идёт через sparse vector / dense vector.

## Вектора

### Dense (плотный)

- Размерность: зависит от embedding модели
  - `nomic-embed-text`: 768d
  - `text-embedding-3-small`: 1536d
  - `qwen3-embedding`: зависит от модели
- Distance: `Cosine`
- Индекс: `HNSW` (дефолтный)

### Sparse (разреженный)

- Встроенная поддержка Qdrant (v1.10+)
- Строится автоматически из текста
- Используется для BM25-style поиска
- Не требует отдельного FTS индекса

## Hybrid Search

```go
type HybridSearchRequest struct {
    QueryText  string   // оригинальный запрос
    QueryEmbed []float32 // эмбеддинг запроса
    TopK       int
    DocumentID string   // опционально: фильтр по документу
}

type SearchResult struct {
    Point
    Score float64
}
```

Qdrant выполняет prefetch для dense и sparse параллельно, затем RRF (Reciprocal Rank Fusion):

```go
// Псевдокод
func (s *Store) HybridSearch(ctx context.Context, req HybridSearchRequest) ([]SearchResult, error) {
    filter := buildFilter(req.DocumentID)

    results, err := s.client.Query(ctx, &qdrant.QueryPoints{
        Collection: "chunks",
        Prefetch: []*qdrant.PrefetchQuery{
            {Query: req.QueryEmbed, Using: "dense", Limit: req.TopK * 2},
            {Query: req.QueryText, Using: "sparse", Limit: req.TopK * 2},
        },
        Query:     fusionQuery, // RRF fusion
        Filter:    filter,
        Limit:     req.TopK,
        WithPayload: true,
    })
}
```

## Batch Upsert

```go
func (s *Store) UpsertChunks(ctx context.Context, chunks []Point) error {
    batchSize := 100
    for i := 0; i < len(chunks); i += batchSize {
        end := min(i+batchSize, len(chunks))
        batch := chunks[i:end]
        points := toQdrantPoints(batch)
        _, err := s.client.UpsertPoints(ctx, &qdrant.UpsertPoints{
            Collection: "chunks",
            Points:     points,
        })
        if err != nil {
            return err
        }
    }
    return nil
}
```

## Parent-Child Lookup

После поиска child-чанков, заменяем их на parent-чанки для контекста:

```go
func (s *Store) GetParentChunks(ctx context.Context, results []SearchResult) ([]SearchResult, error) {
    parentIDs := extractUniqueParentIDs(results)
    points, err := s.client.GetPoints(ctx, &qdrant.GetPoints{
        Collection: "chunks",
        IDs:        parentIDs,
    })
    // Группируем child → parent, усредняем scores
}
```

## Инициализация

```go
func (s *Store) Init(ctx context.Context) error {
    collections, err := s.client.ListCollections(ctx)
    if err != nil {
        return err
    }

    // Создаём коллекцию если нет
    if !hasCollection(collections, "chunks") {
        _, err := s.client.CreateCollection(ctx, &qdrant.CreateCollection{
            CollectionName: "chunks",
            VectorsConfig: &qdrant.VectorsConfig{
                Dense: &qdrant.VectorParams{
                    Size:     768, // nomic-embed-text
                    Distance: qdrant.Distance_Cosine,
                },
            },
            SparseVectorsConfig: &qdrant.SparseVectorConfig{
                Sparse: &qdrant.SparseVectorParams{
                    Index: &qdrant.SparseIndexConfig{
                        FullScanThreshold: 10000,
                    },
                },
            },
        })
        if err != nil {
            return err
        }
    }

    // Создаём payload индексы
    indexes := map[string]qdrant.FieldType{
        "document_id":   qdrant.FieldType_Keyword,
        "doc_type":      qdrant.FieldType_Keyword,
        "parent_id":     qdrant.FieldType_Keyword,
        "content_hash":  qdrant.FieldType_Keyword,
        "created_at":    qdrant.FieldType_Integer,
    }
    for field, typ := range indexes {
        s.client.CreateFieldIndex(ctx, &qdrant.CreateFieldIndex{
            Collection: "chunks",
            FieldName:  field,
            FieldType:  typ,
        })
    }
}
```
