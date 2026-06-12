package main

import (
	"context"
	"fmt"
	"math"
	"time"
	"media2rag/internal/llm"
)

func main() {
	ctx := context.Background()
	
	queries := []string{
		"как масштабировать бизнес",
		"рост компании",
		"увеличение продаж",
		"рецепт борща",
		"футбольный матч",
	}
	
	for _, modelName := range []string{"bge-m3", "qwen3-embedding:0.6b"} {
		fmt.Printf("\n=== %s ===\n", modelName)
		client, err := llm.NewClient(ctx, "ollama", "http://localhost:11434", modelName, "", "", "", 30*time.Second)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		
		base, err := client.Embed(ctx, queries[0])
		if err != nil {
			fmt.Printf("Embed error: %v\n", err)
			continue
		}
		fmt.Printf("dim=%d\n", len(base))
		
		for _, q := range queries[1:] {
			emb, err := client.Embed(ctx, q)
			if err != nil {
				fmt.Printf("  %-30s error: %v\n", q, err)
				continue
			}
			sim := cosineSim(base, emb)
			fmt.Printf("  %-30s %.4f\n", q, sim)
		}
	}
}

func cosineSim(a, b []float32) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
