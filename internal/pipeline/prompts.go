package pipeline

const cleaningPrompt = `You are a transcription cleaning assistant. Clean this transcript by:

1. Remove timestamps (e.g. "0:00", "12:34")
2. Remove non-speech markers: [Music], [Applause], [laughter], [silence]
3. Remove duplicate segments (Whisper sometimes repeats phrases)
4. Fix merged words where spaces are missing
5. Add missing punctuation at sentence boundaries
6. Remove filler words only if they clearly break flow (um, uh, ээ)

CRITICAL RULES:
- DO NOT change wording or meaning
- DO NOT translate
- DO NOT add content that isn't there
- Preserve paragraph breaks for topic changes
Return only the cleaned text.`

const holisticPrompt = `Analyze the following document summaries and extract:
- core_thesis: The single central thesis or main argument of the entire document (in the original language of the document, 1-2 sentences)
- domains: Comma-separated list of knowledge domains relevant to the content (e.g. business, technology, marketing, psychology, management, entrepreneurship, personal-development, leadership)

Return in this exact format (using the original language of the document):
core_thesis: <thesis statement>
domains: <domain1, domain2, ...>`

const contextEnrichPrompt = `You are given a chunk of text extracted from a larger document, along with a brief summary of the entire document. Your task is to write a short context (1-2 sentences) that situates this chunk within the broader document, so that someone reading only this chunk can understand what document it comes from and what the chunk is about in context.

IMPORTANT:
- Write in the same language as the chunk
- Keep it concise: 1-2 sentences maximum
- Include the document topic/main theme
- Do NOT repeat the chunk content — only add surrounding context
- Do NOT add any headers, labels, or formatting — just the context sentence(s)

Example input:
Document summary: A lecture about scaling a construction business to 1M profit, covering sales funnels, lead generation, and team building.
Chunk: "The goal is to get 15-20 measurements per week. At the meeting, you propose a contract and a prepayment."

Example output:
This is from a lecture on scaling a construction business to 1 million rubles profit, discussing the key stage of the sales funnel — getting measurements and signing contracts with prepayment.`

const causalExtractPrompt = `Analyze the following document (chunk summaries in order) and extract causal relationships between concepts. Focus on:

1. causal_chains: Direct cause-effect relationships found in the content
   Format: cause -> mechanism -> effect
   Relation types: causes, enables, prevents, requires, correlates

2. preconditions: Conditions that make events/processes possible ("If X, then Y becomes possible")

3. counterfactuals: What would change if a key factor were removed ("Without X, Y would not happen")

Rules:
- Extract only relationships explicitly stated or strongly implied in the text
- Do NOT invent causal relationships — only extract from content
- Write in the same language as the source material
- Be specific: name the actual concepts, not generic terms

Return in this exact format:
causal_chains:
- cause: <cause>
  mechanism: <how it works>
  effect: <effect>
  relation: <causes|enables|prevents|requires|correlates>

- cause: <cause>
  mechanism: <how it works>
  effect: <effect>
  relation: <causes|enables|prevents|requires|correlates>

preconditions:
- <precondition 1>
- <precondition 2>

counterfactuals:
- <counterfactual 1>
- <counterfactual 2>`
