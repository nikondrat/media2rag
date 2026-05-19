import SwiftUI

struct ContentView: View {
    @EnvironmentObject var queueManager: QueueManager
    @EnvironmentObject var settingsManager: SettingsManager
    @EnvironmentObject var modelManager: ModelManager
    @State private var selectedItemId: UUID?
    @State private var showSettings = false
    @State private var searchText = ""
    @State private var filterType: SourceTypeFilter = .all

    var filteredItems: [QueueItem] {
        var items = queueManager.items

        if !searchText.isEmpty {
            items = items.filter {
                $0.fileName.localizedCaseInsensitiveContains(searchText)
            }
        }

        if filterType != .all {
            items = items.filter { $0.sourceType.rawValue == filterType.rawValue }
        }

        return items
    }

    var selectedItem: QueueItem? {
        queueManager.items.first { $0.id == selectedItemId }
    }

    var body: some View {
        NavigationSplitView {
            sidebar
                .navigationSplitViewColumnWidth(min: 280, ideal: 320, max: 400)
        } detail: {
            if let item = selectedItem {
                DetailView(item: item)
            } else {
                EmptyStateView()
            }
        }
        .navigationSplitViewStyle(.prominentDetail)
        .toolbar {
            ToolbarItemGroup(placement: .primaryAction) {
                Button(action: {
                    Task { await queueManager.startProcessing() }
                }) {
                    Label("Start", systemImage: "play.fill")
                }
                .buttonStyle(.borderedProminent)
                .disabled(queueManager.isProcessing || queueManager.queuedCount == 0)

                Button(action: {
                    queueManager.stopProcessing()
                }) {
                    Label("Stop", systemImage: "stop.fill")
                }
                .buttonStyle(.bordered)
                .tint(.red)
                .disabled(!queueManager.isProcessing)

                Divider()

                Button(action: {
                    queueManager.clearCompleted()
                }) {
                    Image(systemName: "trash")
                }
                .disabled(queueManager.completedCount == 0)

                Button(action: { showSettings = true }) {
                    Image(systemName: "gearshape")
                }
            }
        }
        .sheet(isPresented: $showSettings) {
            SettingsView()
                .environmentObject(settingsManager)
                .environmentObject(modelManager)
        }
        .onAppear {
            queueManager.setSettingsManager(settingsManager)
        }
    }

