import re


class Compressor:
    """Clean up raw transcripts: remove timestamps, filler words, noise."""

    SYSTEM_PROMPT = (
        "Clean up this raw transcript. Remove:\n"
        "- Timestamps like [0:00], [1:30], [2:56:34]\n"
        "- Filler words and stutters\n"
        "- Promotional content and CTAs\n\n"
        "Preserve ALL facts, numbers, examples, frameworks, quotes, and actionable advice.\n"
        "Keep the original language. Output ONLY the cleaned text."
    )

    def __init__(self, llm_client):
        self._client = llm_client

    def compress(self, raw_text: str, max_input_tokens: int = 8000) -> str:
        if len(raw_text) < 500:
            return raw_text

        chunks = self._split_into_chunks(raw_text, max_input_tokens)
        if len(chunks) == 1:
            return self._client.chat(
                prompt=f"Clean up this raw transcript:\n\n{chunks[0]}",
                system=self.SYSTEM_PROMPT,
            )

        compressed_chunks = []
        total = len(chunks)
        for i, chunk in enumerate(chunks, 1):
            print(f"    Cleaning chunk {i}/{total}...")
            result = self._client.chat(
                prompt=f"Clean up this raw transcript:\n\n{chunk}",
                system=self.SYSTEM_PROMPT,
            )
            compressed_chunks.append(result)
            print(f"    Chunk {i}/{total} done")

        return "\n\n".join(compressed_chunks)

    def _split_into_chunks(self, text: str, max_chars: int) -> list[str]:
        chars_per_token = 4
        max_chars_limit = max_chars * chars_per_token
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

    @staticmethod
    def clean_artifacts(text: str) -> str:
        text = re.sub(r'\n{3,}', '\n\n', text)
        text = re.sub(r'\[?\d{1,2}:\d{2}(:\d{2})?\]?\s*', '', text)
        return text.strip()
