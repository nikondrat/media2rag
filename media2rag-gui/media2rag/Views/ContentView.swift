import SwiftUI

struct ContentView: View {
    @EnvironmentObject var queueManager: QueueManager
    @EnvironmentObject var settingsManager: SettingsManager
    @State private var selectedItems = Set<UUID>()
    @State private var showSettings = false

    var body: some View {
        NavigationSplitView {
            QueueListView()
                .navigationSplitViewColumnWidth(min: 250, ideal: 300)
        } detail: {
            if let selectedItem = queueManager.items.first(where: { selectedItems.contains($0.id) }) {
                PreviewView(item: selectedItem)
            } else {
                EmptyStateView()
            }
        }
        .toolbar {
            ToolbarItem(placement: .primaryAction) {
                Button(action: {
                    Task { await queueManager.startProcessing() }
                }) {
                    Label("Start", systemImage: "play.circle")
                }
                .disabled(queueManager.isProcessing || queueManager.items.isEmpty)
            }

            ToolbarItem(placement: .primaryAction) {
                Button(action: {
                    queueManager.stopProcessing()
                }) {
                    Label("Stop", systemImage: "stop.circle")
                }
                .disabled(!queueManager.isProcessing)
            }

            ToolbarItem(placement: .primaryAction) {
                Button(action: {
                    queueManager.clearCompleted()
                }) {
                    Label("Clear", systemImage: "trash")
                }
                .disabled(queueManager.items.isEmpty)
            }
        }
        .onAppear {
            queueManager.setSettingsManager(settingsManager)
        }
    }
}

struct QueueListView: View {
    @EnvironmentObject var queueManager: QueueManager
    @State private var inputText = ""

    var body: some View {
        VStack(spacing: 0) {
            DropZoneView()
                .padding(.horizontal)
                .padding(.top)

            Divider()

            List(queueManager.items, selection: $queueManager.items) { item in
                QueueItemRow(item: item)
            }
            .listStyle(.sidebar)

            Divider()

            HStack(spacing: 8) {
                TextField("Paste URL or drag files...", text: $inputText)
                    .textFieldStyle(.roundedBorder)
                    .onSubmit {
                        if !inputText.isEmpty {
                            queueManager.addSource(inputText)
                            inputText = ""
                        }
                    }

                Button(action: {
                    if !inputText.isEmpty {
                        queueManager.addSource(inputText)
                        inputText = ""
                    }
                }) {
                    Image(systemName: "plus.circle.fill")
                }
            }
            .padding()
        }
        .frame(minWidth: 250)
    }
}

struct DropZoneView: View {
    @EnvironmentObject var queueManager: QueueManager

    var body: some View {
        VStack(spacing: 8) {
            Image(systemName: "square.and.arrow.down")
                .font(.system(size: 32))
                .foregroundColor(.secondary)

            Text("Drop files or paste URLs")
                .font(.headline)

            Text("PDF, EPUB, MP4, MP3, MD, images, YouTube, Telegram")
                .font(.caption)
                .foregroundColor(.secondary)
        }
        .frame(maxWidth: .infinity)
        .padding()
        .background(Color(nsColor: .windowBackgroundColor))
        .cornerRadius(8)
        .onDrop(of: [.fileURL, .URL], isTargeted: nil) { providers in
            for provider in providers {
                if provider.canLoadObject(ofClass: URL.self) {
                    provider.loadObject(ofClass: URL.self) { url, _ in
                        if let url = url {
                            Task { @MainActor in
                                queueManager.addSource(url.absoluteString)
                            }
                        }
                    }
                }
            }
            return true
        }
    }
}

struct QueueItemRow: View {
    let item: QueueItem

    var body: some View {
        HStack(spacing: 12) {
            Image(systemName: item.fileIcon)
                .foregroundColor(.secondary)

            VStack(alignment: .leading, spacing: 2) {
                Text(item.fileName)
                    .font(.body)
                    .lineLimit(1)

                HStack(spacing: 4) {
                    Image(systemName: item.state.icon)
                        .font(.caption2)
                        .foregroundColor(item.state.color == "accent" ? .accentColor : Color(item.state.color))

                    Text(item.state.rawValue)
                        .font(.caption2)
                        .foregroundColor(.secondary)

                    if let elapsed = item.elapsedTime {
                        Text("• \(elapsed)")
                            .font(.caption2)
                            .foregroundColor(.secondary)
                    }
                }
            }

            Spacer()

            if item.state == .completed {
                Image(systemName: "checkmark.circle.fill")
                    .foregroundColor(.green)
            } else if item.state == .failed {
                Image(systemName: "exclamationmark.circle.fill")
                    .foregroundColor(.red)
            } else if item.state != .queued {
                ProgressView(value: item.progress)
                    .frame(width: 60)
            }
        }
        .padding(.vertical, 4)
    }
}

struct EmptyStateView: View {
    var body: some View {
        VStack(spacing: 16) {
            Image(systemName: "doc.text.magnifyingglass")
                .font(.system(size: 48))
                .foregroundColor(.secondary)

            Text("Select a file to preview")
                .font(.title2)

            Text("Processed files will appear here")
                .font(.body)
                .foregroundColor(.secondary)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}
