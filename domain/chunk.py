from dataclasses import dataclass, field


@dataclass
class Chunk:
    id: str
    document_id: str
    parent_id: str
    content: str
    section: str = ""
    chunk_index: int = 0
    content_type: str = "text"
    metadata: dict = field(default_factory=dict)


@dataclass
class ParentChunk:
    id: str
    document_id: str
    content: str
    section: str = ""
    metadata: dict = field(default_factory=dict)
