# cli-json-events Specification

## Purpose
TBD - created by archiving change storage-revamp. Update Purpose after archive.
## Requirements
### Requirement: Workspace path in CLI args
The CLI SHALL accept `--workspace` argument to specify the workspace root directory. This replaces the previous `-o` / `--output` behavior.

#### Scenario: Workspace flag used
- **WHEN** user runs `cli.py file.pdf --workspace /my/docs`
- **THEN** processing uses `/my/docs` as workspace root

### Requirement: Backward compatible -o flag
The CLI SHALL support `-o` and `--output` as aliases for `--workspace` for backward compatibility.

#### Scenario: Legacy -o flag
- **WHEN** user runs `cli.py file.pdf -o /my/docs`
- **THEN** processing uses `/my/docs` as workspace root (same as `--workspace`)

