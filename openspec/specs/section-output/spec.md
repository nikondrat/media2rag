# section-output Specification

## Purpose
TBD - created by archiving change storage-revamp. Update Purpose after archive.
## Requirements
### Requirement: Section files saved separately
The system SHALL save each merged section as an individual file in the `sections/` subdirectory of the workspace. Section filenames SHALL be derived from section names with unsafe characters replaced by underscores.

#### Scenario: Multiple sections saved
- **WHEN** a document has sections "Thesis", "Mechanism", "Key Terms"
- **THEN** files `sections/Thesis.md`, `sections/Mechanism.md`, `sections/Key_Terms.md` are created

#### Scenario: Section with special characters
- **WHEN** a section name is "How it works (deep dive)"
- **THEN** the file is saved as `sections/How_it_works__deep_dive_.md`

### Requirement: Final document assembled from sections
The system SHALL assemble the final RAG document by concatenating all section files from `sections/` in their original order.

#### Scenario: Final output includes all sections
- **WHEN** sections/ contains Thesis.md, Mechanism.md, Takeaways.md
- **THEN** `output/final.md` contains all three sections concatenated with proper headings

### Requirement: Section output in CLI events
The system SHALL emit a JSON event listing all saved section names when section output is complete.

#### Scenario: Sections event emitted
- **WHEN** all sections are saved to disk
- **THEN** a JSON event with `status: "sections_saved"` and `sections: ["Thesis", "Mechanism", ...]` is emitted

