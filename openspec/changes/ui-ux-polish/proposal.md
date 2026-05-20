## Why

Приложение функционально работает, но UX можно значительно улучшить: анимации, визуальная иерархия, удобство навигации, feedback при действиях. Цель — чтобы пользоваться приложением было кайфово, не хотелось выходить.

## What Changes

- Улучшенные анимации переходов между состояниями (processing → completed)
- Визуальная иерархия: цветовая кодировка типов файлов, бэкендов, статусов
- Toast-уведомления вместо status bar сообщений
- Keyboard shortcuts для частых действий
- Контекстное меню (right-click) на QueueItemRow
- Улучшенный empty state с подсказками
- Smooth progress indicators вместо линейных
- Drag-and-drop реорганизация очереди
- Быстрые действия: "Обработать только этот", "Открыть workspace"

## Capabilities

### New Capabilities
- `toast-notifications`: Toast-уведомления для событий завершения/ошибок
- `context-menu`: Контекстное меню на элементах очереди
- `visual-hierarchy`: Цветовая кодировка типов, бэкендов, статусов
- `queue-reordering`: Drag-and-drop для изменения порядка обработки
- `quick-actions`: Быстрые действия (process single, open workspace, copy path)

### Modified Capabilities
- `animations`: Улучшенные анимации переходов состояний
- `keyboard-shortcuts`: Расширенные keyboard shortcuts

## Impact

- `media2rag-gui/media2rag/Views/ContentView.swift` — context menu, drag-drop, animations
- `media2rag-gui/media2rag/Views/DetailView.swift` — animations, visual hierarchy
- `media2rag-gui/media2rag/Views/SettingsView.swift` — visual polish
- `media2rag-gui/media2rag/media2ragApp.swift` — keyboard shortcuts
- Новые файлы: `ToastManager.swift`, `ContextMenuActions.swift`
