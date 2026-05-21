import SwiftUI

@main
struct media2ragApp: App {
    @StateObject private var queueManager = QueueManager()
    @StateObject private var settingsManager = SettingsManager()
    @StateObject private var modelManager = ModelManager()

    var body: some Scene {
        WindowGroup {
            ContentView()
                .environmentObject(queueManager)
                .environmentObject(settingsManager)
                .environmentObject(modelManager)
                .onAppear {
                    Task {
                        await modelManager.refreshModels(settingsManager.backend, apiKey: settingsManager.openRouterApiKey)
                    }
                }
        }
        .windowStyle(.hiddenTitleBar)
        .windowToolbarStyle(.unified)
        .windowResizability(.contentMinSize)
        .commands {
            CommandGroup(replacing: .newItem) {}
            CommandMenu("Обработка") {
                Button("Запустить") {
                    Task { await queueManager.startProcessing() }
                }
                .keyboardShortcut("r", modifiers: .command)
                .disabled(queueManager.isProcessing || queueManager.items.isEmpty)

                Button("Остановить") {
                    queueManager.stopProcessing()
                }
                .keyboardShortcut(".", modifiers: .command)
                .disabled(!queueManager.isProcessing)

                Divider()

                Button("Очистить выполненные") {
                    queueManager.clearCompleted()
                }
                .keyboardShortcut("k", modifiers: [.command, .shift])

                Button("Очистить всё") {
                    queueManager.clearAll()
                }
                .keyboardShortcut("k", modifiers: [.command, .option])

                Divider()

                Button("Обработать выбранный") {
                    if let selId = queueManager.selectedItemId,
                       let item = queueManager.items.first(where: { $0.id == selId }) {
                        queueManager.processSingle(item)
                    } else if let first = queueManager.items.first(where: { $0.state == .queued }) {
                        queueManager.processSingle(first)
                    }
                }
                .keyboardShortcut(.return, modifiers: .command)
                .disabled(queueManager.isProcessing || queueManager.items.isEmpty)

                Button("Удалить выбранный") {
                    if let selId = queueManager.selectedItemId,
                       let item = queueManager.items.first(where: { $0.id == selId }) {
                        queueManager.removeItem(item)
                    }
                }
                .keyboardShortcut(.delete, modifiers: [])
                .disabled(queueManager.items.isEmpty)
            }
        }

        Settings {
            SettingsView()
                .environmentObject(settingsManager)
                .environmentObject(modelManager)
        }
    }
}
