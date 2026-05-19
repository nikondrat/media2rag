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
    @Published var extractOnly: Bool {
        didSet { UserDefaults.standard.set(extractOnly, forKey: "extractOnly") }
    }
    @Published var openRouterApiKey: String {
        didSet { UserDefaults.standard.set(openRouterApiKey, forKey: "openRouterApiKey") }
    }

    init() {
        self.backend = UserDefaults.standard.string(forKey: "backend") ?? "openrouter"
        self.model = UserDefaults.standard.string(forKey: "model") ?? "qwen/qwen-plus"
        self.outputDirectory = UserDefaults.standard.string(forKey: "outputDirectory") ?? "\(NSHomeDirectory())/Documents/media2rag"
        self.whisperModel = UserDefaults.standard.string(forKey: "whisperModel") ?? "large-v3"
        self.whisperLanguage = UserDefaults.standard.string(forKey: "whisperLanguage") ?? "auto"
        self.cliPath = UserDefaults.standard.string(forKey: "cliPath") ?? ""
        self.extractOnly = UserDefaults.standard.bool(forKey: "extractOnly")
        self.openRouterApiKey = UserDefaults.standard.string(forKey: "openRouterApiKey") ?? ""
    }

    func resetToDefaults() {
        backend = "openrouter"
        model = "qwen/qwen-plus"
        outputDirectory = "\(NSHomeDirectory())/Documents/media2rag"
        whisperModel = "large-v3"
        whisperLanguage = "auto"
        cliPath = ""
        extractOnly = false
        openRouterApiKey = ""
    }

    var resolvedCLIPath: String {
        if !cliPath.isEmpty { return cliPath }
        let defaultPath = "/Users/a1/dev/tools/transcripts/cli.py"
        return defaultPath
    }
}
