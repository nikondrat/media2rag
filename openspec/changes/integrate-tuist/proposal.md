## Why

The media2rag-gui Xcode project requires manual file management — every new Swift file must be added to `project.json` and the `.xcodeproj` file, causing merge conflicts and slowing development. Tuist will automate this by dynamically discovering and including all source files, eliminating manual project file maintenance.

## What Changes

- Replace `media2rag.xcodeproj` with Tuist-generated project definition
- Add `Project.swift` manifest that dynamically discovers all Swift files
- Remove static `project.json` file references
- Add Tuist configuration and dependencies
- Update build scripts to use `tuist generate` before building

## Capabilities

### New Capabilities
- `tuist-project`: Tuist-based project generation with dynamic file discovery, replacing manual Xcode project management
- `tuist-build-flow`: Build and development workflows using Tuist commands instead of direct Xcode project operations

### Modified Capabilities
<!-- No existing capabilities are modified at the spec level -->

## Impact

- **Build system**: Xcode project becomes generated output, not source-controlled
- **Development workflow**: Developers run `tuist generate` instead of opening `.xcodeproj` directly
- **CI/CD**: Build scripts need Tuist installation step
- **Dependencies**: Tuist CLI becomes a development dependency
- **project.json**: Will be replaced by Tuist's dynamic file discovery
