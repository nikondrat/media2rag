import yaml
from pathlib import Path

from domain.document import DocumentMetadata, RAGDocument


class Generator:
    """Assemble final RAG-ready markdown with frontmatter."""

    def generate(
        self,
        structured_content: str,
        metadata: DocumentMetadata,
        source_path: str = "",
    ) -> RAGDocument:
        frontmatter = self._build_frontmatter(metadata, source_path)
        markdown = f"{frontmatter}\n\n{structured_content}"
        return RAGDocument(markdown=markdown, metadata=metadata)

    def _build_frontmatter(self, metadata: DocumentMetadata, source_path: str) -> str:
        fm = {
            "title": metadata.title,
            "source": metadata.source,
            "type": metadata.doc_type,
        }
        if metadata.author:
            fm["author"] = metadata.author
        if metadata.topics:
            fm["topics"] = metadata.topics
        if metadata.summary:
            fm["summary"] = metadata.summary
        if metadata.key_insights:
            fm["key_insights"] = metadata.key_insights
        if source_path:
            fm["original_path"] = source_path

        return "---\n" + yaml.dump(fm, allow_unicode=True, sort_keys=False).strip() + "\n---"
