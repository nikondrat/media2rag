## Why

EPUB extraction produces output with extra blank lines, broken book title references (inline `<em>`/`<i>` tags split into separate paragraphs), and missing images from the book. PDF extractor uses PyMuPDF but lacks OCR fallback. Audio/video extractors were fixed to use Python whisper API instead of CLI, but the overall extraction quality for books needs improvement to match the clean output of video/audio transcripts.

## What Changes

- **EPUB image extraction**: Extract embedded images from EPUB archive and insert markdown image references at appropriate positions in the text
- **EPUB text cleanup**: Fix paragraph splitting caused by inline formatting tags (`<em>`, `<i>`, `<span>`) that create artificial line breaks around book titles and quoted text
- **EPUB subheading detection**: Convert ALL-CAPS section headers within paragraphs into proper markdown `##` headings
- **PDF OCR fallback**: Add Tesseract OCR fallback when PyMuPDF text extraction returns empty or insufficient text
- **Image extractor enhancement**: Support image extraction from PDF pages (diagrams, charts) and insert into output markdown

## Capabilities

### New Capabilities
- `epub-images`: Extract and embed images from EPUB files into output markdown
- `epub-text-quality`: Improved paragraph reconstruction and subheading detection for EPUB extraction
- `pdf-ocr`: Tesseract OCR fallback for PDF files with image-based content
- `pdf-images`: Extract diagrams/charts from PDF pages and embed in output

### Modified Capabilities
<!-- No existing specs to modify -->

## Impact

- `extractors/pdf_epub_extractor.py` — EPUB text extraction logic, new image extraction, PDF OCR fallback
- `extractors/image_extractor.py` — Enhanced to support PDF page image extraction
- `domain/document.py` — May need `images` field in ExtractedContent (already exists but unused)
- `pyproject.toml` — Add `tesseract-ocr` dependency (via `pytesseract`), `Pillow` already present
- Requires `tesseract` binary installed (`brew install tesseract`)
