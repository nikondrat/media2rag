# Skill System — Design

## What is a Skill?

A skill is a **packaged domain configuration** that tells media2rag how to process, analyze, and query content for a specific use case.

Unlike Claude Code skills (which require MCP, tool definitions, agent configuration), media2rag skills are **YAML files + prompts** — simple, readable, editable by anyone.

---

## Structure

```
skills/<skill-name>/
├── skill.yaml          # Metadata and entry point
├── prompts/
│   ├── extract.yaml    # Extraction prompts
│   ├── analyze.yaml    # Analysis prompts
│   └── coach.yaml      # Coaching prompts (if applicable)
├── pipeline.yaml       # Pipeline configuration
├── memory.yaml         # Memory schema
├── rag.yaml            # RAG search configuration
└── assets/             # Optional: pre-processed knowledge base
    └── ...
```

---

## skill.yaml

```yaml
name: sales-call-analysis
version: 1.0.0
description: Analyze sales call recordings for script adherence, objection handling, and closing techniques
author: Your Name
tags: [sales, calls, analysis]

# What this skill does
capabilities:
  - extract: Process audio/video transcripts
  - analyze: Score call quality, identify strengths/weaknesses
  - coach: Provide actionable feedback

# Required LLM models
models:
  pipeline: qwen3.5:27b      # for extraction/analysis
  embedding: qwen3-embedding:0.6b
  reranker: bge-reranker-v2-m3
```

---

## pipeline.yaml

```yaml
# Pipeline configuration for this skill
compressor:
  chunk_size: 32000   # tokens
  prompt: prompts/extract.yaml#compress

splitter:
  chunk_size: 4000    # characters
  chunk_overlap: 200

processing:
  max_concurrency: 3
  prompt: prompts/extract.yaml#process
  extract:
    - title
    - topics
    - summary
    - claims
    - objections        # domain-specific
    - techniques_used   # domain-specific

generator:
  prompt: prompts/extract.yaml#assemble
  frontmatter_fields:
    - title
    - source
    - type
    - topics
    - call_score        # domain-specific
    - objections_handled
    - techniques_used

quality_check:
  enabled: true
  prompt: prompts/extract.yaml#quality
  max_retries: 2
  criteria:
    - completeness
    - accuracy
    - actionable_insights
```

---

## memory.yaml

```yaml
# What facts to extract and store during conversations
extraction:
  prompt: prompts/coach.yaml#memory_extract
  categories:
    - fact: "Factual information about the user or business"
    - goal: "User's objectives and targets"
    - weakness: "Identified areas for improvement"
    - strength: "Identified strengths"
    - pattern: "Recurring behaviors or themes"

# How to use memory in context
recall:
  top_k: 5
  categories: [fact, goal, weakness, strength, pattern]
```

---

## rag.yaml

```yaml
# RAG search configuration for this skill
query_rewrite:
  enabled: true
  prompt: prompts/analyze.yaml#rewrite

hybrid_search:
  enabled: true
  dense_weight: 0.5
  sparse_weight: 0.5
  rrf_k: 60

reranker:
  enabled: true
  model: bge-reranker-v2-m3
  top_k: 5
  search_factor: 2

context:
  max_chunks: 5
  include_sources: true
  format: prompts/analyze.yaml#context_format
```

---

## Prompts

Prompts are stored in separate YAML files for easy editing:

```yaml
# prompts/extract.yaml

compress: |
  Clean this sales call transcript by removing:
  - Timestamps and timecodes
  - Small talk and greetings (keep only if relevant to rapport building)
  - Technical issues ("can you hear me?", "you're on mute")
  Keep all sales-related content: objections, techniques, closing attempts.
  Return only the cleaned text.

  Text:
  {input}

process: |
  Analyze this sales call chunk and return:

  title: <short descriptive title>
  topics: <2-3 key topics, comma separated>
  summary: <1-2 sentence summary>
  objections: <list any customer objections mentioned, or "none">
  techniques_used: <sales techniques identified, or "none">

  Text:
  {chunk}

quality: |
  Evaluate this processed sales call analysis:
  1. Does it capture all key moments? (completeness)
  2. Are objections and techniques accurately identified? (accuracy)
  3. Are the insights actionable? (actionability)

  Score 1-5 for each criterion. Pass if all >= 3.

  Content:
  {content}
```

---

## Skill Loading

```go
type Skill struct {
    Name        string
    Version     string
    Description string
    Prompts     map[string]string    // loaded from prompts/*.yaml
    Pipeline    PipelineConfig
    Memory      MemoryConfig
    RAG         RAGConfig
}

type SkillLoader struct {
    skillsDir string
    registry  map[string]*Skill
}

func (l *SkillLoader) Load(name string) (*Skill, error)
func (l *SkillLoader) List() []*Skill
func (l *SkillLoader) Enable(name string) error
func (l *SkillLoader) Active() *Skill
```

---

## CLI

```bash
# List available skills
media2rag skills list

# Install a skill from marketplace
media2rag skills install sales-call-analysis

# Enable a skill for current session
media2rag skills enable sales-call-analysis

# Process with a specific skill
media2rag process ./call.mp4 --skill sales-call-analysis

# Chat with a skill's knowledge base
media2rag chat --skill sales-call-analysis
```

---

## Built-in Skills (Phase 2)

### 1. Sales Call Analysis
- Process call recordings
- Score script adherence
- Identify objections and handling quality
- Compare agents

### 2. Business Coach
- Idea validation
- Business model formulation
- Step-by-step planning
- Progress tracking

### 3. Legal Review
- Contract analysis
- Risk identification
- Clause extraction
- Compliance checking

---

## Marketplace Format

Skills and knowledge bases are packaged as tarballs:

```
skill-name-v1.0.0.tar.gz
├── skill.yaml
├── prompts/
├── pipeline.yaml
├── memory.yaml
├── rag.yaml
└── CHECKSUM.sha256
```

Knowledge bases (pre-processed data):

```
knowledge-base-v1.0.0.tar.gz
├── manifest.yaml
├── documents/
│   ├── doc1/
│   │   ├── source.md
│   │   └── output/final.md
│   └── ...
└── qdrant-export/    # optional: pre-indexed vectors
    └── ...
```
