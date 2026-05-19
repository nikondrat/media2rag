import SwiftUI

@main
struct media2ragApp: App {
    @StateObject private var queueManager = QueueManager()
    @StateObject private var settingsManager = SettingsManager()

    var body: some Scene {
        WindowGroup {
            ContentView()
                .environmentObject(queueManager)
                .environmentObject(settingsManager)
        }
        .windowStyle(.hiddenTitleBar)
        .windowResizability(.contentSize)
        .commands {
            CommandGroup(replacing: .newItem) {}
        }

        Settings {
            SettingsView()
                .environmentObject(settingsManager)
        }
    }
}
