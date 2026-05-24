import json
import re


class Compressor:
    """Clean up raw transcripts: remove timestamps, filler words, noise."""

    SYSTEM_PROMPT = (
        "You are a transcript cleaning specialist. Clean up raw transcripts while preserving all valuable content.\n\n"
        "REMOVE:\n"
        "- Timestamps: [0:00], [1:30], [2:56:34], (00:00), etc.\n"
        "- Filler words: um, uh, like, you know, basically, literally, etc.\n"
        "- Stutters and repetitions\n"
        "- Promotional content and CTAs (subscribe, follow, check link, etc.)\n"
        "- Self-promotion and links to other content\n"
        "- Sponsor reads and ad segments\n"
        "- Off-topic small talk and tangents\n\n"
        "PRESERVE:\n"
        "- ALL facts, numbers, dates, statistics\n"
        "- Examples, case studies, stories\n"
        "- Frameworks, models, processes\n"
        "- Quotes and verbatim statements\n"
        "- Actionable advice and recommendations\n"
        "- Speaker attribution and labels (e.g., 'Михаил Гребенюк:', 'Жанна:')\n"
        "- The original language and tone\n\n"
        "SPEAKER ATTRIBUTION RULES:\n"
        "- Keep speaker names/labels at the start of their statements\n"
        "- Remove conversational filler between speaker exchanges\n"
        "- Preserve the dialogue structure in interviews\n\n"
        "FEW-SHOT EXAMPLES:\n\n"
        "BEFORE:\n"
        '[0:00] Привет, добро пожаловать на наш подкаст. Um, как дела?\n[0:15] Да, всё отлично, спасибо! Перед началом — подпишитесь на наш канал.\n[0:30] Михаил: Итак, главный инсайт — 80% успеха это дисциплина.\n\n'
        "AFTER:\n"
        "Михаил: Главный инсайт — 80% успеха это дисциплина.\n\n"
        "BEFORE:\n"
        '[1:00] Жанна: Расскажите про ваш подход к инвестициям. Um, что вы думаете про...\n[1:15] Иван: Ну, знаете, я считаю что диверсификация — это ключ. Как бы, 60% акции, 40% облигации.\n\n'
        "AFTER:\n"
        "Жанна: Расскажите про ваш подход к инвестициям.\n"
        "Иван: Диверсификация — это ключ. 60% акции, 40% облигации.\n\n"
        "Output ONLY the cleaned text — no preamble, no commentary."
    )

    def __init__(self, llm_client, json_mode: bool = False, reasoning: bool = False):
        self._client = llm_client
        self._json_mode = json_mode
        self._reasoning = reasoning

    def _emit(self, status: str, **kwargs):
        if self._json_mode:
            obj = {"status": status, **kwargs}
            print(json.dumps(obj, ensure_ascii=False), flush=True)
        else:
            messages = {
                "cleaning_part": f"  🧹 Cleaning transcript part {kwargs.get('current')}/{kwargs.get('total')}",
                "cleaning_part_done": f"  ✅ Part {kwargs.get('current')}/{kwargs.get('total')} cleaned",
            }
            msg = messages.get(status, f"  [{status}] {kwargs}")
            print(msg, flush=True)

    def _chat_with_streaming(self, prompt: str, system: str = "") -> str:
        """Call LLM with streaming and emit tokens as events."""
        result_parts = []
        token_count = 0
        batch_size = 20

        try:
            for token in self._client.chat_stream(prompt=prompt, system=system, reasoning=self._reasoning):
                result_parts.append(token)
                token_count += 1
                if token_count % batch_size == 0:
                    self._emit("llm_token", tokens="".join(result_parts[-batch_size:]))
        except Exception:
            if not result_parts:
                result_parts.append(self._client.chat(prompt=prompt, system=system, reasoning=self._reasoning))

        return "".join(result_parts)

    def compress(self, raw_text: str, max_input_tokens: int = 8000) -> str:
        if len(raw_text) < 500:
            return raw_text

        chunks = self._split_into_chunks(raw_text, max_input_tokens)
        if len(chunks) == 1:
            self._emit("cleaning_part", current=1, total=1)
            result = self._chat_with_streaming(
                prompt=f"Clean up this raw transcript:\n\n{chunks[0]}",
                system=self.SYSTEM_PROMPT,
            )
            self._emit("cleaning_part_done", current=1, total=1)
            return result

        compressed_chunks = []
        total = len(chunks)
        for i, chunk in enumerate(chunks, 1):
            self._emit("cleaning_part", current=i, total=total)
            result = self._chat_with_streaming(
                prompt=f"Clean up this raw transcript:\n\n{chunk}",
                system=self.SYSTEM_PROMPT,
            )
            compressed_chunks.append(result)
            self._emit("cleaning_part_done", current=i, total=total)

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
