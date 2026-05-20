import SwiftUI
import UniformTypeIdentifiers

struct DetailView: View {
    let itemId: UUID
    @EnvironmentObject var queueManager: QueueManager
    @State private var markdownContent = ""
    @State private var intermediateContent = ""
    @State private var showSavePanel = false
    @State private var previewMode: PreviewMode = .formatted
    @State private var mmapReader: MmapFileReader?
    @State private var sections: [SectionIndex] = []
    @State private var sectionCache: [Int: String] = [:]
    @State private var lazySectionsLoaded = false
    @State private var currentPage = 0
    @State private var sectionsPerPage = 50
    private let paginationThreshold = 200

    var item: QueueItem? {
        queueManager.items.first { $0.id == itemId }
    }

    enum PreviewMode: String, CaseIterable {
        case formatted = "Предпросмотр"
        case intermediate = "Промежуточный"
        case raw = "Исходник"
    }

    var body: some View {
        Group {
            if let item = item {
                content(for: item)
            } else {
                Text("Файл не найден")
                    .foregroundColor(.secondary)
            }
        }
    }

    @ViewBuilder
    private func content(for item: QueueItem) -> some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 20) {
                headerSection(for: item)

                if item.state == .completed {
                    metadataSection(for: item)
                    Divider()
                    contentSection(for: item)
                } else if item.state == .failed {
                    errorSection(for: item)
                } else if item.state == .queued {
                    queuedSection
                } else {
                    processingSection(for: item)
                }
            }
            .padding(24)
        }
        .frame(minWidth: 500)
        .id(itemId)
        .onChange(of: item.state) { _, newState in
            if newState == .completed {
                loadContent(from: item.outputURL)
                loadIntermediate(from: item.workspaceURL?.appendingPathComponent("intermediate/raw.md"))
                initMmapReader(for: item.outputURL)
            }
        }
        .onAppear {
            if item.state == .completed {
                loadContent(from: item.outputURL)
                loadIntermediate(from: item.workspaceURL?.appendingPathComponent("intermediate/raw.md"))
                initMmapReader(for: item.outputURL)
            }
        }
        .onDisappear {
            mmapReader?.close()
            mmapReader = nil
        }
        .onChange(of: previewMode) { _, newMode in
            if newMode == .intermediate, let wsURL = item.workspaceURL, intermediateContent.isEmpty {
                loadIntermediate(from: wsURL.appendingPathComponent("intermediate/raw.md"))
            }
        }
        .fileExporter(
            isPresented: $showSavePanel,
            document: MarkdownDocument(content: markdownContent),
            contentType: .plainText,
            defaultFilename: item.displayTitle + ".md"
        ) { result in
            if case .success(let url) = result {
                try? markdownContent.write(to: url, atomically: true, encoding: .utf8)
            }
        }
    }

    private func headerSection(for item: QueueItem) -> some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack {
                Image(systemName: item.fileIcon)
                    .font(.system(size: 28))
                    .foregroundColor(.accentColor)

                VStack(alignment: .leading, spacing: 4) {
                    Text(item.displayTitle)
                        .font(.title2)
                        .fontWeight(.semibold)
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

    private func metadataSection(for item: QueueItem) -> some View {
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

    private func contentSection(for item: QueueItem) -> some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack {
                Text("Содержимое")
                    .font(.headline)

                Spacer()

                if !item.isTelegramChannel {
                    Picker("Режим", selection: $previewMode) {
                        ForEach(PreviewMode.allCases, id: \.self) { mode in
                            Text(mode.rawValue).tag(mode)
                        }
                    }
                    .pickerStyle(.segmented)
                    .frame(width: 180)

                    Button(action: { loadContent(from: item.outputURL) }) {
                        Image(systemName: "arrow.clockwise")
                    }
                    .buttonStyle(.borderless)
                    .help("Обновить")
                }
            }

            if item.isTelegramChannel {
                channelFilesView(for: item)
            } else if !markdownContent.isEmpty {
                switch previewMode {
                case .formatted:
                    formattedPreview
                case .intermediate:
                    intermediateMarkdownView
                case .raw:
                    rawMarkdownView
                }
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
                    Button("Загрузить содержимое") {
                        loadContent(from: item.outputURL)
                    }
                    .buttonStyle(.bordered)
                }
                .frame(maxWidth: .infinity)
                .padding()
            }
        }
    }

    private func channelFilesView(for item: QueueItem) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("\(item.outputFiles?.count ?? 0) постов обработано")
                .font(.caption)
                .foregroundColor(.secondary)
                .padding(.bottom, 4)

            ScrollView {
                VStack(alignment: .leading, spacing: 4) {
                    ForEach(item.outputFiles ?? [], id: \.self) { filePath in
                        Button(action: { openFile(filePath) }) {
                            HStack {
                                Image(systemName: "doc.text")
                                    .foregroundColor(.accentColor)
                                Text((filePath as NSString).lastPathComponent)
                                    .font(.body)
                                Spacer()
                                Image(systemName: "arrow.up.right")
                                    .font(.caption)
                                    .foregroundColor(.secondary)
                            }
                            .padding(.vertical, 6)
                            .padding(.horizontal, 10)
                            .background(Color(nsColor: .controlBackgroundColor))
                            .cornerRadius(8)
                        }
                        .buttonStyle(.plain)
                    }
                }
            }
            .frame(maxHeight: 400)
        }
        .padding()
        .background(Color(nsColor: .controlBackgroundColor))
        .cornerRadius(12)
    }

    private func openFile(_ path: String) {
        NSWorkspace.shared.open(URL(fileURLWithPath: path))
    }

    private var totalPages: Int {
        guard !sections.isEmpty else { return 0 }
        return (sections.count + sectionsPerPage - 1) / sectionsPerPage
    }

    private var showPagination: Bool {
        sections.count > paginationThreshold
    }

    private var paginatedSectionRange: Range<Int> {
        let start = currentPage * sectionsPerPage
        let end = min(start + sectionsPerPage, sections.count)
        return start..<end
    }

    private var formattedPreview: some View {
        Group {
            if !sections.isEmpty && lazySectionsLoaded {
                VStack(spacing: 0) {
                    ScrollView {
                        LazyVStack(alignment: .leading, spacing: 12) {
                            ForEach(Array(paginatedSectionRange), id: \.self) { index in
                                if let content = loadSectionContent(index) {
                                    let parsedSections = parseMarkdownSections(content)
                                    ForEach(parsedSections, id: \.id) { section in
                                        groupSectionView(section)
                                    }
                                }
                            }
                        }
                        .padding()
                    }

                    if showPagination {
                        pageNavigation
                    }
                }
                .background(Color(nsColor: .controlBackgroundColor))
                .cornerRadius(12)
            } else if !markdownContent.isEmpty {
                VStack(alignment: .leading, spacing: 12) {
                    ForEach(parseMarkdownSections(markdownContent), id: \.id) { section in
                        groupSectionView(section)
                    }
                }
                .padding()
                .background(Color(nsColor: .controlBackgroundColor))
                .cornerRadius(12)
            } else {
                Color.clear
            }
        }
        .id(currentPage)
    }

    private var pageNavigation: some View {
        HStack(spacing: 8) {
            Button(action: { changePage(to: currentPage - 1) }) {
                Image(systemName: "chevron.left")
            }
            .disabled(currentPage == 0)

            ForEach(pageNumbers, id: \.self) { page in
                Button(action: { changePage(to: page) }) {
                    Text("\(page + 1)")
                        .font(.caption)
                        .padding(.horizontal, 8)
                        .padding(.vertical, 4)
                        .background(page == currentPage ? Color.accentColor : Color.clear)
                        .foregroundColor(page == currentPage ? .white : .primary)
                        .cornerRadius(4)
                }
            }

            Button(action: { changePage(to: currentPage + 1) }) {
                Image(systemName: "chevron.right")
            }
            .disabled(currentPage >= totalPages - 1)
        }
        .padding(.vertical, 8)
        .padding(.horizontal, 12)
        .background(Color(nsColor: .windowBackgroundColor))
    }

    private var pageNumbers: [Int] {
        guard totalPages > 0 else { return [] }
        var pages: [Int] = []
        let maxVisible = 7

        if totalPages <= maxVisible {
            pages = Array(0..<totalPages)
        } else {
            pages.append(0)
            let start = max(1, currentPage - 1)
            let end = min(totalPages - 2, currentPage + 1)
            if start > 1 { pages.append(-1) }
            for p in start...end { pages.append(p) }
            if end < totalPages - 2 { pages.append(-2) }
            pages.append(totalPages - 1)
        }

        return pages
    }

    private func changePage(to page: Int) {
        guard page >= 0 && page < totalPages else { return }
        currentPage = page
    }

    private func loadSectionContent(_ index: Int) -> String? {
        if let cached = sectionCache[index] {
            return cached
        }
        if let reader = mmapReader, !sections.isEmpty {
            if let content = reader.getSectionContent(index: index, sections: sections) {
                sectionCache[index] = content
                return content
            }
        }
        return nil
    }

    @ViewBuilder
    private func groupSectionView(_ section: MarkdownSection) -> some View {
        switch section.type {
        case .heading(let level):
            Text(section.text)
                .font(headingFont(for: level))
                .fontWeight(.semibold)
                .padding(.top, level == 1 ? 12 : 8)
                .padding(.bottom, 4)
        case .bulletList(let items):
            VStack(alignment: .leading, spacing: 4) {
                ForEach(items, id: \.self) { itemText in
                    HStack(alignment: .top, spacing: 8) {
                        Text("•")
                            .fontWeight(.bold)
                            .foregroundColor(.accentColor)
                        RichText(text: itemText)
                    }
                }
            }
            .padding(.vertical, 4)
        case .paragraph:
            RichText(text: section.text)
                .padding(.vertical, 2)
        case .blockquote:
            RichText(text: section.text)
                .padding(.leading, 12)
                .padding(.vertical, 4)
                .frame(maxWidth: .infinity, alignment: .leading)
                .overlay(
                    Rectangle()
                        .fill(Color.accentColor.opacity(0.4))
                        .frame(width: 3)
                        .padding(.trailing, 8),
                    alignment: .leading
                )
        }
    }

    private func headingFont(for level: Int) -> Font {
        switch level {
        case 1: return .title2
        case 2: return .title3
        case 3: return .headline
        default: return .body.weight(.semibold)
        }
    }

    private var rawMarkdownView: some View {
        Text(markdownContent)
            .textSelection(.enabled)
            .font(.system(.body, design: .monospaced))
            .foregroundColor(.primary)
            .lineSpacing(4)
            .padding()
            .background(Color(nsColor: .controlBackgroundColor))
            .cornerRadius(12)
    }

    private var intermediateMarkdownView: some View {
        Group {
            if intermediateContent.isEmpty {
                VStack(spacing: 12) {
                    Image(systemName: "doc.text")
                        .font(.system(size: 36))
                        .foregroundColor(.secondary)
                    Text("Промежуточный файл не найден")
                        .font(.title3)
                    if let url = item?.workspaceURL {
                        let intermediateURL = url.appendingPathComponent("intermediate/raw.md")
                        Button("Открыть в Finder") {
                            NSWorkspace.shared.activateFileViewerSelecting([intermediateURL])
                        }
                        .buttonStyle(.bordered)
                    }
                }
                .frame(maxWidth: .infinity)
                .padding()
            } else {
                Text(intermediateContent)
                    .textSelection(.enabled)
                    .font(.system(.body, design: .monospaced))
                    .foregroundColor(.secondary)
                    .lineSpacing(4)
                    .padding()
                    .background(Color(nsColor: .controlBackgroundColor))
                    .cornerRadius(12)
            }
        }
    }

    private func parseMarkdownSections(_ content: String) -> [MarkdownSection] {
        var sections: [MarkdownSection] = []
        let lines = content.split(separator: "\n", omittingEmptySubsequences: false)

        var currentText = ""
        var currentType: MarkdownSectionType = .paragraph
        var currentLevel = 0
        var bulletItems: [String] = []
        var inFrontmatter = false

        func flushCurrent() {
            if !currentText.isEmpty {
                sections.append(MarkdownSection(id: UUID(), text: currentText, level: currentLevel, type: currentType))
                currentText = ""
            }
        }

        func flushBullets() {
            if !bulletItems.isEmpty {
                let joined = bulletItems.joined(separator: "\n")
                sections.append(MarkdownSection(id: UUID(), text: joined, level: 0, type: .bulletList(bulletItems)))
                bulletItems = []
            }
        }

        for line in lines {
            let trimmed = line.trimmingCharacters(in: .whitespaces)

            if trimmed == "---" {
                if sections.isEmpty && !inFrontmatter {
                    inFrontmatter = true
                    continue
                } else if inFrontmatter {
                    inFrontmatter = false
                    continue
                }
            }

            if inFrontmatter {
                continue
            }

            let headingMatch = trimmed.firstIndex(of: "#")
            if let idx = headingMatch, idx == trimmed.startIndex {
                flushBullets()
                flushCurrent()
                let hashCount = trimmed.prefix { $0 == "#" }.count
                let text = String(trimmed.dropFirst(hashCount)).trimmingCharacters(in: .whitespaces)
                let cleanText = stripHTML(text)
                currentText = cleanText
                currentLevel = hashCount
                currentType = .heading(hashCount)
                continue
            }

            if trimmed.hasPrefix("- ") || trimmed.hasPrefix("* ") || trimmed.hasPrefix("+ ") {
                flushCurrent()
                let itemText = String(trimmed.dropFirst(2)).trimmingCharacters(in: .whitespaces)
                bulletItems.append(stripHTML(itemText))
                continue
            }

            if !bulletItems.isEmpty {
                flushBullets()
            }

            if trimmed.hasPrefix("> ") {
                flushCurrent()
                let quoteText = String(trimmed.dropFirst(2)).trimmingCharacters(in: .whitespaces)
                if currentType != .blockquote {
                    flushCurrent()
                    currentType = .blockquote
                }
                if !currentText.isEmpty {
                    currentText += "\n" + stripHTML(quoteText)
                } else {
                    currentText = stripHTML(quoteText)
                }
                continue
            }

            if currentType != .paragraph {
                flushCurrent()
                currentType = .paragraph
            }

            if !trimmed.isEmpty {
                if !currentText.isEmpty {
                    currentText += "\n" + stripHTML(trimmed)
                } else {
                    currentText = stripHTML(trimmed)
                }
            } else {
                flushCurrent()
            }
        }

        flushBullets()
        flushCurrent()

        return sections
    }

    private func stripHTML(_ text: String) -> String {
        var result = text
        while let range = result.range(of: "<[^>]+>", options: .regularExpression) {
            result.removeSubrange(range)
        }
        result = result.replacingOccurrences(of: "&amp;", with: "&")
        result = result.replacingOccurrences(of: "&lt;", with: "<")
        result = result.replacingOccurrences(of: "&gt;", with: ">")
        result = result.replacingOccurrences(of: "&quot;", with: "\"")
        result = result.replacingOccurrences(of: "&#39;", with: "'")
        result = result.replacingOccurrences(of: "&nbsp;", with: " ")
        return result.trimmingCharacters(in: .whitespaces)
    }

    private func errorSection(for item: QueueItem) -> some View {
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

    private var queuedSection: some View {
        VStack(spacing: 16) {
            Image(systemName: "circle.dashed")
                .font(.system(size: 48))
                .foregroundColor(.secondary.opacity(0.5))

            Text("Файл в очереди")
                .font(.title2)
                .fontWeight(.medium)

            Text("Нажмите «Запустить» для начала обработки")
                .font(.body)
                .foregroundColor(.secondary)
        }
        .frame(maxWidth: .infinity, minHeight: 300)
    }

    private func processingSection(for item: QueueItem) -> some View {
        VStack(spacing: 20) {
            ProgressView()
                .scaleEffect(1.2)

            VStack(spacing: 8) {
                Text(item.stateLabel)
                    .font(.title2)
                    .fontWeight(.medium)

                Text(item.statusMessage)
                    .font(.body)
                    .foregroundColor(.secondary)
                    .multilineTextAlignment(.center)
            }

            ProgressView(value: item.progress)
                .progressViewStyle(.linear)
                .frame(width: 250)

            if let elapsed = item.elapsedTime {
                Text("Время: \(elapsed)")
                    .font(.caption)
                    .foregroundColor(.secondary)
            }
        }
        .frame(maxWidth: .infinity, minHeight: 300)
    }

    private func loadContent(from url: URL?) {
        guard let url = url else { return }
        do {
            markdownContent = try String(contentsOf: url, encoding: .utf8)
        } catch {
            markdownContent = "Не удалось загрузить: \(error.localizedDescription)"
        }
    }

    private func initMmapReader(for url: URL?) {
        guard let url = url, FileManager.default.fileExists(atPath: url.path) else { return }
        let reader = MmapFileReader(url: url)
        do {
            try reader.open()
            let scannedSections = reader.scanSections()
            mmapReader = reader
            sections = scannedSections
            lazySectionsLoaded = !scannedSections.isEmpty
        } catch {
            mmapReader = nil
            sections = []
            lazySectionsLoaded = false
        }
    }

    private func loadIntermediate(from url: URL?) {
        guard let workspaceURL = item?.workspaceURL else {
            guard let url = url else { return }
            do {
                intermediateContent = try String(contentsOf: url, encoding: .utf8)
            } catch {
                intermediateContent = ""
            }
            return
        }
        let intermediatePath = workspaceURL.appendingPathComponent("intermediate/raw.md")
        do {
            intermediateContent = try String(contentsOf: intermediatePath, encoding: .utf8)
        } catch {
            intermediateContent = ""
        }
    }

    private func openInFinder() {
        guard let url = item?.outputURL else { return }
        NSWorkspace.shared.activateFileViewerSelecting([url])
    }
}

struct RichText: View {
    let text: String

    var body: some View {
        if let attributed = try? AttributedString(markdown: text) {
            Text(attributed)
                .font(.body)
                .lineSpacing(4)
        } else {
            Text(text)
                .font(.body)
                .lineSpacing(4)
        }
    }
}

enum MarkdownSectionType: Equatable {
    case heading(Int)
    case paragraph
    case bulletList([String])
    case blockquote

    static func == (lhs: MarkdownSectionType, rhs: MarkdownSectionType) -> Bool {
        switch (lhs, rhs) {
        case (.heading(let l), .heading(let r)): return l == r
        case (.paragraph, .paragraph): return true
        case (.bulletList, .bulletList): return true
        case (.blockquote, .blockquote): return true
        default: return false
        }
    }
}

struct MarkdownSection: Identifiable {
    let id: UUID
    let text: String
    let level: Int
    let type: MarkdownSectionType
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
        case .mapReduce: return "Обработка"
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
