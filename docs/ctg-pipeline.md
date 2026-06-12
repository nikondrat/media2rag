# CTG Pipeline — v1 Design

## Philosophy

Каждый этап pipeline делает **одну маленькую задачу** и делает её хорошо. Никаких "универсальных" промптов, которые делают всё сразу — они халтурят.

Pipeline строится как цепочка Stage'ов. Каждый Stage — явная функция:

```go
type Stage func(ctx context.Context, input string, emitter EventEmitter) (string, error)
```

---

## Этапы v1

```
Raw text
  │
  ▼
[Compress] — LLM: clean artifacts
  │
  ▼
[Split] — Recursive character split
  │
  ├── Chunk 1 ──► [Process] — LLM: title + topics + summary
  ├── Chunk 2 ──► [Process] — LLM: title + topics + summary
  │  ... (параллельно, worker pool)
  └── Chunk N ──► [Process] — LLM: title + topics + summary
  │
  ▼
[Holistic] — LLM: core_thesis + domains (1 call, все summaries)
  │
  ▼
[Causal] — LLM: causal_chains + preconditions + counterfactuals (1 call)
  │
  ▼
[Context Enrich] — LLM: per-chunk context (параллельно)
  │
  ▼
[Assemble] — Merge chunks → final MD с causal секциями
```

### 1. Compress

**Задача:** убрать мусор, который не является контентом.

**Что убираем:**
- Таймстемпы (транскрипты YouTube)
- Рекламные вставки ("подпишись на канал", "это спонсорский выпуск")
- Повторяющиеся строки
- Артефакты OCR
- Пустые строки/секции

**Промпт (один маленький):**
```
Clean this text by removing:
- Timestamps and timecodes
- Advertisement/Sponsor messages
- Repeated or redundant lines
- OCR artifacts
Keep all meaningful content. Preserve paragraph structure.
Return only the cleaned text.

Text:
{input}
```

**Разбивка на части:** если текст > контекстного окна LLM (>32K токенов), рекурсивно делим по параграфам и чистим каждую часть отдельно.

### 2. Split

**Задача:** разбить очищенный текст на семантически-связные чанки для обработки LLM.

**Алгоритм:** Recursive character split (без LLM, без эмбеддингов).

```go
type Splitter struct {
    ChunkSize    int  // default: 4000 символов
    ChunkOverlap int  // default: 200 символов
}

func (s *Splitter) Split(text string) []string
```

**Порядок разделителей** (пробуем по очереди):
1. `\n\n\n` — multiple blank lines (strong section break)
2. `\n\n` — paragraph break
3. `\n` — line break
4. `. ` — sentence boundary
5. Hard cut at ChunkSize (с overlap)

**Правила:**
- Если кусок меньше ChunkSize — не режем
- overlap только на границах чанков (чтобы не потерять контекст)
- Размер чанка настраивается под контекстное окно модели

### 3. Process (per chunk)

**Задача:** один маленький промпт на чанк, который делает три простых вещи.

**Промпт:**
```
Analyze this text chunk and return:

title: <short descriptive title, max 8 words>
topics: <2-3 key topics, comma separated>
summary: <1-2 sentence summary>

Text:
{chunk}
```

**Парсинг ответа:** регулярка или простой split по `: `.

Каждый чанк обрабатывается параллельно через worker pool (например, 3 concurrent LLM вызова).

**Почему такой маленький промпт:**
- LLM не успевает "халтурить" — задача конкретная
- Ответ предсказуемый (KV формат)
- Легко парсить
- Быстро: один чанк = 1 LLM вызов, а не 3

### 4. Assemble

**Задача:** собрать все чанки в финальный RAG-ready документ.

```go
type Assembler struct {
    // Объединение тем со всех чанков (дедупликация + частотность)
    MergeTopics(chunks []ChunkResult) []string
    // Выбор лучшего заголовка (первый непустой или частота)
    PickTitle(chunks []ChunkResult) string
    // Объединение саммари в одно
    BuildSummary(chunks []ChunkResult) string
    // Генерация Markdown с frontmatter
    Generate(chunks []ChunkResult, meta DocumentMetadata) RAGDocument
}
```

**Выход:**
```markdown
---
title: "Как работает X"
source: video_123.mp4
type: transcript
topics:
  - topic1
  - topic2
  - topic3
summary: |
  Основная идея документа...
word_count: 12400
---

# Clean text content...

[Chunk summaries могли бы быть вставлены как комментарии или разделы]
```

---

## JSON Events

Pipeline эмитит события для клиента (GUI):

| Event | When | Data |
|-------|------|------|
| `compression_start` | Начало чистки | chars: int |
| `cleaning_part` | Чистка части текста | current, total |
| `compression_done` | Чистка завершена | chars: int |
| `splitting` | Начало разбивки | chars: int |
| `split_done` | Разбивка завершена | chunks: int |
| `processing_start` | Начало LLM обработки | chunks: int |
| `processing_chunk` | Обработка чанка | current, total |
| `processing_chunk_done` | Чанк обработан | current, total |
| `processing_done` | Все чанки обработаны | — |
| `assembling` | Сборка документа | — |
| `completed` | Pipeline завершён | output: path |
| `error` | Ошибка | message: string |

---

## План расширения (v2+)

Каждый новый этап — отдельный маленький промпт, добавляемый в Process stage:

| v | Prompt | Output |
|---|--------|--------|
| v1 | title + topics + summary | Базовый MD |
| v2 | extract claims | claims: []Claim |
| v3 | extract mental models | mental_models: []string |
| v4 | key terms | key_terms: []KeyTerm |
| v5 | core thesis | core_thesis: string |
| v6 | takeaways | takeaways: []string |
| v7 | **causal extraction** | causal_chains, preconditions, counterfactuals |

Каждый новый промпт добавляется как ещё один параллельный LLM вызов к тому же чанку. Ничего не ломается — только обогащается.

### Causal Extraction (v7)

**Stage 2: Relational Extraction** — один LLM вызов на весь документ (после Holistic):

```
Prompt: Analyze chunk summaries → extract causal relationships
Output:
  causal_chains: [cause → mechanism → effect] with relation type
  preconditions: conditions that enable processes
  counterfactuals: what changes if a factor is removed
```

**Почему document-level, не per-chunk:**
- Causality часто跨-чанковая (причина в chunk 3, следствие в chunk 7)
- Один вызов = дешевле, чем N вызовов
- LLM видит полную картину, а не изолированные фрагменты

**Edge types:**
| Edge | Описание |
|------|----------|
| `causes` | A непосредственно вызывает B |
| `enables` | A делает B возможным |
| `prevents` | A блокирует B |
| `requires` | B невозможно без A |
| `correlates` | A и B связаны, но causality неясна |

---

## Конфигурация

```go
type PipelineConfig struct {
    Compressor struct {
        ChunkSize      int // контекстное окно для очистки (дефолт: 32000 токенов)
    }
    Splitter struct {
        ChunkSize      int // дефолт: 4000 символов
        ChunkOverlap   int // дефолт: 200
        MinChunkSize   int // минимальный размер чанка (дефолт: 500)
    }
    Processing struct {
        MaxConcurrency int // параллельных LLM вызовов (дефолт: 3)
        Model          string // модель для pipeline (дефолт: из config)
    }
}
```
