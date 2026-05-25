## ADDED Requirements

### Requirement: Documents list
The `documents list` command SHALL display all documents in workspace with hash, title, source, and version info.

#### Scenario: List all documents
- **WHEN** `media2rag documents list` is executed
- **THEN** a table is printed with columns: hash, title, source, versions, updated_at

#### Scenario: Empty workspace
- **WHEN** `media2rag documents list` is executed with no documents
- **THEN** message "No documents in workspace" is shown

### Requirement: Documents show
The `documents show <hash>` command SHALL display document details including metadata and version list.

#### Scenario: Show document details
- **WHEN** `media2rag documents show abc12345` is executed
- **THEN** metadata from `.media2rag.yaml` is displayed

#### Scenario: Show with versions flag
- **WHEN** `media2rag documents show abc12345 --versions` is executed
- **THEN** list of versions with dates is displayed

#### Scenario: Show specific version
- **WHEN** `media2rag documents show abc12345 --version 1` is executed
- **THEN** content of `versions/v1/final.md` is printed

### Requirement: Documents delete
The `documents delete <hash>` command SHALL remove a document from workspace.

#### Scenario: Delete document
- **WHEN** `media2rag documents delete abc12345` is executed
- **THEN** the document directory is removed

#### Scenario: Delete with confirmation
- **WHEN** `media2rag documents delete abc12345` is executed without `--force`
- **THEN** user is prompted for confirmation

#### Scenario: Delete with force flag
- **WHEN** `media2rag documents delete abc12345 --force` is executed
- **THEN** document is deleted without prompt
