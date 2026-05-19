import SwiftUI

struct SettingsView: View {
    @EnvironmentObject var settingsManager: SettingsManager
    @EnvironmentObject var modelManager: ModelManager
    @Environment(\.dismiss) var dismiss
    @State private var showDirectoryPicker = false
    @State private var showCLIPicker = false
    @State private var testResult = ""
    @State private var testSuccess = false

    var body: some View {
        NavigationStack {
            Form {
                Section("Бэкенд") {
                    Picker("LLM бэкенд", selection: $settingsManager.backend) {
                        Text("OpenRouter (облако)").tag("openrouter")
                        Text("Ollama (локально)").tag("ollama")
                    }
                    .pickerStyle(.segmented)
                    .onChange(of: settingsManager.backend) { _, newValue in
                        Task {
                            await modelManager.refreshModels(newValue, apiKey: settingsManager.openRouterApiKey)
                            if newValue == "openrouter" && !modelManager.openRouterModels.isEmpty {
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

                    if settingsManager.backend == "openrouter" {
                        SecureField("OpenRouter API ключ", text: $settingsManager.openRouterApiKey)
                            .textFieldStyle(.roundedBorder)
                            .onChange(of: settingsManager.openRouterApiKey) { _, newValue in
                                if !newValue.isEmpty {
                                    Task {
                                        await modelManager.refreshModels("openrouter", apiKey: newValue)
                                    }
                                }
                            }
                    }
                }

                Section("Обработка") {
                    Toggle("Только извлечение (без LLM)", isOn: $settingsManager.extractOnly)

                    Picker("Модель Whisper", selection: $settingsManager.whisperModel) {
                        Text("tiny (быстро)").tag("tiny")
                        Text("base").tag("base")
                        Text("small").tag("small")
                        Text("medium").tag("medium")
                        Text("large-v3 (лучше)").tag("large-v3")
                    }

                    TextField("Язык (авто)", text: $settingsManager.whisperLanguage)
                        .textFieldStyle(.roundedBorder)
                }

                Section("Вывод") {
                    HStack {
                        TextField("Папка вывода", text: $settingsManager.outputDirectory)
                            .textFieldStyle(.roundedBorder)

                        Button(action: { showDirectoryPicker = true }) {
                            Image(systemName: "folder")
                        }
                    }
                }

                Section("CLI путь") {
                    HStack {
                        TextField("Авто, если пусто", text: $settingsManager.cliPath)
                            .textFieldStyle(.roundedBorder)

                        Button(action: { showCLIPicker = true }) {
                            Image(systemName: "doc")
                        }
                    }

                    if settingsManager.cliPath.isEmpty {
                        Text("Используется встроенный CLI из ресурсов приложения")
                            .font(.caption)
                            .foregroundColor(.secondary)
                    } else if settingsManager.cliPath.hasSuffix(".py") {
                        Text("Запускается через uv run")
                            .font(.caption)
                            .foregroundColor(.secondary)
                    } else {
                        Text(settingsManager.cliPath)
                            .font(.caption)
                            .foregroundColor(.secondary)
                            .lineLimit(1)
                    }

                    Button("Проверить CLI") {
                        Task { await testCLI() }
                    }
                    .buttonStyle(.bordered)

                    if !testResult.isEmpty {
                        Text(testResult)
                            .font(.caption)
                            .foregroundColor(testSuccess ? .green : .red)
                    }
                }

                Button("Сбросить настройки") {
                    settingsManager.resetToDefaults()
                }
            }
            .formStyle(.grouped)
            .navigationTitle("Настройки")
            .toolbar {
                ToolbarItem(placement: .confirmationAction) {
                    Button("Готово") {
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
            .fileImporter(
                isPresented: $showCLIPicker,
                allowedContentTypes: [.plainText, .unixExecutable, .application]
            ) { result in
                if case .success(let url) = result {
                    settingsManager.cliPath = url.path
                }
            }
        }
        .frame(width: 520, height: 620)
    }

    private func testCLI() async {
        let cliPath = settingsManager.resolvedCLIPath
        if cliPath.isEmpty {
            testResult = "Путь к CLI не указан"
            testSuccess = false
            return
        }

        let process = Process()
        let pipe = Pipe()

        process.executableURL = URL(fileURLWithPath: "/Users/a1/.local/bin/uv")
        process.arguments = ["run", cliPath, "--help"]
        process.standardOutput = pipe
        process.standardError = pipe

        do {
            try process.run()
            process.waitUntilExit()

            let data = pipe.fileHandleForReading.readDataToEndOfFile()
            if let output = String(data: data, encoding: .utf8) {
                let firstLine = output.split(separator: "\n").first.map(String.init) ?? ""
                testResult = "CLI работает: \(firstLine)"
                testSuccess = true
            }
        } catch {
            testResult = "Ошибка: \(error.localizedDescription)"
            testSuccess = false
        }
    }
}

struct OllamaModelPicker: View {
    @ObservedObject var settingsManager: SettingsManager
    @ObservedObject var modelManager: ModelManager

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Text("Модель")
                Spacer()
                if modelManager.isLoading {
                    ProgressView()
                        .scaleEffect(0.7)
                } else {
                    Button(action: {
                        Task {
                            await modelManager.refreshModels("ollama", apiKey: "")
                        }
                    }) {
                        Image(systemName: "arrow.clockwise")
                    }
                    .buttonStyle(.borderless)
                }
            }

            Picker("Модель", selection: $settingsManager.model) {
                if modelManager.ollamaModels.isEmpty {
                    Text("Нет моделей").tag("")
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
                Text("Модель")
                Spacer()
                if modelManager.isLoading {
                    ProgressView()
                        .scaleEffect(0.7)
                } else {
                    Button(action: {
                        Task {
                            await modelManager.refreshModels("openrouter", apiKey: settingsManager.openRouterApiKey)
                        }
                    }) {
                        Image(systemName: "arrow.clockwise")
                    }
                    .buttonStyle(.borderless)
                }
            }

            Picker("Модель", selection: $settingsManager.model) {
                if modelManager.openRouterModels.isEmpty {
                    Text("Нет моделей").tag("")
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
