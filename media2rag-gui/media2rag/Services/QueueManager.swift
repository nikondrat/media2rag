import Foundation
import AppKit

@MainActor
class QueueManager: ObservableObject {
    @Published var items: [QueueItem] = []
    @Published var activeItemId: UUID?
    @Published var isProcessing = false
    @Published var currentIndex = 0
    @Published var totalProcessed = 0
    @Published var totalErrors = 0
    @Published var selectedItemId: UUID?

    private var cliRunner = CLIRunner()
    private var settingsManager: SettingsManager?
    var toastManager: ToastManager?

    var totalCount: Int { items.count }
    var queuedCount: Int { items.filter { $0.state == .queued }.count }
    var completedCount: Int { items.filter { $0.state == .completed }.count }
    var failedCount: Int { items.filter { $0.state == .failed }.count }

    var activeItem: QueueItem? {
        guard let id = activeItemId else { return nil }
        return items.first { $0.id == id }
    }

    func setSettingsManager(_ manager: SettingsManager) {
        self.settingsManager = manager
    }

    func addSource(_ source: String) {
        var cleanSource = source.replacingOccurrences(of: "file://", with: "")
        if let decoded = cleanSource.removingPercentEncoding {
            cleanSource = decoded
        }
        let item = QueueItem(
            source: cleanSource,
            sourceType: SourceType.from(source: cleanSource)
        )
        items.append(item)
    }

    func addSources(_ sources: [String]) {
        for source in sources {
            addSource(source)
        }
    }

    func removeItem(_ item: QueueItem) {
        items.removeAll { $0.id == item.id }
    }

    func clearCompleted() {
        items.removeAll { $0.state == .completed || $0.state == .failed }
    }

    func clearAll() {
        items.removeAll()
    }

    func startProcessing() async {
        guard !isProcessing, !items.isEmpty else { return }
        guard let settings = settingsManager else { return }

        isProcessing = true
        currentIndex = 0
        totalProcessed = 0
        totalErrors = 0

        for index in items.indices {
            guard items[index].state == .queued else { continue }

            currentIndex = index
            activeItemId = items[index].id
            await processItem(index: index, settings: settings)
        }

        activeItemId = nil
        isProcessing = false
    }

    func stopProcessing() {
        cliRunner.stop()
        activeItemId = nil
        isProcessing = false
    }

