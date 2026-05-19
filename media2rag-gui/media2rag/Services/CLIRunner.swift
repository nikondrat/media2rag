import Foundation

class CLIRunner: ObservableObject {
    private var process: Process?
    private var outputPipe: Pipe?
    private var streamContinuation: AsyncStream<CLIJSONEvent>.Continuation?

    @MainActor
    func run(
        arguments: [String],
        cliPath: String
    ) -> AsyncStream<CLIJSONEvent> {
        let stream = AsyncStream<CLIJSONEvent> { [weak self] continuation in
            self?.streamContinuation = continuation
        }

        let continuation = streamContinuation
        let process = Process()
        let pipe = Pipe()

        if cliPath.hasSuffix(".py") {
            process.executableURL = URL(fileURLWithPath: "/usr/bin/env")
            process.arguments = ["uv", "run", cliPath] + arguments
        } else {
            process.executableURL = URL(fileURLWithPath: cliPath)
            process.arguments = arguments
        }

        process.standardOutput = pipe
        process.standardError = pipe

        pipe.fileHandleForReading.readabilityHandler = { [weak self] handle in
            let data = handle.availableData
            if data.isEmpty { return }

            guard let line = String(data: data, encoding: .utf8) else { return }

            for rawLine in line.split(separator: "\n") {
                let trimmed = rawLine.trimmingCharacters(in: .whitespacesAndNewlines)
                if trimmed.isEmpty { continue }

                if let jsonData = trimmed.data(using: .utf8),
                   let event = try? JSONDecoder().decode(CLIJSONEvent.self, from: jsonData) {
                    self?.streamContinuation?.yield(event)
                }
            }
        }

        process.terminationHandler = { [weak self] _ in
            self?.streamContinuation?.finish()
        }

        do {
            try process.run()
        } catch {
            let errEvent = CLIJSONEvent(eventType: "error", message: error.localizedDescription)
            continuation?.yield(errEvent)
            continuation?.finish()
        }

        self.process = process
        return stream
    }

    @MainActor
    func stop() {
        process?.terminate()
        streamContinuation?.finish()
        process = nil
        streamContinuation = nil
    }
}

struct CLIJSONEvent: Codable, Sendable {
    let eventType: String
    let file: String?
    let fileType: String?
    let words: Int?
    let chars: Int?
    let current: Int?
    let total: Int?
    let topics: [String]?
    let output: String?
    let message: String?
    let processed: Int?
    let errors: Int?

    enum CodingKeys: String, CodingKey {
        case eventType = "status"
        case file
        case fileType = "type"
        case words, chars, current, total, topics, output, message, processed, errors
    }

    init(eventType: String, file: String? = nil, fileType: String? = nil, words: Int? = nil,
         chars: Int? = nil, current: Int? = nil, total: Int? = nil, topics: [String]? = nil,
         output: String? = nil, message: String? = nil, processed: Int? = nil, errors: Int? = nil) {
        self.eventType = eventType
        self.file = file
        self.fileType = fileType
        self.words = words
        self.chars = chars
        self.current = current
        self.total = total
        self.topics = topics
        self.output = output
        self.message = message
        self.processed = processed
        self.errors = errors
    }
}
