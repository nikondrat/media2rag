import SwiftUI

enum ToastType {
    case success
    case error
    case info

    var icon: String {
        switch self {
        case .success: return "checkmark.circle.fill"
        case .error: return "exclamationmark.circle.fill"
        case .info: return "info.circle.fill"
        }
    }

    var color: Color {
        switch self {
        case .success: return .green
        case .error: return .red
        case .info: return .blue
        }
    }
}

struct Toast: Identifiable, Equatable {
    let id: UUID
    let message: String
    let type: ToastType
    let duration: TimeInterval

    init(message: String, type: ToastType, duration: TimeInterval = 3) {
        self.id = UUID()
        self.message = message
        self.type = type
        self.duration = duration
    }
}

@MainActor
class ToastManager: ObservableObject {
    @Published var toasts: [Toast] = []
    private let maxVisible = 3

    func show(message: String, type: ToastType) {
        let toast = Toast(message: message, type: type)
        if toasts.count >= maxVisible {
            toasts.removeFirst()
        }
        toasts.append(toast)
        Task {
            try? await Task.sleep(nanoseconds: UInt64(toast.duration * 1_000_000_000))
            await MainActor.run {
                toasts.removeAll { $0.id == toast.id }
            }
        }
    }

    func dismiss(_ toast: Toast) {
        toasts.removeAll { $0.id == toast.id }
    }
}