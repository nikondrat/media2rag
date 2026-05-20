## Context

The media2rag-gui project currently uses a manually maintained `project.json` file that lists every Swift source file explicitly. This file is used to generate the `media2rag.xcodeproj` file. When developers add new Swift files, they must update `project.json` and regenerate the Xcode project, which causes:
- Merge conflicts when multiple developers add files simultaneously
- Forgotten file additions leading to build failures
- Manual overhead that slows development

The project is a SwiftUI macOS application (macOS 14+) with 11 Swift files organized into Models, Services, and Views directories.

## Goals / Non-Goals

**Goals:**
- Eliminate manual file tracking — Tuist dynamically discovers all Swift files
- Preserve existing project structure (groups, target, build settings)
- Maintain compatibility with current build settings (CLI_PATH, deployment target)
- Provide simple developer workflow: `tuist generate` → open workspace → build

**Non-Goals:**
- Restructuring the project or refactoring Swift code
- Adding new dependencies or frameworks
- Changing the app architecture or JSON protocol with the CLI backend
- Modifying the media2rag Python backend

## Decisions

### 1. Use Tuist's `sources` glob pattern for dynamic file discovery
**Decision:** Use `Target.sources: .glob("media2rag/**")` to automatically include all Swift files.
**Rationale:** Tuist supports glob patterns that recursively match source files, eliminating the need for explicit file lists. This matches the current directory structure (Models/, Services/, Views/) without requiring changes.
**Alternatives considered:**
- Keep `project.json` but auto-generate it — still requires a custom tool
- Use Xcode's folder references — doesn't work well with Swift package structure

### 2. Replace `.xcodeproj` with `.xcworkspace` workflow
**Decision:** Developers will run `tuist generate` which creates a `.xcworkspace` file to open in Xcode.
**Rationale:** Tuist generates a workspace that includes the project and any dependencies. This is the standard Tuist workflow and provides better IDE integration.
**Alternatives considered:**
- Generate `.xcodeproj` directly — Tuist can do this, but workspace is the recommended approach

### 3. Preserve build settings in Tuist configuration
**Decision:** All current build settings (deployment target, CLI_PATH environment variable, signing) will be migrated to `Project.swift`.
**Rationale:** The app depends on specific build settings to function correctly. These must be preserved exactly.

### 4. Add Tuist to `.gitignore` for generated files
**Decision:** Generated `.xcworkspace` and `.xcodeproj` will be gitignored; only `Project.swift` and `Config.swift` are source-controlled.
**Rationale:** Generated files should not be committed — they're reproducible from the Tuist manifest.

## Risks / Trade-offs

- **[Risk]** Tuist version compatibility → Mitigation: Pin Tuist version in `Tuist/Package.swift` and document required version in README
- **[Risk]** Build settings migration errors → Mitigation: Compare generated project settings with original before committing
- **[Risk]** Developer onboarding overhead (installing Tuist) → Mitigation: Add setup script and update README with clear instructions
- **[Trade-off]** Initial setup time vs long-term maintenance savings — one-time migration cost pays off immediately with first new file added

## Migration Plan

1. Install Tuist (`brew install tuist` or use `mise`)
2. Create `Project.swift` with current project configuration
3. Run `tuist generate` to verify project builds correctly
4. Compare build outputs with original project
5. Update README with new build instructions
6. Remove `project.json` and commit generated files to `.gitignore`
7. Add `Tuist/` directory with version pinning

## Open Questions

- Should we use `mise` or `brew` for Tuist version management? (Recommendation: `mise` for project-local version pinning)
- Do we need to preserve the `project.xcworkspace` contents or let Tuist regenerate it? (Answer: Let Tuist regenerate)
