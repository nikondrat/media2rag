## Context

Приложение имеет базовый UI: sidebar со списком, detail view с контентом, settings. Но отсутствуют: анимации состояний, toast-уведомления, контекстные меню, drag-and-drop, расширенные keyboard shortcuts. Визуальная иерархия плоская — все элементы выглядят одинаково.

## Goals / Non-Goals

**Goals:**
- Toast-уведомления для завершения/ошибок
- Context menu на QueueItemRow
- Цветовая кодировка типов файлов и бэкендов
- Drag-and-drop реорганизация очереди
- Keyboard shortcuts для всех частых действий
- Smooth animations для state transitions

**Non-Goals:**
- Изменение основной архитектуры (NavigationSplitView остаётся)
- Тёмная/светлая тема (системная работает)
- Изменение CLI или Python кода

## Decisions

### 1. Toast notifications

Используем SwiftUI overlay с `Animation` для toast:
```swift
.toast(isPresented: $showToast) {
    ToastView(message: toastMessage, type: toastType)
}
```

ToastManager — `@MainActor ObservableObject` с очередью уведомлений.

### 2. Context menu

`.contextMenu { }` на `QueueItemRow`:
- Process this file
- Remove from queue
- Open in Finder
- Copy path
- Open workspace folder
- Retry (if failed)

### 3. Visual hierarchy

| Element | Color |
|---------|-------|
| PDF | Red accent |
| EPUB | Blue accent |
| Video | Purple accent |
| Audio | Green accent |
| Ollama backend | Orange badge |
| OpenRouter backend | Blue badge |
| Custom model (per-file) | Italic label |

### 4. Drag-and-drop

`.onDrop` и `.drag` на `QueueItemRow`. `QueueManager.reorder(from:to:)` меняет порядок в массиве.

### 5. Keyboard shortcuts

| Action | Shortcut |
|--------|----------|
| Process all | ⌘R |
| Stop | ⌘. |
| Clear completed | ⇧⌘K |
| Clear all | ⌥⌘K |
| Add file | ⌘O |
| Settings | ⌘, |
| Process selected | ⌘↵ |
| Delete selected | ⌫ |

## Risks / Trade-offs

| Risk | Mitigation |
|------|-----------|
| Много анимаций = тормоза | Использовать `.animation(.easeInOut, value:)` только для нужных свойств |
| Drag-and-drop конфликтует с selection | `.drag` только на иконке, не на всей row |
| Toast перекрывает контент | Auto-dismiss через 3 секунды, max 3 toast в очереди |
