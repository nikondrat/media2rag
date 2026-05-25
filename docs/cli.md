# CLI Commands — v1 Design

## Команды

### process — обработка файла/URL

```bash
# Обработать URL (YouTube, веб-страница, Telegram)
media2rag process "https://youtube.com/watch?v=..."

# Обработать локальный Markdown
media2rag process ./notes.md

# С флагами
media2rag process ./notes.md \
  --backend ollama \
  --model qwen3.5:27b \
  --workspace ~/.media2rag/workspace \
  --json

# Только экстракция (без LLM)
media2rag process ./notes.md --extract-only

# С проверкой качества
media2rag process ./notes.md --quality-check

# С reasoning
media2rag process ./notes.md --reasoning
```

**Флаги:**
| Флаг | Описание |
|------|----------|
| `--backend` | Бэкенд: `ollama` или `openai` |
| `--model` | Модель для LLM |
| `--workspace` | Директория workspace |
| `--json` | Выводить JSON события в stdout |
| `--extract-only` | Только экстракция, без pipeline |
| `--quality-check` | Включить проверку качества |
| `--reasoning` | Включить reasoning mode |

### serve — HTTP сервер

```bash
# Запуск с дефолтами
media2rag serve

# С настройками
media2rag serve --port 8542 --host 0.0.0.0
```

**Флаги:**
| Флаг | Описание |
|------|----------|
| `--port` | Порт (дефолт: 8542) |
| `--host` | Хост (дефолт: localhost) |
| `--config` | Путь к config.yaml |

### ask — быстрый RAG-запрос

```bash
# Один вопрос → один ответ
media2rag ask "расскажи про HNSW"

# С флагами
media2rag ask "расскажи про HNSW" \
  --top-k 5 \
  --rewrite \
  --rerank \
  --json
```

**Флаги:**
| Флаг | Описание |
|------|----------|
| `--top-k` | Количество источников (дефолт: 5) |
| `--rewrite` | Включить query rewrite |
| `--rerank` | Включить reranking |
| `--json` | JSON вывод |

### chat — интерактивный чат

```bash
# Новая сессия
media2rag chat

# Продолжить сессию
media2rag chat --session abc123

# С флагами
media2rag chat --session abc123 --backend ollama --model qwen3.5:27b
```

**Флаги:**
| Флаг | Описание |
|------|----------|
| `--session` | ID сессии для продолжения |
| `--backend` | Бэкенд |
| `--model` | Модель |

### documents — управление документами

```bash
# Список документов
media2rag documents list

# Удалить документ
media2rag documents delete doc123

# Показать детали
media2rag documents show doc123
```

### memory — управление памятью

```bash
# Список фактов
media2rag memory list

# Поиск фактов
media2rag memory search "HNSW"

# Добавить факт
media2rag memory add "Пользователь интересуется HNSW"

# Удалить факт
media2rag memory delete fact123
```

### status — проверка системы

```bash
media2rag status

→ Qdrant:    connected (localhost:6334)
  Ollama:    connected (localhost:11434)
  rdrr:      available
  Embedding: qwen3-embedding:0.6b (1024d)
  Documents: 15 indexed
  Memory:    42 facts
```

## Глобальные флаги

| Флаг | Описание |
|------|----------|
| `--config` | Путь к config.yaml |
| `--verbose` | Подробный вывод |
| `--quiet` | Только ошибки |
| `--json` | JSON вывод для всех команд |

## Структура (cobra)

```go
var rootCmd = &cobra.Command{
    Use:   "media2rag",
    Short: "Convert any content into RAG-ready Markdown",
}

var processCmd = &cobra.Command{
    Use:   "process <source>",
    Short: "Process a file or URL",
    RunE:  runProcess,
}

var serveCmd = &cobra.Command{
    Use:   "serve",
    Short: "Start HTTP server",
    RunE:  runServe,
}

var askCmd = &cobra.Command{
    Use:   "ask <question>",
    Short: "Quick RAG query",
    RunE:  runAsk,
}

var chatCmd = &cobra.Command{
    Use:   "chat",
    Short: "Interactive chat",
    RunE:  runChat,
}

var documentsCmd = &cobra.Command{
    Use:   "documents",
    Short: "Manage documents",
}

var memoryCmd = &cobra.Command{
    Use:   "memory",
    Short: "Manage memory",
}

var statusCmd = &cobra.Command{
    Use:   "status",
    Short: "Check system status",
    RunE:  runStatus,
}

func init() {
    rootCmd.AddCommand(processCmd, serveCmd, askCmd, chatCmd)
    rootCmd.AddCommand(documentsCmd, memoryCmd, statusCmd)

    // Глобальные флаги
    rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "config file")
    rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "verbose output")
    rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "JSON output")
}
```
