import SwiftUI
import UniformTypeIdentifiers

struct DetailView: View {
    let item: QueueItem
    @State private var markdownContent = ""
    @State private var showSavePanel = false

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 20) {
                headerSection

                if item.state == .completed {
                    metadataSection
                    Divider()
                    contentSection
                } else if item.state == .failed {
                    errorSection
                } else {
                    processingSection
                }
            }
            .padding(24)
        }
        .frame(minWidth: 500)
        .onAppear {
            loadContent()
        }
        .fileExporter(
            isPresented: $showSavePanel,
            document: MarkdownDocument(content: markdownContent),
            contentType: .plainText,
            defaultFilename: item.fileName + ".md"
        ) { result in
            if case .success(let url) = result {
                try? markdownContent.write(to: url, atomically: true, encoding: .utf8)
            }
        }
    }

    private var headerSection: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack {
                Image(systemName: item.fileIcon)
                    .font(.system(size: 28))
                    .foregroundColor(.accentColor)

                VStack(alignment: .leading, spacing: 4) {
                    Text(item.fileName)
                        .font(.title2)
                        .fontWeight(.semibold)

                    Text(item.source)
                        .font(.caption)
                        .foregroundColor(.secondary)
                        .lineLimit(1)
                }

                Spacer()

                HStack(spacing: 8) {
                    if item.state == .completed {
                        Button(action: { openInFinder() }) {
                            Label("Finder", systemImage: "folder")
                        }
                        .buttonStyle(.bordered)
                    }

                    if item.state == .completed && !markdownContent.isEmpty {
                        Button(action: { showSavePanel = true }) {
                            Label("Сохранить как...", systemImage: "square.and.arrow.down")
                        }
                        .buttonStyle(.borderedProminent)
                    }
                }
            }

            HStack(spacing: 8) {
                BadgeView(text: item.sourceType.rawValue.uppercased(), color: .accentColor)

                if let words = item.wordCount {
                    BadgeView(text: "\(words) слов", color: .secondary)
                }

                if let elapsed = item.elapsedTime {
                    BadgeView(text: elapsed, color: .secondary)
                }

                StateBadgeView(state: item.state)
            }
        }
    }

    private var metadataSection: some View {
        VStack(alignment: .leading, spacing: 16) {
            if let summary = item.summary {
                VStack(alignment: .leading, spacing: 8) {
                    Text("Сводка")
                        .font(.headline)
                    Text(summary)
                        .font(.body)
                        .foregroundColor(.secondary)
                        .lineLimit(3)
                }
            }

            if let topics = item.topics, !topics.isEmpty {
                VStack(alignment: .leading, spacing: 8) {
                    Text("Темы")
                        .font(.headline)
                    FlowLayout(items: topics) { topic in
                        Text(topic)
                            .font(.caption)
                            .padding(.horizontal, 8)
                            .padding(.vertical, 4)
                            .background(Color.accentColor.opacity(0.15))
                            .cornerRadius(6)
                    }
                }
            }

            if let insights = item.keyInsights, !insights.isEmpty {
                VStack(alignment: .leading, spacing: 8) {
                    Text("Ключевые идеи")
                        .font(.headline)
                    ForEach(insights.prefix(5), id: \.self) { insight in
                        HStack(alignment: .top, spacing: 8) {
                            Image(systemName: "lightbulb.fill")
                                .font(.system(size: 10))
                                .foregroundColor(.yellow)
                            Text(insight)
                                .font(.caption)
                                .foregroundColor(.secondary)
                        }
                    }
                }
            }
        }
        .padding()
        .background(Color(nsColor: .controlBackgroundColor))
        .cornerRadius(12)
    }

    private var contentSection: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Содержимое")
                .font(.headline)

            if !markdownContent.isEmpty {
                Text(markdownContent)
                    .textSelection(.enabled)
                    .font(.system(.body, design: .monospaced))
                    .foregroundColor(.primary)
                    .lineSpacing(4)
            } else {
                VStack(spacing: 12) {
                    Image(systemName: "doc.text")
                        .font(.system(size: 36))
                        .foregroundColor(.secondary)
                    Text("Файл готов")
                        .font(.title3)
                    if let url = item.outputURL {
                        Text(url.lastPathComponent)
                            .font(.caption)
                            .foregroundColor(.secondary)
                    }
                }
                .frame(maxWidth: .infinity)
                .padding()
            }
        }
    }

    private var errorSection: some View {
        VStack(spacing: 16) {
            Image(systemName: "exclamationmark.triangle.fill")
                .font(.system(size: 48))
                .foregroundColor(.red)

            Text("Ошибка обработки")
                .font(.title2)
                .fontWeight(.semibold)

            if let error = item.errorMessage {
                Text(error)
                    .font(.body)
                    .foregroundColor(.secondary)
                    .multilineTextAlignment(.center)
                    .padding(.horizontal)
            }
        }
        .frame(maxWidth: .infinity)
        .padding()
    }

    private var processingSection: some View {
        VStack(spacing: 20) {
            ProgressView()
                .scaleEffect(1.2)

            Text("Обработка...")
                .font(.title2)
                .fontWeight(.medium)

            Text(item.stateLabel)
                .font(.body)
                .foregroundColor(.secondary)

            ProgressView(value: item.progress)
                .progressViewStyle(.linear)
                .frame(width: 200)
        }
        .frame(maxWidth: .infinity, minHeight: 300)
    }

    private func loadContent() {
        guard let url = item.outputURL else { return }
        do {
            markdownContent = try String(contentsOf: url, encoding: .utf8)
        } catch {
            markdownContent = "Не удалось загрузить: \(error.localizedDescription)"
        }
    }

    private func openInFinder() {
        if let url = item.outputURL {
            NSWorkspace.shared.activateFileViewerSelecting([url])
        }
    }
}

