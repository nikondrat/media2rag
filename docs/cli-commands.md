# CLI Commands — media2rag

## `media2rag process [file|url|directory]`

Обработать контент в RAG-ready Markdown.

### Флаги

| Флаг | Тип | По умолч. | Описание |
|------|-----|-----------|----------|
| `--json` | `bool` | `false` | Вывод JSON-событий (для агентов) |
| `--backend` | `string` | `""` | LLM backend: `ollama`, `openrouter`, `lmstudio` |
| `--model` | `string` | `""` | LLM модель (переопределяет config) |
| `--extract-only` | `bool` | `false` | Только экстракция, без pipeline |
| `-o, --output` | `string` | `""` | Путь для выходного файла |
| `-d, --output-dir` | `string` | `""` | Директория для intermediate файлов |
| `--final-dir` | `string` | `""` | Плоская директория с .md по названиям |
| `--log-file` | `string` | `""` | Файл лога (авто в output dir) |
| `--force` | `bool` | `false` | Переобработать, даже если есть output |
| `--file-concurrency` | `int` | `0` | Параллельных файлов (0=auto per backend) |
| `--total-concurrency` | `int` | `0` | Всего параллельных LLM запросов (0=auto) |

### Auto по backend (concurrency)

| Backend | file_concurrency | total_concurrency |
|---------|-----------------|-------------------|
| openrouter | 3 | 100 |
| lmstudio | 4 | 16 |
| ollama (default) | 1 | 8 |

### Примеры

```bash
# Один файл
media2rag process ./notes.md

# URL (авто через npx rdrr)
media2rag process https://example.com/article
media2rag process https://youtube.com/watch?v=...

# Директория (batch)
media2rag process ./docs/

# С переопределением бекенда
media2rag process ./notes.md --backend openrouter --model claude-sonnet-4-20250514

# Только экстракция (посмотреть что получится)
media2rag process ./notes.md --extract-only

# С output dir для intermediate файлов
media2rag process ./notes.md -d ./output/my-notes

# Переобработать
media2rag process ./notes.md --force

# JSON output для AI агента
media2rag process https://example.com --json

# С параллельной обработкой
media2rag process ./directory/ --file-concurrency 3 --total-concurrency 20
```

### Что поддерживает source

| Source | Формат | Как |
|--------|--------|-----|
| URL (http/https) | Любой | `npx rdrr` — веб, YouTube, Telegram, GitHub |
| `.md` / `.markdown` | Markdown | Чтение файла, стрип frontmatter |
| Директория | `.md`/`.markdown` | Batch по всем файлам (не рекурсивно) |

### Output структура (с `-d`)

```
<output-dir>/
├── <title>.md                    # Копия final.md с именем по тайтлу
├── output/
│   └── final.md                  # Финальный RAG-ready Markdown
├── chunks/
│   ├── chunk_001.md
│   └── chunk_002.md
├── intermediate/
│   ├── raw.md                    # Сырой extracted контент
│   ├── cleaned.md                # После preClean
│   └── holistic.md              # core_thesis + domains
├── results/
│   ├── result_001.json           # Per-chunk результат
│   └── result_002.json
├── process.log                   # Лог обработки
├── status.yaml                   # Статус для resume
├── .media2rag.yaml               # Метаданные
└── telemetry.jsonl               # LLM вызовы (токены, cost, latency)
```

### Pipeline stages

```
source → extract (rdrr/file) → preClean (LLM) → splitText (paragraphs)
  → processChunks (LLM, parallel) → holisticAnalysis (LLM)
  → causalExtraction (LLM) → contextualEnrich (LLM, parallel)
  → assemble → output
```

---

## `media2rag health`

Проверка доступности LLM backend и модели.

```bash
media2rag health
```

| Backend | Проверка |
|---------|----------|
| `lmstudio` | GET `/v1/models` — сервер отвечает, модель загружена |
| `ollama` | GET `/api/tags` — модель установлена |
| `openrouter` | `OPENROUTER_API` ключ сконфигурирован |

---

## `media2rag documents`

Управление обработанными документами в workspace.

### `documents list`

```bash
media2rag documents list
# Hash      Title                           Source               Versions
# --------  ------------------------------  --------------------  --------
# a1b2c3d4  Как масштабировать бизнес      business-scale.md     3
```

### `documents show <hash>`

```bash
# Метаданные
media2rag documents show a1b2c3d4

# Контент конкретной версии
media2rag documents show a1b2c3d4 --version 2

# Список всех версий
media2rag documents show a1b2c3d4 --versions
```

| Флаг | Описание |
|------|----------|
| `--version` | Показать контент версии N |
| `--versions` | Список всех версий |

### `documents delete <hash>`

```bash
# С подтверждением
media2rag documents delete a1b2c3d4

# Без подтверждения
media2rag documents delete a1b2c3d4 --force
```

---

## `media2rag chat` (stub)

Заглушка. Команда не реализована — AI агенты вызывают CLI напрямую.

---

## Global Flags

| Флаг | Тип | Описание |
|------|-----|----------|
| `--config` | `string` | Путь к config файлу (default: `~/.media2rag/config.yaml`) |
| `-v, --verbose` | `bool` | Подробный вывод |

---

## Config (`~/.media2rag/config.yaml`)

```yaml
llm:
  default_backend: lmstudio       # ollama | openrouter | lmstudio
  ollama_url: http://localhost:11434
  lmstudio_url: http://localhost:1234
  openrouter_url: https://openrouter.ai/api/v1
  openrouter_key: ""              # или OPENROUTER_API env
  model: ""                       # модель по умолчанию
  timeout: 600                    # секунд (10 мин)

pipeline:
  max_tokens: 4096
  chunk_size: 1500
  max_concurrency: 3             # параллельных LLM запросов на файл
  max_file_concurrency: 0        # параллельных файлов (0 = auto)
  max_total_concurrency: 0       # всего LLM запросов (0 = auto)
  holistic_analysis: true        # core_thesis + domains + causal

workspace:
  data_dir: ""                    # default: ~/.media2rag/workspace/
```

### Environment Variables

| Variable | Overrides |
|----------|-----------|
| `OPENROUTER_API` | `llm.openrouter_key` |
| `MEDIA2RAG_LLM_DEFAULT_BACKEND` | `llm.default_backend` |
| `MEDIA2RAG_LLM_OLLAMA_URL` | `llm.ollama_url` |
| `MEDIA2RAG_LLM_OPENROUTER_URL` | `llm.openrouter_url` |
| `MEDIA2RAG_LLM_OPENROUTER_KEY` | `llm.openrouter_key` |
| `MEDIA2RAG_LLM_MODEL` | `llm.model` |
| `MEDIA2RAG_LLM_TIMEOUT` | `llm.timeout` |

---

## Build & Run

```bash
# Сборка
go build -o media2rag ./cmd/media2rag

# Запуск
./media2rag process ./notes.md

# Makefile
make build     # go build
make run       # build + run
make test      # go test ./...
make clean     # rm -f media2rag

# Установка в GOPATH
go install ./cmd/media2rag
```