    private func processItem(index: Int, settings: SettingsManager) async {
        guard index < items.count else { return }

        items[index].state = .extracting
        items[index].statusMessage = "Загрузка файла..."
        items[index].startedAt = Date()
        items[index].progress = 0

        let cliPath = settings.resolvedCLIPath
        if cliPath.isEmpty {
            guard index < items.count else { return }
            items[index].state = .failed
            items[index].statusMessage = "Путь к CLI не указан"
            items[index].errorMessage = "Путь к CLI не указан"
            items[index].completedAt = Date()
            totalErrors += 1
            return
        }

        let effectiveBackend = items[index].backend ?? settings.backend
        let effectiveModel = items[index].model ?? settings.model

        let sourceForCLI: String
        if let originalSource = items[index].originalSource, !originalSource.isEmpty {
            sourceForCLI = originalSource
        } else {
            sourceForCLI = items[index].source
        }

        var args = [
            sourceForCLI,
            "--workspace", settings.workspaceDirectory,
            "--backend", effectiveBackend,
            "--model", effectiveModel,
            "--json"
        ]

        if let workspaceURL = items[index].workspaceURL {
            args.append("--work-dir")
            args.append(workspaceURL.path)
        }

        if settings.extractOnly {
            args.append("--extract-only")
        }

        let events = cliRunner.run(
            arguments: args,
            cliPath: cliPath
        )

        var isTelegramChannel = false

        for await event in events {
            guard index < items.count else { break }

            switch event.eventType {
            case "telegram_channel":
                isTelegramChannel = true
                items[index].statusMessage = "Скрапинг канала: \(event.totalPosts ?? 0) постов найдено"
                items[index].progress = 0.05
            case "telegram_progress":
                if let current = event.current, let total = event.total {
                    let pct = Double(current) / Double(total) * 0.85
                    items[index].progress = 0.05 + pct
                    items[index].statusMessage = "Пост \(current) из \(total)..."
                }
            case "telegram_complete":
                items[index].state = .completed
                items[index].statusMessage = "Готово: \(event.processed ?? 0) из \(event.total ?? 0) постов"
                items[index].progress = 1.0
                items[index].completedAt = Date()
                items[index].outputFiles = event.outputFiles
                if let files = event.outputFiles, let first = files.first {
                    items[index].outputURL = URL(fileURLWithPath: first)
                }
                if let words = event.words {
                    items[index].wordCount = words
                }
                totalProcessed += 1
                toastManager?.show(message: "✅ \(items[index].fileName) — \(event.processed ?? 0) постов", type: .success)
            case "large_doc_detected":
                items[index].state = .mapReduce
                items[index].statusMessage = "Большой документ: обработка по частям..."
                items[index].progress = 0.2
            case "map_start":
                items[index].state = .mapReduce
                if let total = event.total {
                    items[index].statusMessage = "Разбито на \(total) чанков"
                }
                items[index].progress = 0.2
            case "map_skip":
                if let current = event.current, let total = event.total {
                    items[index].statusMessage = "Чанк \(current)/\(total) (пропуск, уже готов)"
                }
            case "map_chunk":
                if let chunkId = event.current {
                    ensureChunksArray(for: index, upTo: chunkId)
                    items[index].chunks[chunkId - 1].status = .processing
                }
            case "map_chunk_done":
                if let chunkId = event.current {
                    ensureChunksArray(for: index, upTo: chunkId)
                    items[index].chunks[chunkId - 1].status = .done
                }
            case "map_chunk_error":
                if let chunkId = event.current {
                    ensureChunksArray(for: index, upTo: chunkId)
                    items[index].chunks[chunkId - 1].status = .error
                    items[index].chunks[chunkId - 1].errorMessage = event.message
                }
            case "map_done":
                items[index].statusMessage = "Все чанки обработаны, объединение..."
                items[index].progress = 0.6
            case "reduce_start":
                items[index].statusMessage = "Объединение секций..."
                items[index].progress = 0.65
            case "reduce_skip":
                if let section = event.section {
                    items[index].statusMessage = "Секция «\(section)» (пропуск, уже готова)"
                }
            case "merge_subsection":
                if let section = event.section, let part = event.part, let total = event.total {
                    items[index].statusMessage = "Секция «\(section)»: часть \(part) из \(total)"
                }
            case "merge_subsection_skip":
                if let section = event.section, let part = event.part {
                    items[index].statusMessage = "Секция «\(section)»: часть \(part) (пропуск)"
                }
            case "reduce_done":
                items[index].statusMessage = "Объединение завершено"
                items[index].progress = 0.85
            case "extracting":
                items[index].state = .extracting
                items[index].statusMessage = "Извлечение содержимого..."
                items[index].progress = 0.1
            case "extracted":
                items[index].wordCount = event.words
                items[index].statusMessage = "Содержимое извлечено (\(event.words ?? 0) слов)"
                items[index].progress = 0.2
            case "compression_start":
                items[index].state = .compressing
                items[index].statusMessage = "Очистка текста от шума..."
                items[index].progress = 0.3
            case "compressing_chunk":
                if let current = event.current, let total = event.total {
                    let chunkProgress = Double(current) / Double(total) * 0.3
                    items[index].progress = 0.3 + chunkProgress
                    items[index].statusMessage = "Очистка чанка \(current) из \(total)..."
                }
            case "compressed_chunk":
                if let current = event.current, let total = event.total {
                    items[index].statusMessage = "Чанк \(current) из \(total) готов ✓"
                }
            case "compression_done":
                items[index].statusMessage = "Текст очищен и сжат"
                items[index].progress = 0.6
            case "transformation_start":
                items[index].state = .transforming
                items[index].statusMessage = "Поиск тем и структуры..."
            case "transformation_done":
                items[index].topics = event.topics
                items[index].statusMessage = "Найдено тем: \(event.topics?.count ?? 0)"
                items[index].progress = 0.8
            case "generation_start":
                items[index].state = .generating
                items[index].statusMessage = "Генерация RAG markdown..."
            case "generation_done":
                items[index].statusMessage = "Документ сгенерирован"
                items[index].progress = 0.95
            case "completed":
                if !isTelegramChannel {
                    items[index].state = .completed
                    items[index].statusMessage = "Готово"
                    items[index].progress = 1.0
                    items[index].completedAt = Date()
                    if let workDir = event.workDir {
                        items[index].workspaceURL = URL(fileURLWithPath: workDir)
                        items[index].outputURL = URL(fileURLWithPath: workDir).appendingPathComponent("output/final.md")
                    } else if let output = event.output {
                        items[index].outputURL = URL(fileURLWithPath: output)
                    }
                    totalProcessed += 1
                    toastManager?.show(message: "✅ \(items[index].fileName) — готово", type: .success)
                }
            case "error":
                items[index].state = .failed
                items[index].statusMessage = "Ошибка"
                items[index].errorMessage = event.message ?? "Неизвестная ошибка"
                items[index].completedAt = Date()
                totalErrors += 1
                let msg = event.message ?? "Неизвестная ошибка"
                toastManager?.show(message: "❌ \(items[index].fileName) — \(msg)", type: .error)
            default:
                break
            }
        }

        guard index < items.count else { return }

        if items[index].state == .completed {
            loadMetadata(from: items[index].outputURL, index: index)
        }
    }

