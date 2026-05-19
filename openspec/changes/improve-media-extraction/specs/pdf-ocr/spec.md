## ADDED Requirements

### Requirement: Detect PDFs requiring OCR
The system SHALL detect when a PDF file contains insufficient extractable text (< 100 characters) and fall back to OCR processing.

#### Scenario: Scanned PDF
- **WHEN** a PDF page contains only images with no extractable text layer
- **THEN** the system detects insufficient text and triggers OCR for that page

#### Scenario: Text-based PDF
- **WHEN** a PDF page contains extractable text (>= 100 characters)
- **THEN** OCR is NOT triggered and normal text extraction is used

### Requirement: Perform OCR with Tesseract
The system SHALL use Tesseract OCR to extract text from PDF pages when the text layer is missing or insufficient.

#### Scenario: OCR successful
- **WHEN** Tesseract is installed and available
- **THEN** text is extracted from PDF pages using OCR and returned as the page content

#### Scenario: Tesseract not installed
- **WHEN** Tesseract binary is not available on the system
- **THEN** the system logs a warning and returns whatever text was extractable without OCR

### Requirement: Configure OCR language
The system SHALL support language configuration for OCR to improve accuracy for non-English documents.

#### Scenario: Russian language OCR
- **WHEN** the PDF is in Russian and `langs` config includes `rus`
- **THEN** OCR uses the Russian language model for better accuracy

#### Scenario: Multi-language OCR
- **WHEN** the PDF may contain multiple languages
- **THEN** OCR uses all configured languages (e.g., `eng+rus`) for recognition
