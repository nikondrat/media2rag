## ADDED Requirements

### Requirement: Tuist project manifest
The project SHALL define a `Project.swift` manifest at the project root that declares the media2rag target, its source files, and build settings.

#### Scenario: Manifest declares macOS application target
- **WHEN** the manifest is parsed by Tuist
- **THEN** it declares a single macOS application target named `media2rag` with deployment target macOS 14.0

#### Scenario: Manifest specifies source directory
- **WHEN** the manifest is parsed by Tuist
- **THEN** it sets the target's sources to the `media2rag` directory

### Requirement: Dynamic source file discovery
Tuist SHALL automatically include all Swift files under the `media2rag` directory without requiring explicit file enumeration.

#### Scenario: New Swift file is included automatically
- **WHEN** a developer adds a new `.swift` file anywhere under the `media2rag` directory
- **THEN** the file is included in the build target on the next `tuist generate` without editing any manifest

#### Scenario: Removed Swift file is excluded automatically
- **WHEN** a developer deletes a `.swift` file from the `media2rag` directory
- **THEN** the file is excluded from the build target on the next `tuist generate` without editing any manifest

### Requirement: Directory structure preservation
The generated project SHALL preserve the existing logical grouping of source files: Models, Services, and Views.

#### Scenario: Source groups match directory structure
- **WHEN** `tuist generate` creates the project
- **THEN** Xcode displays source files grouped by their subdirectories (Models, Services, Views)

### Requirement: Generated files are gitignored
The `.xcworkspace`, `.xcodeproj`, and any other Tuist-generated files SHALL be excluded from version control.

#### Scenario: Generated workspace is not tracked
- **WHEN** a developer runs `git status` after `tuist generate`
- **THEN** no generated workspace or project files appear as untracked or modified

### Requirement: Tuist version pinning
The project SHALL pin a specific Tuist version to ensure reproducible project generation across machines.

#### Scenario: Version is pinned in configuration
- **WHEN** a developer clones the repository
- **THEN** the pinned Tuist version is specified in a `Tuist/` configuration directory or `.tool-versions` file
