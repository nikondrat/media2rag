## ADDED Requirements

### Requirement: Extract images from PDF pages
The system SHALL extract embedded images from PDF pages using PyMuPDF's image extraction capabilities.

#### Scenario: PDF with embedded images
- **WHEN** a PDF page contains embedded images (diagrams, charts, photos)
- **THEN** images are extracted as PNG files and saved to the output directory

#### Scenario: PDF without images
- **WHEN** a PDF page contains no embedded images
- **THEN** no images are extracted for that page

### Requirement: Embed image references in PDF extracted text
The system SHALL insert markdown image references (`![image](path)`) at the approximate position where each image appeared on the PDF page.

#### Scenario: Image positioned in text
- **WHEN** an image is extracted from a specific page and position
- **THEN** the markdown image reference is inserted in the text at the corresponding page boundary

### Requirement: Track images in ExtractedContent for PDFs
The system SHALL populate the `images` field of `ExtractedContent` with metadata for each extracted PDF image.

#### Scenario: PDF image metadata
- **WHEN** images are extracted from a PDF
- **THEN** each image entry contains `path` (relative file path), `page` (page number), and `dimensions` (width x height)