    private func ensureChunksArray(for index: Int, upTo chunkId: Int) {
        var chunks = items[index].chunks
        while chunks.count < chunkId {
            chunks.append(ChunkInfo(id: chunks.count + 1))
        }
        items[index].chunks = chunks
    }
    private func parseFrontmatter(_ content: String) -> (title: String?, topics: [String]?, summary: String?, keyInsights: [String]?)? {
        guard content.hasPrefix("---") else { return nil }

        let parts = content.split(separator: "---", maxSplits: 2)
        guard parts.count >= 2 else { return nil }

        let yamlContent = String(parts[1])
        var title: String?
        var topics: [String]?
        var summary: String?
        var keyInsights: [String]?

        for line in yamlContent.split(separator: "\n") {
            if line.hasPrefix("title:") {
                var raw = String(line.dropFirst(6).trimmingCharacters(in: .whitespaces))
                if raw.hasPrefix("\"") && raw.hasSuffix("\"") {
                    raw = String(raw.dropFirst().dropLast())
                } else if raw.hasPrefix("'") && raw.hasSuffix("'") {
                    raw = String(raw.dropFirst().dropLast())
                }
                title = raw
            } else if line.hasPrefix("topics:") {
                topics = []
            } else if line.hasPrefix("  - ") && topics != nil {
                topics?.append(String(line.dropFirst(4)))
            } else if line.hasPrefix("summary:") {
                summary = String(line.dropFirst(9).trimmingCharacters(in: .whitespaces))
            } else if line.hasPrefix("key_insights:") {
                keyInsights = []
            } else if line.hasPrefix("  - ") && keyInsights != nil {
                keyInsights?.append(String(line.dropFirst(4)))
            }
        }

        return (title: title, topics: topics, summary: summary, keyInsights: keyInsights)
    }

    private func loadMetadata(from url: URL?, index: Int) {
        guard let url = url else { return }
        do {
            let content = try String(contentsOf: url, encoding: .utf8)
            if let frontmatter = parseFrontmatter(content) {
                items[index].title = frontmatter.title
                items[index].topics = frontmatter.topics
                items[index].summary = frontmatter.summary
                items[index].keyInsights = frontmatter.keyInsights
            }
        } catch {
            // Ignore metadata parsing errors
        }
    }

    func processSingle(_ item: QueueItem) {
        guard let index = items.firstIndex(where: { $0.id == item.id }), !isProcessing else { return }
        guard let settings = settingsManager else { return }
        Task {
            await processItem(index: index, settings: settings)
        }
    }

