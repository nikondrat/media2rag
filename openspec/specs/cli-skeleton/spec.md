## ADDED Requirements

### Requirement: Root command
The CLI SHALL have a root command `media2rag` with `--help` flag showing available subcommands.

#### Scenario: Root help
- **WHEN** `media2rag --help` is executed
- **THEN** it lists subcommands: process, serve, ask, chat

### Requirement: Process subcommand
The `process` subcommand SHALL accept a file path or URL and optional `--json` flag.

#### Scenario: Process with file
- **WHEN** `media2rag process ./file.md` is executed
- **THEN** it parses the path and prepares for processing

#### Scenario: Process with JSON flag
- **WHEN** `media2rag process ./file.md --json` is executed
- **THEN** StdoutEmitter is used for event output

### Requirement: Serve subcommand
The `serve` subcommand SHALL accept `--host` and `--port` flags.

#### Scenario: Serve with defaults
- **WHEN** `media2rag serve` is executed
- **THEN** it starts on localhost:8542 (config defaults)

### Requirement: Ask subcommand
The `ask` subcommand SHALL accept a question string and optional `--json` flag.

#### Scenario: Ask question
- **WHEN** `media2rag ask "what is RAG?"` is executed
- **THEN** it parses the question and prepares for RAG query

### Requirement: Chat subcommand
The `chat` subcommand SHALL start interactive terminal chat.

#### Scenario: Chat starts
- **WHEN** `media2rag chat` is executed
- **THEN** it enters interactive mode

### Requirement: Global flags
The CLI SHALL support global flags: `--config` (config file path), `--verbose` (debug logging).

#### Scenario: Custom config path
- **WHEN** `media2rag --config ./my-config.yaml process ./file.md`
- **THEN** configuration is loaded from specified path
