## ADDED Requirements

### Requirement: ImageAnalyzer class processes extracted images
The system SHALL provide an `ImageAnalyzer` class that accepts image file paths and returns structured descriptions using a vision-capable LLM.

#### Scenario: ImageAnalyzer initializes with LLM client
- **WHEN** `ImageAnalyzer` is instantiated with an LLM client
- **THEN** it stores the client and is ready to analyze images

#### Scenario: ImageAnalyzer returns description for a valid image
- **WHEN** `ImageAnalyzer.analyze(path)` is called with a valid image file
- **THEN** it returns a dict with `description`, `type`, and `confidence` fields

#### Scenario: ImageAnalyzer handles missing files gracefully
- **WHEN** `ImageAnalyzer.analyze(path)` is called with a non-existent file
- **THEN** it returns `None` without raising an exception

### Requirement: Image type classification
The ImageAnalyzer SHALL classify each image into one of: `diagram`, `chart`, `photo`, `text`, `icon`, `decorative` before deciding whether to analyze it.

#### Scenario: Diagram is identified for analysis
- **WHEN** an image contains line graphs, flowcharts, or schematic drawings
- **THEN** it is classified as `diagram` and queued for full analysis

#### Scenario: Chart is identified for analysis
- **WHEN** an image contains bar charts, pie charts, or data visualizations
- **THEN** it is classified as `chart` and queued for full analysis

#### Scenario: Icon is skipped
- **WHEN** an image is smaller than 100x100 pixels
- **THEN** it is classified as `icon` and skipped from analysis

#### Scenario: Decorative element is skipped
- **WHEN** an image has aspect ratio > 10:1 or < 1:10
- **THEN** it is classified as `decorative` and skipped from analysis

### Requirement: Vision LLM prompt for diagram description
When analyzing a diagram or chart, the system SHALL use a prompt that requests: (1) title/caption if present, (2) axes labels and scales, (3) key data points or trends, (4) annotations and labels, (5) overall message of the visualization.

#### Scenario: Wyckoff distribution diagram is described
- **WHEN** a Wyckoff distribution schematic with phases A-E, BC, AR, ST, UTAD, LPSY labels is analyzed
- **THEN** the description includes phase labels, key events (BC, AR, ST), support/resistance lines, and volume bars

#### Scenario: Description is in source document language
- **WHEN** the source document is in Russian
- **THEN** the image description is generated in Russian

### Requirement: Image descriptions are embedded in extracted text
Image descriptions SHALL be inserted into `ExtractedContent.raw_text` as markdown adjacent to the `![image]()` reference, in the format: `>  [Diagram description]` followed by the image reference.

#### Scenario: Description inserted before image reference
- **WHEN** an image at page 5 has been analyzed
- **THEN** the raw_text contains `> 📊 Описание диаграммы...\n\n![image](path)` at the corresponding position

#### Scenario: Unanalyzed images retain original reference
- **WHEN** an image is classified as `icon` or `decorative` and skipped
- **THEN** the original `![image](path)` reference is preserved without description

### Requirement: Image metadata is attached to ExtractedContent
The `ExtractedContent.images` list SHALL include `description`, `type`, and `analyzed` fields for each image that was processed.

#### Scenario: Analyzed image has full metadata
- **WHEN** an image is successfully analyzed
- **THEN** its entry in `images` includes `path`, `page`, `type`, `description`, and `analyzed: true`

#### Scenario: Skipped image has minimal metadata
- **WHEN** an image is skipped (icon/decorative)
- **THEN** its entry includes `path`, `page`, `type`, and `analyzed: false`

### Requirement: Vision backend fallback
If the configured LLM backend does not support vision (e.g., text-only Ollama model), the system SHALL skip image analysis and log a warning, preserving original image references.

#### Scenario: Text-only model skips analysis
- **WHEN** the LLM client reports no vision capability
- **THEN** all images are marked as `analyzed: false` with a warning logged

#### Scenario: OpenRouter vision model is used when available
- **WHEN** OpenRouter backend is configured with a vision-capable model
- **THEN** images are analyzed through OpenRouter's vision API
