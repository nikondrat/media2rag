class Compressor:
    """Remove noise, filler words, and extract core essence from raw content."""

    SYSTEM_PROMPT = (
        "You are a knowledge extraction specialist. Your task is to compress content "
        "by removing filler words, repetitions, off-topic tangents, and conversational noise. "
        "Preserve ALL key facts, principles, frameworks, examples, and actionable advice. "
        "Output should be 40-60% shorter but retain 100% of valuable information. "
        "Keep the original language (English or Russian)."
    )

    def __init__(self, llm_client):
        self._client = llm_client

    def compress(self, raw_text: str, max_input_tokens: int = 8000) -> str:
        if len(raw_text) < 500:
            return raw_text

        chunks = self._split_into_chunks(raw_text, max_input_tokens)
        if len(chunks) == 1:
            return self._client.chat(
                prompt=f"Compress this content while preserving all valuable information:\n\n{chunks[0]}",
                system=self.SYSTEM_PROMPT,
            )

        compressed_chunks = []
        for chunk in chunks:
            result = self._client.chat(
                prompt=f"Compress this content while preserving all valuable information:\n\n{chunk}",
                system=self.SYSTEM_PROMPT,
            )
            compressed_chunks.append(result)

        return "\n\n---\n\n".join(compressed_chunks)

    def _split_into_chunks(self, text: str, max_chars: int) -> list[str]:
        chars_per_token = 4
        max_chars_limit = max_input_tokens * chars_per_token
        if len(text) <= max_chars_limit:
            return [text]

        chunks = []
        current_chunk = []
        current_length = 0

        for paragraph in text.split("\n\n"):
            para_len = len(paragraph)
            if current_length + para_len > max_chars_limit and current_chunk:
                chunks.append("\n\n".join(current_chunk))
                current_chunk = [paragraph]
                current_length = para_len
            else:
                current_chunk.append(paragraph)
                current_length += para_len

        if current_chunk:
            chunks.append("\n\n".join(current_chunk))

        return chunks
