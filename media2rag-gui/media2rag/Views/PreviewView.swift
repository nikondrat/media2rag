import SwiftUI

struct PreviewView: View {
    let item: QueueItem
    @State private var markdownContent = ""
    @State private var showFinder = false

    var body: some View {
        VStack(spacing: 0) {
            // Header
            HStack {
                VStack(alignment: .leading, spacing: 4) {
                    Text(item.fileName)
                        .font(.headline)

                    HStack(spacing: 8) {
                        if let topics = item.topics, !topics.isEmpty {
                            ForEach(topics.prefix(3), id: \.self) { topic in
                                Text(topic)
                                    .font(.caption)
                                    .padding(.horizontal, 6)
                                    .padding(.vertical, 2)
                                    .background(Color.accentColor.opacity(0.1))
                                    .cornerRadius(4)
                            }
                        }

                        if let words = item.wordCount {
                            Text("\(words) words")
                                .font(.caption)
                                .foregroundColor(.secondary)
                        }
                    }
                }

                Spacer()

                if item.state == .completed {
                    Button(action: { openInFinder() }) {
                        Label("Finder", systemImage: "folder")
                    }
                }
            }
            .padding()

            Divider()

            // Content
            ScrollView {
                if item.state == .completed {
                    if !markdownContent.isEmpty {
                        Text(markdownContent)
                            .textSelection(.enabled)
                            .font(.system(.body, design: .monospaced))
                            .padding()
                    } else {
                        VStack(spacing: 16) {
                            Image(systemName: "doc.text")
                                .font(.system(size: 48))
                                .foregroundColor(.secondary)
                            Text("Output file ready")
                                .font(.title2)
                            if let url = item.outputURL {
                                Text(url.path)
                                    .font(.caption)
                                    .foregroundColor(.secondary)
                            }
                        }
                        .frame(maxWidth: .infinity, maxHeight: .infinity)
                    }
                } else if item.state == .failed {
                    VStack(spacing: 16) {
                        Image(systemName: "exclamationmark.triangle")
                            .font(.system(size: 48))
                            .foregroundColor(.red)
                        Text("Processing failed")
                            .font(.title2)
                        if let error = item.errorMessage {
                            Text(error)
                                .font(.caption)
                                .foregroundColor(.secondary)
                                .multilineTextAlignment(.center)
                        }
                    }
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
                } else {
                    VStack(spacing: 16) {
                        ProgressView()
                            .scaleEffect(1.5)
                        Text("Processing...")
                            .font(.title2)
                        Text(item.state.rawValue)
                            .font(.caption)
                            .foregroundColor(.secondary)
                    }
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
                }
            }
        }
        .frame(minWidth: 400)
        .onAppear {
            loadContent()
        }
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
