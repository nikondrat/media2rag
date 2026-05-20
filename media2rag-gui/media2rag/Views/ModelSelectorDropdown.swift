import SwiftUI

struct ModelSelectorDropdown: View {
    @EnvironmentObject var modelManager: ModelManager
    @EnvironmentObject var settingsManager: SettingsManager
    
    @Binding var selectedBackend: String
    @Binding var selectedModel: String
    
    @State private var showMenu = false
    @State private var isLoading = false
    
    var onModelChanged: ((String, String) -> Void)?
    
    var currentModels: [String] {
        if selectedBackend == "ollama" {
            return modelManager.ollamaModels
        } else {
            return modelManager.openRouterModels.map { $0.id }
        }
    }
    
    var body: some View {
        Menu {
            Section("Backend") {
                Button("Ollama") {
                    switchBackend("ollama")
                }
                .tint(selectedBackend == "ollama" ? .accentColor : .primary)
                
                Button("OpenRouter") {
                    switchBackend("openrouter")
                }
                .tint(selectedBackend == "openrouter" ? .accentColor : .primary)
            }
            
            if !currentModels.isEmpty {
                Divider()
                Section("Models") {
                    ForEach(currentModels, id: \.self) { model in
                        Button(model) {
                            selectModel(model)
                        }
                        .tint(model == selectedModel ? .accentColor : .primary)
                    }
                }
            }
        } label: {
            HStack(spacing: 4) {
                Image(systemName: "brain")
                    .font(.system(size: 10))
                Text(modelLabel)
                    .font(.caption2)
                    .lineLimit(1)
                Image(systemName: "chevron.down")
                    .font(.system(size: 7))
            }
            .foregroundColor(.secondary)
            .padding(.horizontal, 6)
            .padding(.vertical, 2)
            .background(Color.secondary.opacity(0.1), in: RoundedRectangle(cornerRadius: 4))
        }
        .menuStyle(.borderlessButton)
        .fixedSize()
    }
    
    private var modelLabel: String {
        if selectedModel.isEmpty {
            return selectedBackend == "ollama" ? "Ollama" : "OpenRouter"
        }
        let short = selectedModel.split(separator: "/").last ?? Substring(selectedModel)
        return String(short)
    }
    
    private func switchBackend(_ backend: String) {
        selectedBackend = backend
        Task {
            isLoading = true
            defer { isLoading = false }
            await modelManager.refreshModels(backend, apiKey: settingsManager.openRouterApiKey)
            
            if !currentModels.isEmpty {
                selectedModel = currentModels.first ?? ""
                onModelChanged?(selectedBackend, selectedModel)
            }
        }
    }
    
    private func selectModel(_ model: String) {
        selectedModel = model
        onModelChanged?(selectedBackend, selectedModel)
    }
}
