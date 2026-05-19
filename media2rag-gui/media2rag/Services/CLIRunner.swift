import Foundation

@MainActor
class CLIRunner: ObservableObject {
    private var process: Process?
    private var outputPipe: Pipe?

    func run(
        source: String,
        outputDir: String,
        backend: String,
        model: String,
        cliPath: String
    ) -> AsyncStream<JSONEvent> {
        AsyncStream { continuation in
            let process = Process()
            let pipe = Pipe()

            process.executableURL = URL(fileURLWithPath: cliPath)
            process.arguments = [
                source,
                "-o", outputDir,
                "--backend", backend,
                "--model", model,
                "--json"
            ]

            process.standardOutput = pipe
            process.standardError = pipe

            outputPipe = pipe
            self.process = process

            pipe.fileHandleForReading.readabilityHandler = { handle in
                let data = handle.availableData
                if data.isEmpty { return }

                guard let line = String(data: data, encoding: .utf8) else { return }

                for rawLine in line.split(separator: "\n") {
                    let trimmed = rawLine.trimmingCharacters(in: .whitespacesAndNewlines)
                    if trimmed.isEmpty { continue }

                    if let jsonData = trimmed.data(using: .utf8),
                       let event = try? JSONDecoder().decode(JSONEvent.self, from: jsonData) {
                        continuation.yield(event)
                    }
                }
            }

            process.terminationHandler = { _ in
                continuation.finish()
            }

            do {
                try process.run()
            } catch {
                continuation.yield(JSONEvent(status: "error", message: error.localizedDescription))
                continuation.finish()
            }
        }
    }

    func stop() {
        process?.terminate()
        process = nil
    }
}

struct JSONEvent: Codable {
    let status: String
    let file: String?
    let type: String?
    let words: Int?
    let chars: Int?
    let current: Int?
    let total: Int?
    let topics: [String]?
    let output: String?
    let message: String?
    let processed: Int?
    let errors: Int?
}
