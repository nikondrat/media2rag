from abc import ABC, abstractmethod
from pathlib import Path

from domain.document import ExtractedContent


class BaseExtractor(ABC):
    @abstractmethod
    def extract(self, source: Path | str) -> ExtractedContent:
        """Extract content from file path or URL."""
        ...

    @abstractmethod
    def supports(self, source: Path | str) -> bool:
        """Check if this extractor can handle the given source."""
        ...
