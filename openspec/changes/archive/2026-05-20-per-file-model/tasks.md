## 1. Model updates

- [x] 1.1 Add `backend: String?` field to `QueueItem`
- [x] 1.2 Add `model: String?` field to `QueueItem`
- [x] 1.3 Update `QueueItem` Equatable conformance if needed

## 2. QueueManager defaults

- [x] 2.1 Update `addSource()` to set `backend` and `model` from `SettingsManager` defaults
- [x] 2.2 Update `processItem()` to use per-file backend/model with fallback to settings

## 3. UI dropdown

- [x] 3.1 Create `ModelSelectorDropdown` view component
- [x] 3.2 Add dropdown to `QueueItemRow` next to file name
- [x] 3.3 Populate dropdown from `ModelManager` based on selected backend
- [x] 3.4 Handle backend switch (refresh model list, select first model)
- [x] 3.5 Style dropdown to fit compact row layout

## 4. CLI integration

- [x] 4.1 Update CLI args construction in `processItem()` to use per-file backend/model
- [x] 4.2 Test CLI with different backend/model combinations per file