    func openWorkspace(for item: QueueItem) {
        guard let url = item.outputURL?.deletingLastPathComponent() else {
            if let wsURL = item.workspaceURL {
                NSWorkspace.shared.open(wsURL)
            }
            return
        }
        NSWorkspace.shared.open(url)
    }

    func copyPath(for item: QueueItem) {
        var path = item.source
        if let output = item.outputURL?.path {
            path = output
        }
        NSPasteboard.general.clearContents()
        NSPasteboard.general.setString(path, forType: .string)
        toastManager?.show(message: "📋 Путь скопирован: \((path as NSString).lastPathComponent)", type: .info)
    }

    func retryItem(_ item: QueueItem) {
        guard let index = items.firstIndex(where: { $0.id == item.id }), !isProcessing else { return }
        guard let settings = settingsManager else { return }
        items[index].state = .queued
        items[index].progress = 0
        items[index].statusMessage = ""
        items[index].errorMessage = nil
        Task {
            await processItem(index: index, settings: settings)
        }
    }

    func reorder(from source: IndexSet, to destination: Int) {
        items.move(fromOffsets: source, toOffset: destination)
    }

    func loadExistingFiles() {
        guard let settings = settingsManager else { return }
        let workspaceURL = URL(fileURLWithPath: settings.workspaceDirectory)

        guard FileManager.default.fileExists(atPath: workspaceURL.path) else { return }

        do {
            let subdirs = try FileManager.default.contentsOfDirectory(atPath: workspaceURL.path)
                .filter { !$0.hasPrefix(".") }

            var loadedCount = 0

            for subdir in subdirs {
                let subdirURL = workspaceURL.appendingPathComponent(subdir)
                let yamlPath = subdirURL.appendingPathComponent(".media2rag.yaml")

                if FileManager.default.fileExists(atPath: yamlPath.path),
                   let yamlContent = try? String(contentsOf: yamlPath, encoding: .utf8),
                   let projectDict = parseSimpleYAML(yamlContent) {
                    loadFromYAML(subdirURL, subdir, projectDict, &loadedCount)
                } else {
                    loadFromDirectory(subdirURL, subdir, &loadedCount)
                }
            }

            if loadedCount > 0 {
                toastManager?.show(message: " Загружено \(loadedCount) проектов", type: .info)
            }
        } catch {
            print("[QueueManager] Failed to load existing files: \(error)")
        }
    }

    private func parseSimpleYAML(_ content: String) -> [String: Any]? {
        var result: [String: Any] = [:]
        var currentKey: String?
        var currentArray: [String] = []

        func flushArray() {
            if let key = currentKey, !currentArray.isEmpty {
                result[key] = currentArray
                currentArray = []
            }
        }

        for line in content.split(separator: "\n") {
            let trimmed = line.trimmingCharacters(in: .whitespaces)
            if trimmed.isEmpty || trimmed.hasPrefix("#") { continue }

            if trimmed.hasPrefix("- ") {
                if currentKey != nil {
                    let value = String(trimmed.dropFirst(2)).trimmingCharacters(in: .whitespaces)
                    let cleanValue = value.trimmingCharacters(in: CharacterSet(charactersIn: "\"'"))
                    currentArray.append(cleanValue)
                }
                continue
            }

            flushArray()
            currentKey = nil

            guard let colonIdx = trimmed.firstIndex(of: ":") else { continue }
            let key = String(trimmed[..<colonIdx]).trimmingCharacters(in: .whitespaces)
            let valueStr = String(trimmed[trimmed.index(after: colonIdx)...]).trimmingCharacters(in: .whitespaces)

            if valueStr.isEmpty || valueStr == "null" || valueStr == "~" {
                currentKey = key
                if valueStr == "null" || valueStr == "~" {
                    result[key] = nil
                    currentKey = nil
                }
            } else {
                let cleanValue = valueStr.trimmingCharacters(in: CharacterSet(charactersIn: "\"'"))
                if let intVal = Int(cleanValue) {
                    result[key] = intVal
                } else if let doubleVal = Double(cleanValue) {
                    result[key] = doubleVal
                } else if cleanValue == "true" {
                    result[key] = true
                } else if cleanValue == "false" {
                    result[key] = false
                } else {
                    result[key] = cleanValue
                }
            }
        }

        flushArray()
        return result.isEmpty ? nil : result
    }

