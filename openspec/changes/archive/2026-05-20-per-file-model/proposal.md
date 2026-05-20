## Why

Сейчас модель и бэкенд задаются глобально в настройках и применяются ко всем файлам в очереди. Невозможно обработать один файл через лёгкую модель, а другой — через мощную. Пользователь вынужден менять настройки между файлами или запускать обработку по одному.

## What Changes

- `QueueItem` получает поля `backend: String?` и `model: String?`
- При добавлении файла — автоматическая подстановка дефолтов из `SettingsManager`
- В `QueueItemRow` добавлен dropdown для выбора модели/бэкенда
- CLI вызов передаёт `--backend` и `--model` для каждого файла индивидуально
- Глобальные настройки остаются как дефолты

## Capabilities

### New Capabilities
- `per-file-model`: Выбор LLM бэкенда и модели для каждого файла в очереди
- `model-dropdown`: Dropdown в QueueItemRow для быстрого выбора модели

### Modified Capabilities
- `queue-processing`: CLI args теперь включают per-file backend и model вместо глобальных

## Impact

- `media2rag-gui/media2rag/Models/QueueItem.swift` — новые поля backend, model
- `media2rag-gui/media2rag/Views/ContentView.swift` — dropdown в QueueItemRow
- `media2rag-gui/media2rag/Services/QueueManager.swift` — передача per-file args в CLI
- `media2rag-gui/media2rag/Services/ModelManager.swift` — переиспользуется для получения списка моделей
