## ADDED Requirements

### Requirement: Extract images from EPUB files
The system SHALL extract all embedded images from EPUB files during text extraction and save them to the output directory.

#### Scenario: EPUB with images
- **WHEN** an EPUB file contains embedded images (JPEG, PNG, GIF, SVG)
- **THEN** all images are extracted and saved to the output directory with unique filenames

#### Scenario: EPUB without images
- **WHEN** an EPUB file contains no images
- **THEN** extraction proceeds normally with an empty images list

### Requirement: Embed image references in extracted text
The system SHALL insert markdown image references (`![image](path)`) at the position in the text where each image appeared in the original EPUB HTML structure.

#### Scenario: Image between paragraphs
- **WHEN** an `<img>` element appears between two `<p>` elements in the EPUB HTML
- **THEN** the markdown image reference is inserted between the corresponding text paragraphs

#### Scenario: Image within a paragraph
- **WHEN** an `<img>` element appears inside a `<p>` element
- **THEN** the markdown image reference is inserted at the corresponding position within the paragraph text

### Requirement: Track images in ExtractedContent
The system SHALL populate the `images` field of `ExtractedContent` with metadata for each extracted image, including file path and source position.

#### Scenario: Image metadata tracking
- **WHEN** images are extracted from an EPUB
- **THEN** each image entry in `ExtractedContent.images` contains `path` (relative file path) and `position` (chapter/element context)
