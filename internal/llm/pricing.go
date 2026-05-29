package llm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type ModelPricing struct {
	InputPrice  float64 `json:"input_price"`
	OutputPrice float64 `json:"output_price"`
}

var (
	pricingCache     map[string]ModelPricing
	pricingCacheMu   sync.RWMutex
	pricingCacheTime time.Time
	pricingURL       = "https://models.dev/api.json"
)

var defaultPricing = map[string]ModelPricing{
	"openrouter/free":                        {InputPrice: 0, OutputPrice: 0},
	"deepseek/deepseek-v4-flash:free":        {InputPrice: 0, OutputPrice: 0},
	"mistralai/mistral-nemo":                 {InputPrice: 0.5, OutputPrice: 1.5},
	"nvidia/nemotron-3-super-120b-a12b:free": {InputPrice: 0, OutputPrice: 0},
	"qwen/qwen3-coder:free":                  {InputPrice: 0, OutputPrice: 0},
	"qwen/qwen3.5-9b":                        {InputPrice: 0.3, OutputPrice: 0.6},
}

func init() {
	pricingCache = make(map[string]ModelPricing)
	for k, v := range defaultPricing {
		pricingCache[k] = v
	}
}

func LoadPricing() error {
	pricingCacheMu.Lock()
	defer pricingCacheMu.Unlock()

	if time.Since(pricingCacheTime) < 15*time.Minute {
		return nil
	}

	resp, err := http.Get(pricingURL)
	if err != nil {
		return fmt.Errorf("fetch pricing: %w", err)
	}
	defer resp.Body.Close()

	var data map[string]struct {
		InputPrice  float64 `json:"input_price"`
		OutputPrice float64 `json:"output_price"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return fmt.Errorf("decode pricing: %w", err)
	}

	for model, p := range data {
		key := strings.ToLower(model)
		pricingCache[key] = ModelPricing{
			InputPrice:  p.InputPrice,
			OutputPrice: p.OutputPrice,
		}
	}

	pricingCacheTime = time.Now()
	return nil
}

func GetPricing(model string) ModelPricing {
	pricingCacheMu.RLock()
	defer pricingCacheMu.RUnlock()

	if p, ok := pricingCache[model]; ok {
		return p
	}

	for key, p := range pricingCache {
		if strings.Contains(strings.ToLower(model), strings.ToLower(key)) {
			return p
		}
	}

	return ModelPricing{}
}

func CalculateCost(model string, promptTokens, completionTokens int) float64 {
	p := GetPricing(model)
	cost := (float64(promptTokens)/1_000_000)*p.InputPrice + (float64(completionTokens)/1_000_000)*p.OutputPrice
	return cost
}
