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
            "title": metadata.title or "Untitled",
            "source": metadata.source,
            "type": metadata.doc_type,
        }
        if metadata.author:
            fm["author"] = metadata.author
        if metadata.language:
            fm["language"] = metadata.language
        if metadata.domains:
            fm["domains"] = metadata.domains
        if metadata.core_thesis:
            fm["core_thesis"] = metadata.core_thesis
        if metadata.mental_models:
            fm["mental_models"] = metadata.mental_models
        if metadata.claims:
            fm["claims"] = [
                {"text": c.text, "type": c.type, "confidence": c.confidence}
                for c in metadata.claims
            ]
        if metadata.takeaways:
            fm["takeaways"] = metadata.takeaways
        if metadata.key_terms:
            fm["key_terms"] = metadata.key_terms
        if metadata.summary:
            fm["summary"] = metadata.summary
        if metadata.key_insights:
            fm["key_insights"] = metadata.key_insights

        return "---\n" + yaml.dump(fm, allow_unicode=True, sort_keys=False).strip() + "\n---"
