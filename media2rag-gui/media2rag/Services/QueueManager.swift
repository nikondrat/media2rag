import Foundation

@MainActor
class QueueManager: ObservableObject {
    @Published var items: [QueueItem] = []
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

    func setSettingsManager(_ manager: SettingsManager) {
        self.settingsManager = manager
    }

    func addSource(_ source: String) {
        let item = QueueItem(
            source: source,
            sourceType: SourceType.from(source: source)
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

        var updatedItems = items
        for index in updatedItems.indices {
            guard updatedItems[index].state == .queued else { continue }

            currentIndex = index
            let updatedItem = await processItem(updatedItems[index], settings: settings)
            updatedItems[index] = updatedItem
            items = updatedItems
        }

        isProcessing = false
    }

    func stopProcessing() {
        cliRunner.stop()
        isProcessing = false
    }

    private func processItem(_ item: QueueItem, settings: SettingsManager) async -> QueueItem {
        var item = item
        item.state = .extracting
        item.startedAt = Date()
        item.progress = 0

        let cliPath = settings.resolvedCLIPath
        if cliPath.isEmpty {
            item.state = .failed
            item.errorMessage = "CLI path not configured"
            totalErrors += 1
            return item
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
            case "extracting":
                item.state = .extracting
                item.progress = 0.1
            case "extracted":
                item.wordCount = event.words
                item.progress = 0.2
            case "compression_start":
                item.state = .compressing
                item.progress = 0.3
            case "compressing_chunk", "compressed_chunk":
                if let current = event.current, let total = event.total {
                    let chunkProgress = Double(current) / Double(total) * 0.3
                    item.progress = 0.3 + chunkProgress
                }
            case "compression_done":
                item.progress = 0.6
            case "transformation_start":
                item.state = .transforming
            case "transformation_done":
                item.topics = event.topics
                item.progress = 0.8
            case "generation_start":
                item.state = .generating
            case "generation_done":
                item.progress = 0.95
            case "completed":
                item.state = .completed
                item.progress = 1.0
                item.completedAt = Date()
                if let output = event.output {
                    item.outputURL = URL(fileURLWithPath: output)
                    loadMetadata(from: item.outputURL, item: &item)
                }
                totalProcessed += 1
            case "error":
                item.state = .failed
                item.errorMessage = event.message ?? "Unknown error"
                item.completedAt = Date()
                totalErrors += 1
            default:
                break
            }
        }

        return item
    }

    private func loadMetadata(from url: URL?, item: inout QueueItem) {
        guard let url = url else { return }
        do {
            let content = try String(contentsOf: url, encoding: .utf8)
            if let frontmatter = parseFrontmatter(content) {
                item.topics = frontmatter.topics
                item.summary = frontmatter.summary
                item.keyInsights = frontmatter.keyInsights
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
