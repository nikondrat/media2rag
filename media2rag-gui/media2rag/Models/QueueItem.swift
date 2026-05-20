import Foundation
import SwiftUI

enum ChunkStatus: String, Codable {
    case queued
    case processing
    case done
    case error
    case skipped

    var icon: String {
        switch self {
        case .queued: return "circle"
        case .processing: return "arrow.triangle.2.circlepath"
        case .done: return "checkmark.circle.fill"
        case .error: return "exclamationmark.circle.fill"
        case .skipped: return "circle.slash"
        }
    }

    var color: Color {
        switch self {
        case .queued: return .secondary.opacity(0.4)
        case .processing: return .blue
        case .done: return .green
        case .error: return .red
        case .skipped: return .orange
        }
    }

    var label: String {
        switch self {
        case .queued: return "В очереди"
        case .processing: return "Обработка"
        case .done: return "Готово"
        case .error: return "Ошибка"
        case .skipped: return "Пропущен"
        }
    }
}

struct ChunkInfo: Identifiable, Codable, Equatable {
    let id: Int
    var status: ChunkStatus = .queued
    var wordCount: Int?
    var errorMessage: String?
    var contentOffset: Int?
    var contentLength: Int?
}

struct SectionIndex: Identifiable {
    let id: Int
    let title: String
    let offset: Int
    let length: Int
    let level: Int
}

enum ProcessingState: String, Codable {
    case queued = "queued"
    case extracting = "extracting"
    case compressing = "compressing"
    case transforming = "transforming"
    case mapReduce = "map_reduce"
    case generating = "generating"
    case completed = "completed"
    case failed = "failed"

    var icon: String {
        switch self {
        case .queued: return "circle"
        case .extracting: return "arrow.down.circle"
        case .compressing: return "rectangle.compress.vertical"
        case .transforming: return "text.justify"
        case .mapReduce: return "rectangle.stack"
        case .generating: return "doc.text"
        case .completed: return "checkmark.circle"
        case .failed: return "exclamationmark.circle"
        }
    }

    var iconColor: Color {
        switch self {
        case .queued: return .secondary.opacity(0.4)
        case .extracting, .compressing, .transforming, .mapReduce, .generating: return .accentColor
        case .completed: return .green
        case .failed: return .red
        }
    }
}

struct QueueItem: Identifiable, Equatable {
    let id = UUID()
    let source: String
    let sourceType: SourceType
    var state: ProcessingState = .queued
    var progress: Double = 0
    var statusMessage: String = ""
    var outputURL: URL?
    var outputFiles: [String]?
    var workspaceURL: URL?
    var errorMessage: String?
    var wordCount: Int?
    var topics: [String]?
    var keyInsights: [String]?
    var summary: String?
    var title: String?
    var startedAt: Date?
    var completedAt: Date?
    var chunks: [ChunkInfo] = []
    var backend: String?
    var model: String?

    var isTelegramChannel: Bool {
        sourceType == .telegram && (outputFiles?.count ?? 0) > 1
    }

    var displayTitle: String {
        title ?? fileName
    }

    var fileName: String {
        if let url = URL(string: source) {
            return url.host ?? source
        }
        return (source as NSString).lastPathComponent
    }

    var fileIcon: String {
        switch sourceType {
        case .pdf: return "doc"
        case .epub: return "book"
        case .video: return "film"
        case .audio: return "waveform"
        case .image: return "photo"
        case .markdown: return "doc.text"
        case .url: return "link"
        case .telegram: return "paperplane"
        }
    }

    var stateLabel: String {
        switch self.state {
        case .queued: return "В очереди"
        case .extracting: return "Извлечение"
        case .compressing: return "Сжатие"
        case .transforming: return "Трансформация"
        case .mapReduce: return "Обработка чанков"
        case .generating: return "Генерация"
        case .completed: return "Готово"
        case .failed: return "Ошибка"
        }
    }

    var elapsedTime: String? {
        guard let started = startedAt else { return nil }
        let end = completedAt ?? Date()
        let interval = end.timeIntervalSince(started)
        if interval < 60 {
            return "\(Int(interval))s"
        }
        return "\(Int(interval / 60))m \(Int(interval.truncatingRemainder(dividingBy: 60)))s"
    }
}

enum SourceType: String, Codable {
    case pdf, epub, video, audio, image, markdown, url, telegram

    var color: Color {
        switch self {
        case .pdf: return .red
        case .epub: return .blue
        case .video: return .purple
        case .audio: return .green
        case .image: return .pink
        case .markdown: return .gray
        case .url: return .teal
        case .telegram: return .cyan
        }
    }

    static func from(source: String) -> SourceType {
        if source.hasPrefix("http") {
            if source.contains("t.me/") || source.contains("telegram.me/") {
                return .telegram
            }
            if source.contains("youtube.com") || source.contains("youtu.be") || source.contains("vimeo.com") {
                return .video
            }
            return .url
        }

        let ext = (source as NSString).pathExtension.lowercased()
        switch ext {
        case "pdf": return .pdf
        case "epub": return .epub
        case "mp4", "mkv", "avi", "mov", "webm": return .video
        case "mp3", "wav", "m4a", "flac", "ogg", "aac": return .audio
        case "png", "jpg", "jpeg", "webp", "bmp", "tiff": return .image
        case "md": return .markdown
        default: return .url
        }
    }
}
