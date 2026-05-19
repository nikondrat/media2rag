## Context

media2rag currently extracts text from EPUB and PDF files but has quality issues:

- **EPUB**: Inline formatting tags (`<em>`, `<i>`, `<span>`) cause book titles and quoted text to be split into separate paragraphs, creating excessive blank lines. Images embedded in EPUBs are ignored. ALL-CAPS section headers within paragraphs are not converted to markdown headings.
- **PDF**: PyMuPDF text extraction works for text-based PDFs but has no OCR fallback for scanned/image-based PDFs. Images/diagrams in PDFs are not extracted.
- **Audio/Video**: Already fixed to use Python whisper API (completed in previous work).

The `ExtractedContent` dataclass already has an `images` field (`list[dict]` with `path` and `description`), but no extractor populates it.

## Goals / Non-Goals

**Goals:**
- Clean EPUB text output: no artificial paragraph breaks from inline tags
- Extract and embed EPUB images into output markdown
- Detect and convert ALL-CAPS section headers to markdown headings
- Add OCR fallback for PDF files with no extractable text
- Extract images from PDF pages and embed in output

**Non-Goals:**
- Image caption generation via vision model (out of scope — manual descriptions only)
- OCR for EPUB (EPUBs are HTML-based, text is always extractable)
- Video frame extraction (video already uses audio transcription)
- Changing the CTG pipeline — improvements are at the extractor level only

## Decisions

### 1. EPUB image extraction via `ebooklib` ZIP access
**Decision**: Use `ebooklib`'s internal ZIP access to extract image files, rather than manually opening the EPUB as a ZIP.
**Rationale**: `ebooklib` already parses the EPUB structure and provides `book.get_item_with_id()` and `book.get_items_of_type(ebooklib.ITEM_IMAGE)`. This is cleaner and handles manifest/NCX correctly.
**Alternatives considered**: Manual `zipfile` access — more control but duplicates ebooklib's parsing logic.

### 2. Image placement in EPUB text
**Decision**: Insert `![image](path)` markers at the position in text where the image's parent HTML element appeared.
**Rationale**: Preserves reading context. Images are placed between paragraphs where they appeared in the original book.
**Approach**: During HTML parsing, track image elements and their position in the DOM. After text extraction, insert markdown references at the corresponding chapter position.

### 3. ALL-CAPS header detection
**Decision**: Use regex to detect ALL-CAPS phrases (3+ words, 10+ chars total) that appear as standalone lines or at the start of paragraphs, and convert them to `##` headings.
**Rationale**: Simple heuristic that catches most section headers without false positives. Avoids LLM calls for this structural task.
**Pattern**: Lines matching `^[A-Z][A-Z\s\-']{10,}$` with 2+ words.

### 4. PDF OCR fallback
**Decision**: Only invoke Tesseract when PyMuPDF returns < 100 characters of text. Use `pytesseract` Python wrapper.
**Rationale**: OCR is expensive. Most PDFs have extractable text — only scanned documents need OCR.
**Dependency**: `brew install tesseract` + `pip install pytesseract`. Graceful degradation if tesseract not installed.

### 5. PDF image extraction
**Decision**: Use PyMuPDF's `page.get_images()` and `page.get_pixmap()` to extract embedded images as PNG files.
**Rationale**: PyMuPDF already provides this capability natively. No additional dependencies needed.

## Risks / Trade-offs

| Risk | Mitigation |
|------|-----------|
| OCR is slow on CPU | Only trigger when text extraction fails; skip for large PDFs (>50 pages) |
| ALL-CAPS detection false positives | Require 3+ words AND minimum 15 chars; skip if line contains punctuation like `.` or `:` |
| EPUB images may be decorative | Extract all images; let LLM pipeline filter if needed |
| Image file naming conflicts | Use hash-based naming: `img_{hash}.png` |
| Tesseract not installed | Graceful fallback with warning message; PDF still processes without OCR |
