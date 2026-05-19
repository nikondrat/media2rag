import Foundation

@MainActor
class SettingsManager: ObservableObject {
    @Published var backend: String {
        didSet { UserDefaults.standard.set(backend, forKey: "backend") }
    }
    @Published var model: String {
        didSet { UserDefaults.standard.set(model, forKey: "model") }
    }
    @Published var outputDirectory: String {
        didSet { UserDefaults.standard.set(outputDirectory, forKey: "outputDirectory") }
    }
    @Published var whisperModel: String {
        didSet { UserDefaults.standard.set(whisperModel, forKey: "whisperModel") }
    }
    @Published var whisperLanguage: String {
        didSet { UserDefaults.standard.set(whisperLanguage, forKey: "whisperLanguage") }
    }
    @Published var cliPath: String {
        didSet { UserDefaults.standard.set(cliPath, forKey: "cliPath") }
    }

    init() {
        self.backend = UserDefaults.standard.string(forKey: "backend") ?? "ollama"
        self.model = UserDefaults.standard.string(forKey: "model") ?? "gemma4:26b"
        self.outputDirectory = UserDefaults.standard.string(forKey: "outputDirectory") ?? "\(NSHomeDirectory())/Documents/media2rag"
        self.whisperModel = UserDefaults.standard.string(forKey: "whisperModel") ?? "large-v3"
        self.whisperLanguage = UserDefaults.standard.string(forKey: "whisperLanguage") ?? "auto"
        self.cliPath = UserDefaults.standard.string(forKey: "cliPath") ?? ""
    }

    func resetToDefaults() {
        backend = "ollama"
        model = "gemma4:26b"
        outputDirectory = "\(NSHomeDirectory())/Documents/media2rag"
        whisperModel = "large-v3"
        whisperLanguage = "auto"
        cliPath = ""
    }

    var resolvedCLIPath: String {
        if !cliPath.isEmpty { return cliPath }
        return Bundle.main.path(forResource: "cli", ofType: "py") ?? ""
    }
}
