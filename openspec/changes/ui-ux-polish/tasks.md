## 1. Toast notifications

- [ ] 1.1 Create `ToastManager` observable object with queue management
- [ ] 1.2 Create `ToastView` component with type-based styling (success, error, info)
- [ ] 1.3 Add toast overlay to ContentView with auto-dismiss timer
- [ ] 1.4 Trigger toasts from QueueManager on completion/error events

## 2. Context menus

- [ ] 2.1 Add `.contextMenu` to `QueueItemRow` with state-based actions
- [ ] 2.2 Implement "Process this" action (single file processing)
- [ ] 2.3 Implement "Open workspace" action (Finder open workspace dir)
- [ ] 2.4 Implement "Copy path" action (NSPasteboard)
- [ ] 2.5 Implement "Retry" action for failed items

## 3. Visual hierarchy

- [ ] 3.1 Add type-based colors to `SourceType` (PDF=red, video=purple, audio=green, etc.)
- [ ] 3.2 Add backend badge colors (Ollama=orange, OpenRouter=blue)
- [ ] 3.3 Add per-file model override indicator (italic/asterisk)
- [ ] 3.4 Update `QueueItemRow` to use type colors for icons

## 4. Drag-and-drop reordering

- [ ] 4.1 Add `.draggable` modifier to `QueueItemRow`
- [ ] 4.2 Add `.dropDestination` to sidebar List
- [ ] 4.3 Implement `reorder(from:to:)` in `QueueManager`
- [ ] 4.4 Add drag visual feedback (highlight drop targets)

## 5. Keyboard shortcuts

- [ ] 5.1 Add ⌘↵ shortcut for "Process selected file"
- [ ] 5.2 Add ⌫ shortcut for "Delete selected item"
- [ ] 5.3 Verify existing shortcuts still work (⌘R, ⌘., ⇧⌘K, ⌥⌘K, ⌘O, ⌘,)

## 6. Animations and polish

- [ ] 6.1 Add `.animation(.easeInOut)` for state transitions in QueueItemRow
- [ ] 6.2 Add smooth progress indicator animation
- [ ] 6.3 Improve empty state with helpful hints and quick actions
- [ ] 6.4 Add loading skeleton for DetailView while content loads