    private func loadFromYAML(_ subdirURL: URL, _ subdir: String, _ projectDict: [String: Any], _ loadedCount: inout Int) {
        var item = QueueItem(
            source: projectDict["source"] as? String ?? subdir,
            sourceType: SourceType(rawValue: projectDict["source_type"] as? String ?? "markdown") ?? .markdown
        )
        item.originalSource = subdir
        item.title = projectDict["title"] as? String
        item.backend = projectDict["backend"] as? String
        item.model = projectDict["model"] as? String
        item.summary = projectDict["summary"] as? String
        item.topics = projectDict["topics"] as? [String]
        item.keyInsights = projectDict["key_insights"] as? [String]
        item.wordCount = projectDict["word_count"] as? Int

        let stateStr = projectDict["state"] as? String ?? "queued"
        item.state = ProcessingState(rawValue: stateStr) ?? .queued
        item.progress = projectDict["progress"] as? Double ?? 0
        item.statusMessage = item.state == .completed ? "Готово" : "Не завершено — готов к обработке"

        if item.state == .completed {
            item.outputURL = subdirURL.appendingPathComponent("output/final.md")
        }
        item.workspaceURL = subdirURL

        if let startedStr = projectDict["started_at"] as? String {
            let formatter = ISO8601DateFormatter()
            item.startedAt = formatter.date(from: startedStr)
        }
        if let completedStr = projectDict["completed_at"] as? String {
            let formatter = ISO8601DateFormatter()
            item.completedAt = formatter.date(from: completedStr)
        }

        let chunkDir = subdirURL.appendingPathComponent(subdir)
        if FileManager.default.fileExists(atPath: chunkDir.path) {
            loadChunks(for: &item, from: chunkDir)
        }

        items.append(item)
        loadedCount += 1
    }

    private func loadFromDirectory(_ subdirURL: URL, _ subdir: String, _ loadedCount: inout Int) {
        let outputURL = subdirURL.appendingPathComponent("output/final.md")
        let hasFinal = FileManager.default.fileExists(atPath: outputURL.path)
        let chunkDir = subdirURL.appendingPathComponent(subdir)
        let hasChunks = FileManager.default.fileExists(atPath: chunkDir.path)

        guard hasFinal || hasChunks else { return }

        let sourcePath = hasFinal ? outputURL.path : chunkDir.path
        var item = QueueItem(
            source: sourcePath,
            sourceType: .markdown
        )
        item.originalSource = subdir
        item.outputURL = hasFinal ? outputURL : nil
        item.workspaceURL = subdirURL
        item.state = hasFinal ? .completed : .queued
        item.statusMessage = hasFinal ? "Готово" : "Не завершено — готов к обработке"
        item.progress = hasFinal ? 1.0 : 0

        if hasChunks {
            loadChunks(for: &item, from: chunkDir)
        }

        if let attributes = try? FileManager.default.attributesOfItem(atPath: subdirURL.path),
           let modDate = attributes[.modificationDate] as? Date {
            item.completedAt = modDate
        }

        items.append(item)
        if hasFinal {
            loadMetadata(from: outputURL, index: items.count - 1)
        }
        loadedCount += 1
    }

    private func loadChunks(for item: inout QueueItem, from chunkDir: URL) {
        do {
            let files = try FileManager.default.contentsOfDirectory(atPath: chunkDir.path)
                .filter { $0.hasPrefix("chunk_") && $0.hasSuffix(".md") }
                .sorted()

            for (index, _) in files.enumerated() {
                let chunk = ChunkInfo(id: index + 1, status: .done)
                item.chunks.append(chunk)
            }
        } catch {
            print("[QueueManager] Failed to load chunks: \(error)")
        }
    }
}
