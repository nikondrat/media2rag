import Foundation

class CLIRunner: ObservableObject {
    private var process: Process?
    private var streamContinuation: AsyncStream<CLIJSONEvent>.Continuation?
    private var timeoutTask: Task<Void, Never>?

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
        let outPipe = Pipe()
        let errPipe = Pipe()
        let stderrLock = NSLock()
        var stderrData = Data()
        var lastEventTime = Date()

        let env = ProcessInfo.processInfo.environment
        var fullEnv = env
        fullEnv["PATH"] = "/Users/a1/.local/bin:/Users/a1/.cargo/bin:/opt/homebrew/bin:/opt/homebrew/sbin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"
        fullEnv["HOME"] = NSHomeDirectory()

        process.environment = fullEnv

        if cliPath.hasSuffix(".py") {
            let projectDir = URL(fileURLWithPath: cliPath).deletingLastPathComponent().path
            process.executableURL = URL(fileURLWithPath: "/Users/a1/.local/bin/uv")
            process.arguments = ["run", cliPath] + arguments
            process.currentDirectoryURL = URL(fileURLWithPath: projectDir)
        } else {
            process.executableURL = URL(fileURLWithPath: cliPath)
            process.arguments = arguments
        }

        process.standardOutput = outPipe
        process.standardError = errPipe

        errPipe.fileHandleForReading.readabilityHandler = { handle in
            let data = handle.availableData
            if !data.isEmpty {
                stderrLock.lock()
                stderrData.append(data)
                stderrLock.unlock()
            }
        }

        outPipe.fileHandleForReading.readabilityHandler = { [weak self] handle in
            let data = handle.availableData
            if data.isEmpty { return }

            guard let line = String(data: data, encoding: .utf8) else { return }

            for rawLine in line.split(separator: "\n") {
                let trimmed = rawLine.trimmingCharacters(in: .whitespacesAndNewlines)
                if trimmed.isEmpty { continue }

                if let jsonData = trimmed.data(using: .utf8),
                   let event = try? JSONDecoder().decode(CLIJSONEvent.self, from: jsonData) {
                    lastEventTime = Date()
                    self?.streamContinuation?.yield(event)
                }
            }
        }

        // Timeout: kill process if no output for 5 minutes
        timeoutTask = Task { [weak self] in
            try? await Task.sleep(for: .seconds(300))
            guard !Task.isCancelled, let self = self else { return }

            stderrLock.lock()
            let errMsg = String(data: stderrData, encoding: .utf8) ?? "Превышено время ожидания (5 мин)"
            stderrLock.unlock()

            let errEvent = CLIJSONEvent(eventType: "error", message: "Таймаут: \(errMsg)")
            self.streamContinuation?.yield(errEvent)
            self.streamContinuation?.finish()
            process.terminate()
        }

        process.terminationHandler = { [weak self] proc in
            self?.timeoutTask?.cancel()

            stderrLock.lock()
            let errMsg = String(data: stderrData, encoding: .utf8) ?? "Process exited with code \(proc.terminationStatus)"
            stderrLock.unlock()

            if proc.terminationStatus != 0 {
                let errEvent = CLIJSONEvent(eventType: "error", message: errMsg)
                self?.streamContinuation?.yield(errEvent)
            }
            self?.streamContinuation?.finish()
        }

        do {
            try process.run()
        } catch {
            timeoutTask?.cancel()
            let errEvent = CLIJSONEvent(eventType: "error", message: "Не удалось запустить CLI: \(error.localizedDescription)")
            continuation?.yield(errEvent)
            continuation?.finish()
        }

        self.process = process
        return stream
    }

    @MainActor
    func stop() {
        timeoutTask?.cancel()
        process?.terminate()
        streamContinuation?.finish()
        process = nil
        streamContinuation = nil
        timeoutTask = nil
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
    let totalPosts: Int?
    let postId: String?
    let postUrl: String?
    let channel: String?
    let outputFiles: [String]?

    enum CodingKeys: String, CodingKey {
        case eventType = "status"
        case file
        case fileType = "type"
        case words, chars, current, total, topics, output, message, processed, errors
        case totalPosts = "total_posts"
        case postId = "post_id"
        case postUrl = "post_url"
        case channel
        case outputFiles = "output_files"
    }

    init(eventType: String, file: String? = nil, fileType: String? = nil, words: Int? = nil,
         chars: Int? = nil, current: Int? = nil, total: Int? = nil, topics: [String]? = nil,
         output: String? = nil, message: String? = nil, processed: Int? = nil, errors: Int? = nil,
         totalPosts: Int? = nil, postId: String? = nil, postUrl: String? = nil, channel: String? = nil,
         outputFiles: [String]? = nil) {
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
        self.totalPosts = totalPosts
        self.postId = postId
        self.postUrl = postUrl
        self.channel = channel
        self.outputFiles = outputFiles
    }
}
