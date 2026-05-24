import re
from dataclasses import dataclass, field

from domain.document import ExtractedContent


@dataclass
class Section:
    heading: str
    level: int
    content: str


@dataclass
class NormalizedDocument:
    text: str
    sections: list[Section] = field(default_factory=list)
    tables: list[str] = field(default_factory=list)
    code_blocks: list[str] = field(default_factory=list)
    metadata: dict = field(default_factory=dict)


class Normalizer:
    _META_LINE_RE = re.compile(r"^\*\*(?:Author|Source|Date|Type|Language):\*\*.*$", re.MULTILINE)
    _HEADING_RE = re.compile(r"^(#{1,6})\s+(.+)$", re.MULTILINE)
    _TABLE_RE = re.compile(r"(\|.+\|\n\|[-| :]+\|\n(?:\|.+\|\n?)+)", re.MULTILINE)
    _CODE_BLOCK_RE = re.compile(r"(```[\s\S]*?```)", re.MULTILINE)

    def normalize(self, extracted: ExtractedContent) -> NormalizedDocument:
        doc_type = extracted.metadata.doc_type
        text = extracted.raw_text

        text = self._strip_metadata_lines(text)

        if doc_type in ("video", "audio"):
            text = self._clean_transcript(text)
        elif doc_type == "telegram":
            text = self._clean_telegram(text)

        tables = self._extract_tables(text)
        code_blocks = self._extract_code_blocks(text)
        sections = self._extract_sections(text)

        return NormalizedDocument(
            text=text,
            sections=sections,
            tables=tables,
            code_blocks=code_blocks,
            metadata={
                "title": extracted.metadata.title,
                "source": extracted.metadata.source,
                "doc_type": doc_type,
                "author": extracted.metadata.author,
                "language": extracted.metadata.language,
                "word_count": extracted.metadata.word_count,
            },
        )

    def _strip_metadata_lines(self, text: str) -> str:
        text = self._META_LINE_RE.sub("", text)
        text = re.sub(r"^---\s*$", "", text, flags=re.MULTILINE)
        return re.sub(r'\n{3,}', '\n\n', text).strip()

    def _clean_transcript(self, text: str) -> str:
        text = re.sub(r'\[?\d{1,2}:\d{2}(?::\d{2})?\]?\s*', '', text)
        text = re.sub(r'\n{3,}', '\n\n', text)
        return text.strip()

    def _clean_telegram(self, text: str) -> str:
        text = re.sub(r'@\w+', '', text)
        text = re.sub(r'https?://\S+', '', text)
        return text.strip()

    def _extract_tables(self, text: str) -> list[str]:
        return [m.group(0).strip() for m in self._TABLE_RE.finditer(text)]

    def _extract_code_blocks(self, text: str) -> list[str]:
        return [m.group(0).strip() for m in self._CODE_BLOCK_RE.finditer(text)]

    def _extract_sections(self, text: str) -> list[Section]:
        sections = []
        lines = text.split("\n")
        current_heading = ""
        current_level = 0
        current_lines: list[str] = []

        for line in lines:
            m = re.match(r"^(#{1,6})\s+(.+)$", line)
            if m:
                if current_lines:
                    content = "\n".join(current_lines).strip()
                    if content:
                        sections.append(Section(
                            heading=current_heading,
                            level=current_level,
                            content=content,
                        ))
                current_level = len(m.group(1))
                current_heading = m.group(2).strip()
                current_lines = []
            else:
                current_lines.append(line)

        if current_lines:
            content = "\n".join(current_lines).strip()
            if content:
                sections.append(Section(
                    heading=current_heading,
                    level=current_level,
                    content=content,
                ))

        return sections
