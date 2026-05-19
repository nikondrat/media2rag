## 1. EPUB Text Quality Improvements

- [ ] 1.1 Fix inline tag unwrapping in `_extract_epub`: unwrap `<em>`, `<i>`, `<strong>`, `<b>`, `<span>`, `<a>` tags without creating line breaks
- [ ] 1.2 Fix punctuation spacing: remove extra spaces before `,`, `.`, `;`, `:` caused by tag unwrapping
- [ ] 1.3 Add ALL-CAPS header detection: regex for lines with 3+ uppercase words (15+ chars) → convert to `##` headings
- [ ] 1.4 Test EPUB extraction on the Random Walk book — verify no broken paragraphs, proper headings

## 2. EPUB Image Extraction

- [ ] 2.1 Add image extraction to `_extract_epub`: use `book.get_items_of_type(ebooklib.ITEM_IMAGE)` to get all images
- [ ] 2.2 Save extracted images to output directory with hash-based filenames (`img_{hash}.{ext}`)
- [ ] 2.3 Track image positions during HTML parsing: map `<img>` elements to their DOM position
- [ ] 2.4 Insert markdown image references at correct positions in extracted text
- [ ] 2.5 Populate `ExtractedContent.images` with path and position metadata
- [ ] 2.6 Test with EPUB containing images — verify images are extracted and referenced

## 3. PDF OCR Fallback

- [ ] 3.1 Add `pytesseract` to `pyproject.toml` dependencies
- [ ] 3.2 Add OCR detection in `_extract_pdf`: check if PyMuDF text < 100 chars per page
- [ ] 3.3 Implement `_extract_with_ocr` method: render page to image, run Tesseract OCR
- [ ] 3.4 Add language configuration: use `MarkerConfig.langs` for OCR language (e.g., `eng+rus`)
- [ ] 3.5 Handle missing Tesseract gracefully: log warning, return available text
- [ ] 3.6 Test OCR on a scanned PDF — verify text extraction works

## 4. PDF Image Extraction

- [ ] 4.1 Add image extraction to `_extract_pdf`: use `page.get_images()` to find embedded images
- [ ] 4.2 Extract images as PNG using `page.get_pixmap()` and save to output directory
- [ ] 4.3 Insert markdown image references at page boundaries in extracted text
- [ ] 4.4 Populate `ExtractedContent.images` with path, page number, and dimensions
- [ ] 4.5 Test PDF with diagrams/charts — verify images are extracted and referenced

## 5. Integration & Testing

- [ ] 5.1 Run full pipeline on EPUB book — verify clean output with images
- [ ] 5.2 Run full pipeline on PDF with images — verify OCR fallback and image extraction
- [ ] 5.3 Run full pipeline on audio file — verify no regression
- [ ] 5.4 Run full pipeline on video file — verify no regression
- [ ] 5.5 Update `AGENTS.md` with new capabilities and prerequisites (tesseract)
