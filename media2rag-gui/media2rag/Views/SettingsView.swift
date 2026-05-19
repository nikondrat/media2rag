import SwiftUI

struct SettingsView: View {
    @EnvironmentObject var settingsManager: SettingsManager
    @State private var showDirectoryPicker = false

    var body: some View {
        TabView {
            generalTab
                .tabItem {
                    Label("General", systemImage: "gear")
                }

            modelsTab
                .tabItem {
                    Label("Models", systemImage: "brain")
                }
        }
        .frame(width: 400)
    }

    private var generalTab: some View {
        Form {
            Section("Backend") {
                Picker("LLM Backend", selection: $settingsManager.backend) {
                    Text("Ollama (local)").tag("ollama")
                    Text("OpenRouter (cloud)").tag("openrouter")
                }

                TextField("Model", text: $settingsManager.model)
                    .textFieldStyle(.roundedBorder)
            }

            Section("Output") {
                HStack {
                    TextField("Output directory", text: $settingsManager.outputDirectory)
                        .textFieldStyle(.roundedBorder)

                    Button(action: { showDirectoryPicker = true }) {
                        Image(systemName: "folder")
                    }
                }
            }

            Section("CLI") {
                TextField("CLI path (leave empty for bundled)", text: $settingsManager.cliPath)
                    .textFieldStyle(.roundedBorder)
            }

            Button("Reset to defaults") {
                settingsManager.resetToDefaults()
            }
        }
        .padding()
        .fileImporter(
            isPresented: $showDirectoryPicker,
            allowedContentTypes: [.folder]
        ) { result in
            if case .success(let url) = result {
                settingsManager.outputDirectory = url.path
            }
        }
    }

    private var modelsTab: some View {
        Form {
            Section("Whisper") {
                Picker("Model", selection: $settingsManager.whisperModel) {
                    Text("tiny").tag("tiny")
                    Text("base").tag("base")
                    Text("small").tag("small")
                    Text("medium").tag("medium")
                    Text("large-v3").tag("large-v3")
                }

                TextField("Language (auto for detection)", text: $settingsManager.whisperLanguage)
                    .textFieldStyle(.roundedBorder)
            }

            Section("Ollama Models") {
                Text("Default: gemma4:26b")
                    .font(.caption)
                    .foregroundColor(.secondary)
            }

            Section("OpenRouter Models") {
                Text("Default: qwen/qwen-plus")
                    .font(.caption)
                    .foregroundColor(.secondary)
            }
        }
        .padding()
    }
}
