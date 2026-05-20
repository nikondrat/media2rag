from dataclasses import dataclass


@dataclass
class Chunk:
    index: int
    total: int
    text: str


class SemanticChunker:
    """Split text into chunks at semantic boundaries with overlap."""

    TARGET_SIZE = 8000
    OVERLAP = 800

    _HEADING_PATTERNS = (
        "\n# ",
        "\n## ",
        "\n### ",
        "\n#### ",
        "\n##### ",
        "\n###### ",
    )

    def split(self, text: str) -> list[Chunk]:
        if len(text) <= self.TARGET_SIZE:
            return [Chunk(index=0, total=1, text=text)]

        boundaries = self._find_boundaries(text)
        raw_chunks = self._split_at_boundaries(text, boundaries)
        chunks_with_overlap = self._add_overlap(text, raw_chunks)

        total = len(chunks_with_overlap)
        return [
            Chunk(index=i, total=total, text=chunk_text)
            for i, chunk_text in enumerate(chunks_with_overlap)
        ]

    def _find_boundaries(self, text: str) -> list[int]:
        positions = []
        for pattern in self._HEADING_PATTERNS:
            start = 0
            while True:
                idx = text.find(pattern, start)
                if idx == -1:
                    break
                positions.append(idx + 1)
                start = idx + len(pattern)

        if not positions:
            positions = self._fallback_boundaries(text)

        positions.sort()
        return positions

    def _fallback_boundaries(self, text: str) -> list[int]:
        positions = []
        start = 0
        while True:
            idx = text.find("\n\n", start)
            if idx == -1:
                break
            positions.append(idx + 2)
            start = idx + 2

        if not positions:
            positions = list(range(0, len(text), self.TARGET_SIZE))

        return positions

    def _split_at_boundaries(self, text: str, boundaries: list[int]) -> list[tuple[int, int]]:
        chunks = []
        chunk_start = 0

        for boundary in boundaries:
            if boundary - chunk_start >= self.TARGET_SIZE:
                if chunk_start > 0:
                    chunks.append((chunk_start, boundary))
                chunk_start = boundary

        if chunk_start < len(text):
            chunks.append((chunk_start, len(text)))

        if not chunks and boundaries:
            for i in range(0, len(text), self.TARGET_SIZE):
                end = min(i + self.TARGET_SIZE, len(text))
                chunks.append((i, end))

        return chunks

    def _add_overlap(self, text: str, raw_chunks: list[tuple[int, int]]) -> list[str]:
        result = []
        for i, (start, end) in enumerate(raw_chunks):
            overlap_start = max(0, start - self.OVERLAP) if i > 0 else 0
            chunk_text = text[overlap_start:end]
            result.append(chunk_text)
        return result
