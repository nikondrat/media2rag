import Foundation

@MainActor
class ModelManager: ObservableObject {
    @Published var ollamaModels: [String] = []
    @Published var openRouterModels: [OpenRouterModel] = []
    @Published var isLoading = false

    func refreshModels(_ backend: String) async {
        isLoading = true
        defer { isLoading = false }

        if backend == "ollama" {
            await fetchOllamaModels()
        } else {
            await fetchOpenRouterModels()
        }
    }

    private func fetchOllamaModels() async {
        guard let url = URL(string: "http://localhost:11434/api/tags") else { return }

        do {
            let (data, _) = try await URLSession.shared.data(from: url)
            let response = try JSONDecoder().decode(OllamaTagsResponse.self, from: data)
            ollamaModels = response.models.map {
                let name = $0.name
                return name.contains(":") ? name : name + ":latest"
            }.sorted()
        } catch {
            ollamaModels = ["gemma4:26b", "qwen3.6:35b"]
        }
    }

    private func fetchOpenRouterModels() async {
        guard let url = URL(string: "https://openrouter.ai/api/v1/models") else { return }

        do {
            let (data, _) = try await URLSession.shared.data(from: url)
            let response = try JSONDecoder().decode(OpenRouterModelsResponse.self, from: data)
            openRouterModels = response.data
                .filter { $0.architecture?.modality == "text/text" }
                .sorted { $0.name < $1.name }
        } catch {
            openRouterModels = [
                OpenRouterModel(id: "qwen/qwen-plus", name: "Qwen Plus", contextLength: 131072, architecture: nil),
                OpenRouterModel(id: "anthropic/claude-sonnet-4", name: "Claude Sonnet 4", contextLength: 200000, architecture: nil),
                OpenRouterModel(id: "openai/gpt-4o", name: "GPT-4o", contextLength: 128000, architecture: nil),
            ]
        }
    }
}

struct OllamaTagsResponse: Codable {
    let models: [OllamaModel]
}

struct OllamaModel: Codable {
    let name: String
}

struct OpenRouterModelsResponse: Codable {
    let data: [OpenRouterModel]
}

struct OpenRouterModel: Codable, Identifiable {
    let id: String
    let name: String
    let contextLength: Int?
    let architecture: Architecture?

    var displayName: String {
        if let ctx = contextLength {
            let ctxStr = ctx >= 1000 ? "\(ctx / 1000)K" : "\(ctx)"
            return "\(name) (\(ctxStr) ctx)"
        }
        return name
    }

    enum CodingKeys: String, CodingKey {
        case id, name
        case contextLength = "context_length"
        case architecture
    }
}

struct Architecture: Codable {
    let modality: String?
}
