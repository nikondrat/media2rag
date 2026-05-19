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
                        await modelManager.refreshModels(settingsManager.backend)
                    }
                }
        }
        .windowStyle(.hiddenTitleBar)
        .windowToolbarStyle(.unified)
        .windowResizability(.contentMinSize)
        .commands {
            CommandGroup(replacing: .newItem) {}
            CommandMenu("Processing") {
                Button("Start Processing") {
                    Task { await queueManager.startProcessing() }
                }
                .keyboardShortcut("r", modifiers: .command)
                .disabled(queueManager.isProcessing || queueManager.items.isEmpty)

                Button("Stop Processing") {
                    queueManager.stopProcessing()
                }
                .keyboardShortcut(".", modifiers: .command)
                .disabled(!queueManager.isProcessing)

                Divider()

                Button("Clear Completed") {
                    queueManager.clearCompleted()
                }
                .keyboardShortcut("k", modifiers: [.command, .shift])
            }
        }

        Settings {
            SettingsView()
                .environmentObject(settingsManager)
                .environmentObject(modelManager)
        }
    }
}
