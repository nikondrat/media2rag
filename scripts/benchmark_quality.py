#!/usr/bin/env python3
"""Benchmark script to compare output quality across different LLM models."""

import argparse
import json
import sys
import time
from pathlib import Path
from datetime import datetime

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent.parent))

from config import AppConfig
from clients.ollama_client import OllamaClient
from clients.openrouter_client import OpenRouterClient
from clients.protocol import LLMClient
from domain.document import ExtractedContent, DocumentMetadata
from processors.compressor import Compressor
from processors.chunked_transformer import ChunkedTransformer
from processors.generator import Generator
from processors.evaluator import ContentEvaluator


DEFAULT_MODELS = [
    {"name": "gemma4:26b", "reasoning": False},
    {"name": "qwen3.5:35b", "reasoning": False},
    {"name": "qwen3.5:9b", "reasoning": False},
]


def get_client_for_model(model_name: str) -> LLMClient:
    """Create an Ollama client configured for the specific model."""
    cfg = AppConfig.from_env()
    cfg.llm_backend = "ollama"
    cfg.ollama.ctg_model = model_name
    return OllamaClient(cfg.ollama)


def run_pipeline_for_model(client: LLMClient, raw_text: str, model_name: str,
                           workspace_dir: Path) -> dict:
    """Run the full CTG pipeline with a specific model and return results."""
    start_time = time.time()

    compressor = Compressor(client, reasoning=False)
    chunked_transformer = ChunkedTransformer(client, reasoning=False, work_dir=workspace_dir)
    generator = Generator()

    compressed = compressor.compress(raw_text)
    compressed = Compressor.clean_artifacts(compressed)

    metadata = DocumentMetadata(
        title="Benchmark Document",
        source="benchmark",
        doc_type="benchmark",
    )

    structured, metadata = chunked_transformer.map_reduce(compressed, metadata, source_path="benchmark")
    doc = generator.generate(structured, metadata, source_path="benchmark")

    elapsed = time.time() - start_time

    output_path = workspace_dir / f"output_{model_name.replace(':', '_')}.md"
    output_path.write_text(doc.markdown, encoding="utf-8")

    return {
        "model": model_name,
        "elapsed_seconds": round(elapsed, 1),
        "output_path": str(output_path),
        "output_size": len(doc.markdown),
        "content": doc.markdown,
    }


def score_output(evaluator: ContentEvaluator, content: str) -> dict:
    """Score output using the evaluator."""
    evaluation = evaluator.evaluate(content)
    return {
        "structure": evaluation["structure"],
        "completeness": evaluation["completeness"],
        "signal_to_noise": evaluation["signal_to_noise"],
        "actionability": evaluation["actionability"],
        "language": evaluation["language"],
        "overall": evaluation["overall"],
    }


def generate_report(results: list[dict]) -> str:
    """Generate a markdown comparison report."""
    sorted_results = sorted(results, key=lambda r: r.get("scores", {}).get("overall", 0), reverse=True)

    lines = [
        "# Quality Benchmark Report",
        f"\nGenerated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}\n",
        "## Results (sorted by overall score)\n",
        "| Model | Time (s) | Size | Structure | Completeness | Signal/Noise | Actionability | Language | Overall |",
        "|-------|----------|------|-----------|--------------|--------------|---------------|----------|---------|",
    ]

    for r in sorted_results:
        scores = r.get("scores", {})
        lines.append(
            f"| {r['model']} | {r['elapsed_seconds']}s | {r['output_size']} chars | "
            f"{scores.get('structure', 'N/A')} | {scores.get('completeness', 'N/A')} | "
            f"{scores.get('signal_to_noise', 'N/A')} | {scores.get('actionability', 'N/A')} | "
            f"{scores.get('language', 'N/A')} | {scores.get('overall', 'N/A')} |"
        )

    lines.append("\n## Output Files\n")
    for r in sorted_results:
        lines.append(f"- **{r['model']}**: `{r['output_path']}`")

    return "\n".join(lines)


def main():
    parser = argparse.ArgumentParser(description="Benchmark CTG pipeline quality across models")
    parser.add_argument("file", help="Input file to process")
    parser.add_argument("--models", nargs="+", help="Models to test (default: gemma4:26b qwen3.5:35b qwen3.5:9b)")
    parser.add_argument("--output", default="benchmark_report.md", help="Output report path")
    parser.add_argument("--workspace", default=None, help="Workspace directory")
    args = parser.parse_args()

    input_path = Path(args.file)
    if not input_path.exists():
        print(f"Error: File not found: {input_path}")
        sys.exit(1)

    raw_text = input_path.read_text(encoding="utf-8")

    models = []
    if args.models:
        for m in args.models:
            models.append({"name": m, "reasoning": False})
    else:
        models = DEFAULT_MODELS

    workspace = Path(args.workspace) if args.workspace else Path("benchmark_workspace")
    workspace.mkdir(parents=True, exist_ok=True)

    evaluator_client = get_client_for_model("qwen3.5:35b")
    evaluator = ContentEvaluator(evaluator_client)

    results = []
    for model_config in models:
        model_name = model_config["name"]
        print(f"\n{'='*60}")
        print(f"Running: {model_name}")
        print(f"{'='*60}")

        client = get_client_for_model(model_name)
        model_workspace = workspace / model_name.replace(":", "_")
        model_workspace.mkdir(parents=True, exist_ok=True)

        try:
            run_result = run_pipeline_for_model(client, raw_text, model_name, model_workspace)
            print(f"  Pipeline complete in {run_result['elapsed_seconds']}s")

            print(f"  Scoring output...")
            scores = score_output(evaluator, run_result["content"])
            run_result["scores"] = scores
            print(f"  Overall score: {scores['overall']}")

            results.append(run_result)
        except Exception as e:
            print(f"  ERROR: {e}")
            results.append({
                "model": model_name,
                "elapsed_seconds": 0,
                "output_size": 0,
                "error": str(e),
                "scores": {"structure": 0, "completeness": 0, "signal_to_noise": 0,
                           "actionability": 0, "language": 0, "overall": 0},
            })

    report = generate_report(results)
    report_path = Path(args.output)
    report_path.write_text(report, encoding="utf-8")

    print(f"\n{'='*60}")
    print(f"Report saved to: {report_path}")
    print(f"{'='*60}")
    print(report)


if __name__ == "__main__":
    main()
