import SwiftUI

struct SettingsView: View {
    @EnvironmentObject var settingsManager: SettingsManager
    @EnvironmentObject var modelManager: ModelManager
    @Environment(\.dismiss) var dismiss
    @State private var showDirectoryPicker = false

    var body: some View {
        NavigationStack {
            Form {
                Section("Backend") {
                    Picker("LLM Backend", selection: $settingsManager.backend) {
                        Text("OpenRouter (cloud)").tag("openrouter")
                        Text("Ollama (local)").tag("ollama")
                    }
                    .pickerStyle(.segmented)
                    .onChange(of: settingsManager.backend) { _, newValue in
                        Task {
                            await modelManager.refreshModels(newValue)
                            if newValue == "openrouter" && modelManager.openRouterModels.isEmpty {
                                settingsManager.model = "qwen/qwen-plus"
                            } else if newValue == "ollama" && modelManager.ollamaModels.isEmpty {
                                settingsManager.model = "gemma4:26b"
                            } else if newValue == "openrouter" && !modelManager.openRouterModels.isEmpty {
                                settingsManager.model = modelManager.openRouterModels.first?.id ?? "qwen/qwen-plus"
                            } else if newValue == "ollama" && !modelManager.ollamaModels.isEmpty {
                                settingsManager.model = modelManager.ollamaModels.first ?? "gemma4:26b"
                            }
                        }
                    }

                    if settingsManager.backend == "ollama" {
                        OllamaModelPicker(settingsManager: settingsManager, modelManager: modelManager)
                    } else {
                        OpenRouterModelPicker(settingsManager: settingsManager, modelManager: modelManager)
                    }
                }

                Section("Processing") {
                    Toggle("Extract only (skip LLM)", isOn: $settingsManager.extractOnly)

                    Picker("Whisper Model", selection: $settingsManager.whisperModel) {
                        Text("tiny (fast)").tag("tiny")
                        Text("base").tag("base")
                        Text("small").tag("small")
                        Text("medium").tag("medium")
                        Text("large-v3 (best)").tag("large-v3")
                    }

                    TextField("Language (auto for detection)", text: $settingsManager.whisperLanguage)
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

                Section("CLI Path") {
                    TextField("Leave empty for bundled CLI", text: $settingsManager.cliPath)
                        .textFieldStyle(.roundedBorder)

                    if settingsManager.cliPath.isEmpty {
                        Text("Using bundled CLI from app resources")
                            .font(.caption)
                            .foregroundColor(.secondary)
                    }
                }

                Button("Reset to defaults") {
                    settingsManager.resetToDefaults()
                }
            }
            .formStyle(.grouped)
            .navigationTitle("Settings")
            .toolbar {
                ToolbarItem(placement: .confirmationAction) {
                    Button("Done") {
                        dismiss()
                    }
                }
            }
            .fileImporter(
                isPresented: $showDirectoryPicker,
                allowedContentTypes: [.folder]
            ) { result in
                if case .success(let url) = result {
                    settingsManager.outputDirectory = url.path
                }
            }
        }
        .frame(width: 500, height: 600)
    }
}

struct OllamaModelPicker: View {
    @ObservedObject var settingsManager: SettingsManager
    @ObservedObject var modelManager: ModelManager

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Text("Model")
                Spacer()
                if modelManager.isLoading {
                    ProgressView()
                        .scaleEffect(0.7)
                } else {
                    Button(action: {
                        Task {
                            await modelManager.refreshModels("ollama")
                        }
                    }) {
                        Image(systemName: "arrow.clockwise")
                    }
                    .buttonStyle(.borderless)
                }
            }

            Picker("Model", selection: $settingsManager.model) {
                if modelManager.ollamaModels.isEmpty {
                    Text("No models found").tag("")
                } else {
                    ForEach(modelManager.ollamaModels, id: \.self) { model in
                        Text(model).tag(model)
                    }
                }
            }
            .pickerStyle(.menu)
        }
    }
}

struct OpenRouterModelPicker: View {
    @ObservedObject var settingsManager: SettingsManager
    @ObservedObject var modelManager: ModelManager

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Text("Model")
                Spacer()
                if modelManager.isLoading {
                    ProgressView()
                        .scaleEffect(0.7)
                } else {
                    Button(action: {
                        Task {
                            await modelManager.refreshModels("openrouter")
                        }
                    }) {
                        Image(systemName: "arrow.clockwise")
                    }
                    .buttonStyle(.borderless)
                }
            }

            Picker("Model", selection: $settingsManager.model) {
                if modelManager.openRouterModels.isEmpty {
                    Text("No models found").tag("")
                } else {
                    ForEach(modelManager.openRouterModels) { model in
                        Text(model.displayName).tag(model.id)
                    }
                }
            }
            .pickerStyle(.menu)
        }
    }
}
