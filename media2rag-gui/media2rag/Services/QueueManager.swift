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

        let events = cliRunner.run(
            source: item.source,
            outputDir: settings.outputDirectory,
            backend: settings.backend,
            model: settings.model,
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
}
