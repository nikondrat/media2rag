## ADDED Requirements

### Requirement: Tuist generate command
Developers SHALL run `tuist generate` to create the Xcode workspace from the Tuist manifest before building the project.

#### Scenario: Successful project generation
- **WHEN** a developer runs `tuist generate` in the project root
- **THEN** a `.xcworkspace` file is created and can be opened in Xcode

#### Scenario: Generation fails without Tuist installed
- **WHEN** a developer runs `tuist generate` without Tuist installed
- **THEN** the command fails with an error message directing them to installation instructions

### Requirement: Build settings preservation
The generated project SHALL preserve all existing build settings including deployment target, environment variables, and signing configuration.

#### Scenario: CLI_PATH environment variable is set
- **WHEN** the generated project is opened in Xcode
- **THEN** the `CLI_PATH` build setting is configured with the same value as the original project

#### Scenario: Deployment target is macOS 14.0
- **WHEN** the generated project is opened in Xcode
- **THEN** the deployment target is set to macOS 14.0 (Sonoma)

### Requirement: README updated with Tuist workflow
The project README SHALL document the new Tuist-based development workflow.

#### Scenario: README includes setup instructions
- **WHEN** a developer reads the README
- **THEN** it includes steps to install Tuist, generate the project, and build

#### Scenario: README removes references to project.json
- **WHEN** a developer reads the README
- **THEN** there are no instructions referencing the removed `project.json` file

### Requirement: Backward compatibility during migration
During the migration, developers SHALL be able to use either the old `project.json` workflow or the new Tuist workflow.

#### Scenario: Both workflows produce buildable projects
- **WHEN** the migration is in progress (both `project.json` and `Project.swift` exist)
- **THEN** developers can build using either the old or new workflow
