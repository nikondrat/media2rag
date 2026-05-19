import Foundation

@MainActor
class QueueManager: ObservableObject {
    @Published var items: [QueueItem] = []
    @Published var activeItemId: UUID?
    @Published var isProcessing = false
    @Published var currentIndex = 0
    @Published var totalProcessed = 0
    @Published var totalErrors = 0

    private var cliRunner = CLIRunner()
    private var settingsManager: SettingsManager?

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
        var item = items[index]
        item.state = .extracting
        item.statusMessage = "Загрузка файла..."
        item.startedAt = Date()
        item.progress = 0
        items[index] = item

        let cliPath = settings.resolvedCLIPath
        if cliPath.isEmpty {
            items[index].state = .failed
            items[index].statusMessage = "Путь к CLI не указан"
            items[index].errorMessage = "Путь к CLI не указан"
            items[index].completedAt = Date()
            totalErrors += 1
            return
        }

        var args = [
            item.source,
            "-o", settings.outputDirectory,
            "--backend", settings.backend,
            "--model", settings.model,
            "--json"
        ]

        if settings.extractOnly {
            args.append("--extract-only")
        }

        let events = cliRunner.run(
            arguments: args,
            cliPath: cliPath
        )

        for await event in events {
            switch event.eventType {
            case "telegram_channel":
                items[index].statusMessage = "Скрапинг канала: \(event.totalPosts ?? 0) постов найдено"
                items[index].progress = 0.05
            case "telegram_progress":
                if let current = event.current, let total = event.total {
                    let pct = Double(current) / Double(total) * 0.3
                    items[index].progress = 0.05 + pct
                    items[index].statusMessage = "Пост \(current) из \(total)..."
                }
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
                items[index].state = .completed
                items[index].statusMessage = "Готово"
                items[index].progress = 1.0
                items[index].completedAt = Date()
                if let output = event.output {
                    items[index].outputURL = URL(fileURLWithPath: output)
                }
                totalProcessed += 1
            case "error":
                items[index].state = .failed
                items[index].statusMessage = "Ошибка"
                items[index].errorMessage = event.message ?? "Неизвестная ошибка"
                items[index].completedAt = Date()
                totalErrors += 1
            default:
                break
            }
        }

        if items[index].state == .completed {
            loadMetadata(from: items[index].outputURL, index: index)
        }
    }

    private func loadMetadata(from url: URL?, index: Int) {
        guard let url = url else { return }
        do {
            let content = try String(contentsOf: url, encoding: .utf8)
            if let frontmatter = parseFrontmatter(content) {
                items[index].topics = frontmatter.topics
                items[index].summary = frontmatter.summary
                items[index].keyInsights = frontmatter.keyInsights
            }
        } catch {
            // Ignore metadata parsing errors
        }
    }

    private func parseFrontmatter(_ content: String) -> (topics: [String]?, summary: String?, keyInsights: [String]?)? {
        guard content.hasPrefix("---") else { return nil }

        let parts = content.split(separator: "---", maxSplits: 2)
        guard parts.count >= 2 else { return nil }

        let yamlContent = String(parts[1])
        var topics: [String]?
        var summary: String?
        var keyInsights: [String]?

        for line in yamlContent.split(separator: "\n") {
            if line.hasPrefix("topics:") {
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

        return (topics: topics, summary: summary, keyInsights: keyInsights)
    }
}
