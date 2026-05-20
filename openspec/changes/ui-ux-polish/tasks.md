## 1. Toast notifications

- [x] 1.1 Create `ToastManager` observable object with queue management
- [x] 1.2 Create `ToastView` component with type-based styling (success, error, info)
- [x] 1.3 Add toast overlay to ContentView with auto-dismiss timer
- [x] 1.4 Trigger toasts from QueueManager on completion/error events

## 2. Context menus

- [x] 2.1 Add `.contextMenu` to `QueueItemRow` with state-based actions
- [x] 2.2 Implement "Process this" action (single file processing)
- [x] 2.3 Implement "Open workspace" action (Finder open workspace dir)
- [x] 2.4 Implement "Copy path" action (NSPasteboard)
- [x] 2.5 Implement "Retry" action for failed items

## 3. Visual hierarchy

- [x] 3.1 Add type-based colors to `SourceType` (PDF=red, video=purple, audio=green, etc.)
- [x] 3.2 Add backend badge colors (Ollama=orange, OpenRouter=blue)
- [x] 3.3 Add per-file model override indicator (italic/asterisk)
- [x] 3.4 Update `QueueItemRow` to use type colors for icons

## 4. Drag-and-drop reordering

- [x] 4.1 Add `.draggable` modifier to `QueueItemRow`
- [x] 4.2 Add `.dropDestination` to sidebar List
- [x] 4.3 Implement `reorder(from:to:)` in `QueueManager`
- [x] 4.4 Add drag visual feedback (highlight drop targets)

## 5. Keyboard shortcuts

- [x] 5.1 Add ⌘↵ shortcut for "Process selected file"
- [x] 5.2 Add ⌫ shortcut for "Delete selected item"
- [x] 5.3 Verify existing shortcuts still work (⌘R, ⌘., ⇧⌘K, ⌥⌘K, ⌘O, ⌘,)

## 6. Animations and polish

- [x] 6.1 Add `.animation(.easeInOut)` for state transitions in QueueItemRow
- [x] 6.2 Add smooth progress indicator animation
- [x] 6.3 Improve empty state with helpful hints and quick actions
- [x] 6.4 Add loading skeleton for DetailView while content loads
