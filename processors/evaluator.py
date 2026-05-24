import json
import re
from pathlib import Path
from typing import Optional

import yaml

from processors.output_parser import MarkdownOutputParser


class ContentEvaluator:
    """Evaluate and optimize document quality through self-reflection loop."""

    EVALUATE_SYSTEM_PROMPT = (
        "You are a quality evaluator for structured knowledge documents. "
        "Assess the document against five criteria and produce scores + critique.\n\n"
        "CRITERIA (score 0.0 to 1.0):\n"
        "- structure: logical organization, clear hierarchy, proper use of sections\n"
        "- completeness: all key information covered, no gaps in reasoning\n"
        "- signal_to_noise: ratio of useful content to filler, no redundancy\n"
        "- actionability: presence of actionable takeaways, frameworks, steps\n"
        "- language: grammar, clarity, consistent tone in source language\n\n"
        "WEIGHTS: structure=0.2, completeness=0.3, signal_to_noise=0.2, actionability=0.15, language=0.15\n\n"
        "THRESHOLDS: overall >= 0.75 AND all individual >= 0.6 = pass\n\n"
        "OUTPUT FORMAT: YAML frontmatter with scores, followed by Markdown critique.\n"
        "Start directly with --- (no preamble)."
    )

    OPTIMIZE_SYSTEM_PROMPT = (
        "You are a content optimizer. Improve a document based on specific critique "
        "while preserving ALL facts, numbers, quotes, and core meaning.\n\n"
        "RULES:\n"
        "- Address every issue mentioned in the critique\n"
        "- Preserve ALL facts, numbers, examples, frameworks, quotes\n"
        "- Do not add new information not present in the original\n"
        "- Maintain the source language\n"
        "- Keep the same section structure\n"
        "- Output ONLY the improved document — no commentary"
    )

    def __init__(self, llm_client, json_mode: bool = False, reasoning: bool = False,
                 threshold: float = 0.75, min_individual: float = 0.6, max_iterations: int = 2):
        self._client = llm_client
        self._json_mode = json_mode
        self._reasoning = reasoning
        self._threshold = threshold
        self._min_individual = min_individual
        self._max_iterations = max_iterations
        self._parser = MarkdownOutputParser()

    def _emit(self, status: str, **kwargs):
        if self._json_mode:
            obj = {"status": status, **kwargs}
            print(json.dumps(obj, ensure_ascii=False), flush=True)
        else:
            messages = {
                "eval_start": f"  🔍 Evaluating... (iteration {kwargs.get('iteration')}/{kwargs.get('max_iterations')})",
                "eval_done": f"  📊 Scores: overall={kwargs.get('overall')}, needs_revision={kwargs.get('needs_revision')}",
                "eval_passed": f"  ✅ Quality check PASSED (overall={kwargs.get('overall')})",
                "optimizing": f"  🔧 Optimizing... (iteration {kwargs.get('iteration')})",
                "optimize_done": f"  ✅ Optimization complete (iteration {kwargs.get('iteration')})",
                "eval_max_iterations": f"  ⚠️ Max iterations reached (overall={kwargs.get('overall')})",
            }
            msg = messages.get(status, f"  [{status}] {kwargs}")
            print(msg, flush=True)

    def _chat(self, prompt: str, system: str = "") -> str:
        try:
            return self._client.chat(prompt=prompt, system=system, reasoning=self._reasoning)
        except Exception:
            return self._client.chat(prompt=prompt, system=system, reasoning=False)

    def evaluate(self, document: str) -> dict:
        """Evaluate a document and return scores + critique.

        Returns dict with: structure, completeness, signal_to_noise, actionability,
        language, overall, needs_revision, critique (markdown string).
        """
        prompt = (
            "Evaluate this document:\n\n"
            f"{document}\n\n"
            "Produce YAML frontmatter with scores and a Markdown critique body."
        )

        response = self._chat(prompt=prompt, system=self.EVALUATE_SYSTEM_PROMPT)
        parsed = self._parser.parse(response)

        scores = parsed.metadata or {}
        structure = float(scores.get("structure", 0.5))
        completeness = float(scores.get("completeness", 0.5))
        signal_to_noise = float(scores.get("signal_to_noise", 0.5))
        actionability = float(scores.get("actionability", 0.5))
        language = float(scores.get("language", 0.5))

        overall = (
            structure * 0.2 +
            completeness * 0.3 +
            signal_to_noise * 0.2 +
            actionability * 0.15 +
            language * 0.15
        )

        needs_revision = overall < self._threshold or any(
            s < self._min_individual for s in [structure, completeness, signal_to_noise, actionability, language]
        )

        return {
            "structure": structure,
            "completeness": completeness,
            "signal_to_noise": signal_to_noise,
            "actionability": actionability,
            "language": language,
            "overall": round(overall, 2),
            "needs_revision": needs_revision,
            "critique": parsed.content,
        }

    def print_evaluation(self, evaluation: dict):
        """Print detailed evaluation results."""
        scores = evaluation
        bar = lambda v: "█" * int(v * 10) + "░" * (10 - int(v * 10))
        print(f"\n  {'='*50}")
        print(f"  {'Quality Evaluation Results':^50}")
        print(f"  {'='*50}")
        print(f"  Structure:      {bar(scores['structure'])} {scores['structure']:.2f}")
        print(f"  Completeness:   {bar(scores['completeness'])} {scores['completeness']:.2f}")
        print(f"  Signal/Noise:   {bar(scores['signal_to_noise'])} {scores['signal_to_noise']:.2f}")
        print(f"  Actionability:  {bar(scores['actionability'])} {scores['actionability']:.2f}")
        print(f"  Language:       {bar(scores['language'])} {scores['language']:.2f}")
        print(f"  {'─'*50}")
        print(f"  Overall:        {bar(scores['overall'])} {scores['overall']:.2f}")
        status = "✅ PASS" if not scores["needs_revision"] else "❌ FAIL"
        print(f"  Status:         {status}")
        if scores["needs_revision"]:
            print(f"\n  Critique preview:")
            critique_lines = scores.get("critique", "")[:300]
            for line in critique_lines.split("\n")[:5]:
                print(f"    {line}")
        print(f"  {'='*50}\n")

    def optimize(self, document: str, critique: str) -> str:
        """Optimize a document based on evaluator critique."""
        prompt = (
            f"## Original Document\n\n{document}\n\n"
            f"## Critique\n\n{critique}\n\n"
            "Improve the document addressing all issues in the critique."
        )

        return self._chat(prompt=prompt, system=self.OPTIMIZE_SYSTEM_PROMPT)

    def save_evaluation(self, workspace_dir: Path | None, evaluation: dict):
        """Save evaluation scores to workspace directory."""
        if not workspace_dir:
            return
        quality_dir = workspace_dir / "quality"
        quality_dir.mkdir(parents=True, exist_ok=True)
        eval_path = quality_dir / "evaluation.json"
        existing = []
        if eval_path.exists():
            try:
                existing = json.loads(eval_path.read_text(encoding="utf-8"))
                if not isinstance(existing, list):
                    existing = [existing]
            except Exception:
                existing = []
        existing.append(evaluation)
        eval_path.write_text(json.dumps(existing, ensure_ascii=False, indent=2), encoding="utf-8")

    def evaluate_and_optimize(self, document: str, workspace_dir: Path | None = None) -> tuple[str, dict]:
        """Run the evaluate → optimize loop up to max_iterations.

        Returns (improved_document, final_evaluation).
        """
        current = document
        final_eval = None

        for iteration in range(1, self._max_iterations + 1):
            self._emit("eval_start", iteration=iteration, max_iterations=self._max_iterations)
            evaluation = self.evaluate(current)
            final_eval = evaluation

            if not self._json_mode:
                self.print_evaluation(evaluation)

            self.save_evaluation(workspace_dir, evaluation)

            self._emit("eval_done", iteration=iteration, overall=evaluation["overall"],
                       needs_revision=evaluation["needs_revision"],
                       scores={k: evaluation[k] for k in ["structure", "completeness", "signal_to_noise", "actionability", "language"] if k in evaluation})

            if not evaluation["needs_revision"]:
                self._emit("eval_passed", iteration=iteration, overall=evaluation["overall"])
                return current, evaluation

            if iteration < self._max_iterations:
                self._emit("optimizing", iteration=iteration)
                current = self.optimize(current, evaluation["critique"])
                self._emit("optimize_done", iteration=iteration)

        self._emit("eval_max_iterations", iterations=self._max_iterations,
                   overall=final_eval["overall"] if final_eval else 0)
        self.save_evaluation(workspace_dir, final_eval)
        return current, final_eval