    private var sidebar: some View {
        VStack(spacing: 0) {
            DropZoneView()
                .padding(.horizontal, 12)
                .padding(.vertical, 8)

            Divider()

            HStack(spacing: 8) {
                Image(systemName: "magnifyingglass")
                    .foregroundColor(.secondary)
                TextField("Search files...", text: $searchText)
                    .textFieldStyle(.plain)

                Picker("Filter", selection: $filterType) {
                    ForEach(SourceTypeFilter.allCases, id: \.self) { filter in
                        Text(filter.label).tag(filter)
                    }
                }
                .pickerStyle(.menu)
                .labelsHidden()
                .frame(width: 80)
            }
            .padding(.horizontal, 12)
            .padding(.vertical, 8)

            Divider()

            if filteredItems.isEmpty {
                VStack(spacing: 8) {
                    Image(systemName: searchText.isEmpty ? "square.and.arrow.down" : "magnifyingglass")
                        .font(.system(size: 28))
                        .foregroundColor(.secondary)
                    Text(searchText.isEmpty ? "No files yet" : "No matches")
                        .font(.caption)
                        .foregroundColor(.secondary)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else {
                List(filteredItems, selection: Binding<UUID?>(
                    get: { selectedItemId },
                    set: { selectedItemId = $0 }
                )) { item in
                    QueueItemRow(item: item, isSelected: item.id == selectedItemId)
                        .tag(item.id)
                        .listRowInsets(EdgeInsets(top: 2, leading: 8, bottom: 2, trailing: 8))
                }
                .listStyle(.sidebar)
            }

            Divider()

            StatusBarView()
                .padding(.horizontal, 12)
                .padding(.vertical, 6)
        }
        .frame(minWidth: 280)
    }
}

struct DropZoneView: View {
    @EnvironmentObject var queueManager: QueueManager
    @State private var isHovering = false

    var body: some View {
        VStack(spacing: 6) {
            Image(systemName: "square.and.arrow.down.on.square")
                .font(.system(size: 24))
                .foregroundColor(isHovering ? .accentColor : .secondary)

            Text("Drop files or paste URLs")
                .font(.subheadline)
                .fontWeight(.medium)

            Text("PDF, EPUB, MP4, MP3, MD, YouTube, Telegram")
                .font(.caption2)
                .foregroundColor(.secondary)
        }
        .frame(maxWidth: .infinity)
        .padding(.vertical, 16)
        .background(
            RoundedRectangle(cornerRadius: 10)
                .fill(isHovering ? Color.accentColor.opacity(0.08) : Color(nsColor: .controlBackgroundColor))
                .overlay(
                    RoundedRectangle(cornerRadius: 10)
                        .stroke(isHovering ? Color.accentColor.opacity(0.4) : Color(nsColor: .separatorColor), lineWidth: isHovering ? 2 : 1)
                )
        )
        .onDrop(of: [.fileURL, .url], isTargeted: $isHovering) { providers in
            for provider in providers {
                _ = provider.loadObject(ofClass: URL.self) { url, _ in
                    if let url = url {
                        Task { @MainActor in
                            queueManager.addSource(url.absoluteString)
                        }
                    }
                }
            }
            return true
        }
        .animation(.easeInOut(duration: 0.2), value: isHovering)
    }
}

struct QueueItemRow: View {
    let item: QueueItem
    let isSelected: Bool

    var body: some View {
        HStack(spacing: 10) {
            Image(systemName: item.fileIcon)
                .font(.system(size: 16))
                .foregroundColor(item.state.iconColor)
                .frame(width: 24, height: 24)

            VStack(alignment: .leading, spacing: 3) {
                Text(item.fileName)
                    .font(.body)
                    .lineLimit(1)
                    .foregroundStyle(isSelected ? .primary : .primary)

                HStack(spacing: 6) {
                    Image(systemName: item.state.icon)
                        .font(.system(size: 9))
                        .foregroundColor(item.state.iconColor)

                    Text(item.stateLabel)
                        .font(.caption2)
                        .foregroundColor(.secondary)

                    if let elapsed = item.elapsedTime {
                        Text("• \(elapsed)")
                            .font(.caption2)
                            .foregroundColor(.secondary)
                    }

                    if let words = item.wordCount {
                        Text("• \(formatWords(words))")
                            .font(.caption2)
                            .foregroundColor(.secondary)
                    }
                }
            }

            Spacer()

            if item.state == .completed {
                Image(systemName: "checkmark.circle.fill")
                    .foregroundColor(.green)
                    .font(.system(size: 14))
            } else if item.state == .failed {
                Image(systemName: "exclamationmark.circle.fill")
                    .foregroundColor(.red)
                    .font(.system(size: 14))
            } else if item.state != .queued {
                ProgressView(value: item.progress)
                    .progressViewStyle(.linear)
                    .frame(width: 50)
            }
        }
        .padding(.vertical, 6)
        .contentShape(Rectangle())
    }

    private func formatWords(_ count: Int) -> String {
        if count >= 1000 {
            return "\(count / 1000)K"
        }
        return "\(count)"
    }
}

struct StatusBarView: View {
    @EnvironmentObject var queueManager: QueueManager

    var body: some View {
        HStack(spacing: 16) {
            Label("\(queueManager.totalCount)", systemImage: "doc")
                .font(.caption)
                .foregroundColor(.secondary)

            Label("\(queueManager.completedCount)", systemImage: "checkmark.circle")
                .font(.caption)
                .foregroundColor(.green)

            Label("\(queueManager.failedCount)", systemImage: "exclamationmark.circle")
                .font(.caption)
                .foregroundColor(.red)

            Spacer()

            if queueManager.isProcessing {
                ProgressView()
                    .scaleEffect(0.7)
                Text("Processing...")
                    .font(.caption)
                    .foregroundColor(.secondary)
            }
        }
    }
}

struct EmptyStateView: View {
    var body: some View {
        VStack(spacing: 20) {
            Image(systemName: "doc.text.magnifyingglass")
                .font(.system(size: 56))
                .foregroundColor(.secondary.opacity(0.6))

            Text("Select a file to preview")
                .font(.title2)
                .fontWeight(.medium)

            Text("Processed files will appear here with full content preview")
                .font(.body)
                .foregroundColor(.secondary)
                .multilineTextAlignment(.center)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}

enum SourceTypeFilter: String, CaseIterable {
    case all = "all"
    case pdf = "pdf"
    case epub = "epub"
    case video = "video"
    case audio = "audio"
    case url = "url"
    case telegram = "telegram"
    case markdown = "markdown"

    var label: String {
        switch self {
        case .all: return "All"
        case .pdf: return "PDF"
        case .epub: return "EPUB"
        case .video: return "Video"
        case .audio: return "Audio"
        case .url: return "URL"
        case .telegram: return "Telegram"
        case .markdown: return "MD"
        }
    }
}
