from typing import Generator, Protocol, runtime_checkable


@runtime_checkable
class LLMClient(Protocol):
    def chat(self, prompt: str, system: str = "", model: str = "", stream: bool = False, reasoning: bool = False) -> str:
        ...

    def chat_stream(self, prompt: str, system: str = "", model: str = "", reasoning: bool = False) -> Generator[str, None, None]:
        ...

    def chat_with_image(self, prompt: str, image_b64: str, system: str = "", model: str = "") -> str:
        ...

    def is_available(self) -> bool:
        ...
