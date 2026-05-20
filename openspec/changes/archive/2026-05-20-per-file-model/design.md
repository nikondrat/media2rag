## Context

`SettingsManager` хранит глобальные `backend` и `model`. `QueueManager.startProcessing()` использует эти значения для всех файлов. Нет возможности переопределить модель на уровне файла.

## Goals / Non-Goals

**Goals:**
- Per-file override для backend и model
- Дефолты из SettingsManager при добавлении файла
- UI для выбора в QueueItemRow (dropdown)
- CLI получает `--backend` и `--model` per file

**Non-Goals:**
- Изменение глобальных настроек
- Валидация модели (CLI сам вернёт ошибку)
- Кэширование результатов при смене модели

## Decisions

### 1. Optional fields with defaults

```swift
struct QueueItem {
    var backend: String?   // nil = use settings default
    var model: String?     // nil = use settings default
}
```

При добавлении файла: `item.backend = settings.backend`, `item.model = settings.model`. Пользователь может изменить.

### 2. Dropdown in QueueItemRow

Маленький dropdown справа от имени файла:
```
📄 my-book.pdf          [gemma4:26b ▼]
```

При клике — меню с доступными моделями из `ModelManager`. Для Ollama — список локальных моделей, для OpenRouter — список из API.

### 3. CLI args construction

```swift
let backend = item.backend ?? settings.backend
let model = item.model ?? settings.model
var args = [source, "--backend", backend, "--model", model, ...]
```

### 4. Model list sharing

`ModelManager` уже загружает списки моделей. Переиспользуем для dropdown. При смене бэкенда в dropdown — обновляем список моделей.

## Risks / Trade-offs

| Risk | Mitigation |
|------|-----------|
| Dropdown в маленьком row — тесно | Компактный menu picker, только иконка + название |
| Модель не доступна на выбранном бэкенде | CLI вернёт ошибку, покажем в errorMessage |
| Много файлов с разными моделями — путаница | Цветовая индикация бэкенда в row |
