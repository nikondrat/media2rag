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
	InputPrice  float64
	OutputPrice float64
}

var (
	pricingCache     map[string]ModelPricing
	pricingCacheMu   sync.RWMutex
	pricingCacheTime time.Time
	pricingURL       = "https://models.dev/api.json"
)

func init() {
	pricingCache = make(map[string]ModelPricing)
}

type pricingAPIProvider struct {
	Models map[string]pricingAPIModel `json:"models"`
}

type pricingAPIModel struct {
	ID   string           `json:"id"`
	Cost *pricingAPICost  `json:"cost"`
}

type pricingAPICost struct {
	Input  float64 `json:"input"`
	Output float64 `json:"output"`
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

	var data map[string]pricingAPIProvider
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return fmt.Errorf("decode pricing: %w", err)
	}

	newCache := make(map[string]ModelPricing, len(data)*10)

	for _, provider := range data {
		for modelKey, m := range provider.Models {
			if m.Cost == nil {
				continue
			}
			key := strings.ToLower(modelKey)
			newCache[key] = ModelPricing{
				InputPrice:  m.Cost.Input,
				OutputPrice: m.Cost.Output,
			}
		}
	}

	pricingCache = newCache
	pricingCacheTime = time.Now()
	return nil
}

func GetPricing(model string) ModelPricing {
	pricingCacheMu.RLock()
	defer pricingCacheMu.RUnlock()

	if p, ok := pricingCache[model]; ok {
		return p
	}

	if idx := strings.Index(model, "-20"); idx > 0 && len(model[idx:]) == 9 {
		allDigits := true
		for _, c := range model[idx+1:] {
			if c < '0' || c > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			base := model[:idx]
			if p, ok := pricingCache[base]; ok {
				return p
			}
		}
	}

	if idx := strings.LastIndex(model, ":"); idx > 0 {
		base := model[:idx]
		if p, ok := pricingCache[base]; ok {
			return p
		}
	}

	return ModelPricing{}
}

func CalculateCost(model string, promptTokens, completionTokens int) float64 {
	p := GetPricing(model)
	return (float64(promptTokens)/1_000_000)*p.InputPrice + (float64(completionTokens)/1_000_000)*p.OutputPrice
}
