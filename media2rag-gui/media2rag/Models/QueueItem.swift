import Foundation

enum ProcessingState: String, Codable {
    case queued = "queued"
    case extracting = "extracting"
    case compressing = "compressing"
    case transforming = "transforming"
    case generating = "generating"
    case completed = "completed"
    case failed = "failed"

    var icon: String {
        switch self {
        case .queued: return "clock"
        case .extracting: return "arrow.down.circle"
        case .compressing: return "rectangle.compress.vertical"
        case .transforming: return "text.justify"
        case .generating: return "doc.text"
        case .completed: return "checkmark.circle"
        case .failed: return "exclamationmark.circle"
        }
    }

    var color: String {
        switch self {
        case .queued: return "secondary"
        case .extracting, .compressing, .transforming, .generating: return "accent"
        case .completed: return "green"
        case .failed: return "red"
        }
    }
}

struct QueueItem: Identifiable, Equatable {
    let id = UUID()
    let source: String
    let sourceType: SourceType
    var state: ProcessingState = .queued
    var progress: Double = 0
    var outputURL: URL?
    var errorMessage: String?
    var wordCount: Int?
    var topics: [String]?
    var startedAt: Date?
    var completedAt: Date?

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

    var elapsedTime: String? {
        guard let started = startedAt else { return nil }
        let end = completedAt ?? Date()
        let interval = end.timeIntervalSince(started)
        if interval < 60 {
            return "\(Int(interval))с"
        }
        return "\(Int(interval / 60))м \(Int(interval.truncatingRemainder(dividingBy: 60)))с"
    }
}

enum SourceType: String, Codable {
    case pdf, epub, video, audio, image, markdown, url, telegram

    static func from(source: String) -> SourceType {
        if source.hasPrefix("http") {
            if source.contains("t.me/") || source.contains("telegram.me/") {
                return .telegram
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