struct BadgeView: View {
    let text: String
    let color: Color

    var body: some View {
        Text(text)
            .font(.caption2)
            .fontWeight(.medium)
            .padding(.horizontal, 8)
            .padding(.vertical, 3)
            .background(color.opacity(0.15))
            .cornerRadius(6)
    }
}

struct StateBadgeView: View {
    let state: ProcessingState

    var label: String {
        switch state {
        case .queued: return "Ожидает"
        case .extracting: return "Извлечение"
        case .compressing: return "Сжатие"
        case .transforming: return "Трансформация"
        case .generating: return "Генерация"
        case .completed: return "Готово"
        case .failed: return "Ошибка"
        }
    }

    var body: some View {
        HStack(spacing: 4) {
            Image(systemName: state.icon)
                .font(.system(size: 10))
            Text(label)
                .font(.caption2)
                .fontWeight(.medium)
        }
        .padding(.horizontal, 8)
        .padding(.vertical, 3)
        .background(state.iconColor.opacity(0.15))
        .cornerRadius(6)
    }
}

struct FlowLayout<Data: Sequence, Content: View>: View where Data.Element: Hashable {
    let items: Data
    let content: (Data.Element) -> Content

    var body: some View {
        LazyVGrid(columns: [GridItem(.adaptive(minimum: 120, maximum: 200), spacing: 8)], spacing: 8) {
            ForEach(Array(items), id: \.self) { item in
                content(item)
            }
        }
    }
}

struct MarkdownDocument: FileDocument {
    static var readableContentTypes: [UTType] { [.plainText] }

    var content: String

    init(content: String) {
        self.content = content
    }

    init(configuration: ReadConfiguration) throws {
        guard let data = configuration.file.regularFileContents,
              let string = String(data: data, encoding: .utf8) else {
            throw CocoaError(.fileReadCorruptFile)
        }
        content = string
    }

    func fileWrapper(configuration: WriteConfiguration) throws -> FileWrapper {
        let data = content.data(using: .utf8) ?? Data()
        return FileWrapper(regularFileWithContents: data)
    }
}
