## ADDED Requirements

### Requirement: Image extraction and storage
The system SHALL extract images from supported file types (PDF, EPUB) and save them to the `images/` subdirectory of the workspace. Images SHALL be named sequentially: `img_001.png`, `img_002.png`, etc.

#### Scenario: EPUB with embedded images
- **WHEN** an EPUB file contains 3 embedded images
- **THEN** files `images/img_001.png`, `images/img_002.png`, `images/img_003.png` are created

#### Scenario: PDF with images
- **WHEN** a PDF contains embedded images
- **THEN** each image is extracted and saved to `images/` directory

#### Scenario: No images in source
- **WHEN** the source file has no images
- **THEN** the `images/` directory is empty or not created

### Requirement: Image paths in ExtractedContent
The `ExtractedContent` domain model SHALL include an `image_paths: list[Path]` field populated by extractors that support image extraction.

#### Scenario: Extractor populates image paths
- **WHEN** PdfEpubExtractor processes a file with images
- **THEN** `ExtractedContent.image_paths` contains paths to saved image files
