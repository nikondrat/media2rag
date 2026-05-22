import json
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

    def __init__(self, llm_client, json_mode: bool = False):
        self._client = llm_client
        self._json_mode = json_mode

    def _emit(self, status: str, **kwargs):
        if self._json_mode:
            obj = {"status": status, **kwargs}
            print(json.dumps(obj, ensure_ascii=False), flush=True)

    def _chat_with_streaming(self, prompt: str, system: str = "") -> str:
        """Call LLM with streaming and emit tokens as events."""
        result_parts = []
        token_count = 0
        batch_size = 20

        try:
            for token in self._client.chat_stream(prompt=prompt, system=system):
                result_parts.append(token)
                token_count += 1
                if token_count % batch_size == 0:
                    self._emit("llm_token", tokens="".join(result_parts[-batch_size:]))
        except Exception:
            if not result_parts:
                result_parts.append(self._client.chat(prompt=prompt, system=system))

        return "".join(result_parts)

    def compress(self, raw_text: str, max_input_tokens: int = 8000) -> str:
        if len(raw_text) < 500:
            return raw_text

        chunks = self._split_into_chunks(raw_text, max_input_tokens)
        if len(chunks) == 1:
            self._emit("compressing_chunk", current=1, total=1)
            result = self._chat_with_streaming(
                prompt=f"Clean up this raw transcript:\n\n{chunks[0]}",
                system=self.SYSTEM_PROMPT,
            )
            self._emit("compressed_chunk", current=1, total=1)
            return result

        compressed_chunks = []
        total = len(chunks)
        for i, chunk in enumerate(chunks, 1):
            self._emit("compressing_chunk", current=i, total=total)
            result = self._chat_with_streaming(
                prompt=f"Clean up this raw transcript:\n\n{chunk}",
                system=self.SYSTEM_PROMPT,
            )
            compressed_chunks.append(result)
            self._emit("compressed_chunk", current=i, total=total)

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
