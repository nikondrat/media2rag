import SwiftUI

struct DetailView: View {
    let item: QueueItem
    @State private var markdownContent = ""
    @State private var showInFinder = false

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

                if item.state == .completed {
                    Button(action: { openInFinder() }) {
                        Label("Finder", systemImage: "folder")
                    }
                    .buttonStyle(.bordered)
                }
            }

            HStack(spacing: 8) {
                BadgeView(text: item.sourceType.rawValue.uppercased(), color: .accentColor)

                if let words = item.wordCount {
                    BadgeView(text: "\(words) words", color: .secondary)
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
                    Text("Summary")
                        .font(.headline)
                    Text(summary)
                        .font(.body)
                        .foregroundColor(.secondary)
                        .lineLimit(3)
                }
            }

            if let topics = item.topics, !topics.isEmpty {
                VStack(alignment: .leading, spacing: 8) {
                    Text("Topics")
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
                    Text("Key Insights")
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
            Text("Content")
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
                    Text("Output file ready")
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

            Text("Processing failed")
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
                .scaleEffect(1.5)

            Text("Processing...")
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
            markdownContent = "Failed to load: \(error.localizedDescription)"
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

    var body: some View {
        HStack(spacing: 4) {
            Image(systemName: state.icon)
                .font(.system(size: 10))
            Text(state.rawValue)
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
