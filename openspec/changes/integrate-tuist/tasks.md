## 1. Tuist Setup

- [x] 1.1 Install Tuist and verify version in `media2rag-gui` directory
- [x] 1.2 Create `Tuist/` directory with version pinning configuration
- [x] 1.3 Add Tuist-generated files to `.gitignore` (`.xcworkspace`, `.xcodeproj`, `Derived/`)

## 2. Project Manifest

- [x] 2.1 Create `Project.swift` with macOS application target configuration
- [x] 2.2 Configure deployment target to macOS 14.0
- [x] 2.3 Set up source directory to `media2rag` with dynamic file discovery
- [x] 2.4 Preserve existing build settings (CLI_PATH environment variable, signing)
- [x] 2.5 Configure target dependencies and frameworks

## 3. Project Generation & Validation

- [x] 3.1 Run `tuist generate` and verify workspace creation
- [x] 3.2 Open generated workspace in Xcode and verify project structure
- [x] 3.3 Verify source files are grouped correctly (Models, Services, Views)
- [ ] 3.4 Build the project and verify successful compilation
- [x] 3.5 Compare build settings with original project to ensure parity

## 4. Documentation & Cleanup

- [x] 4.1 Update README.md with Tuist setup and build instructions
- [x] 4.2 Remove references to `project.json` from documentation
- [x] 4.3 Add setup script or instructions for new developers
- [ ] 4.4 Remove `project.json` file after successful migration verification
- [ ] 4.5 Test full workflow: clone → install Tuist → generate → build → run

## 5. Migration Completion

- [ ] 5.1 Commit `Project.swift`, `Tuist/` config, updated `.gitignore`, and README
- [ ] 5.2 Remove old `media2rag.xcodeproj` from version control (keep in working directory until verified)
- [ ] 5.3 Verify team can build using new workflow
- [ ] 5.4 Archive the change after all tasks complete
