package ego

import (
	_ "embed"
	"encoding/json"
	"sync"
)

//go:embed assets/pricing.json
var embeddedPricingJSON []byte

type rawEmbeddedPricingEntry struct {
	Model         string  `json:"m"`
	InputCost     float64 `json:"in"`
	OutputCost    float64 `json:"out"`
	CacheRead     float64 `json:"cr,omitempty"`
	CacheCreation float64 `json:"cc,omitempty"`
}

var (
	embeddedPricingCache []ModelPricing
	embeddedPricingOnce  sync.Once
)

func getEmbeddedPricing() ([]ModelPricing, error) {
	var parseErr error
	embeddedPricingOnce.Do(func() {
		if len(embeddedPricingJSON) == 0 {
			return
		}
		var raw []rawEmbeddedPricingEntry
		if err := json.Unmarshal(embeddedPricingJSON, &raw); err != nil {
			parseErr = err
			return
		}
		results := make([]ModelPricing, len(raw))
		for i, r := range raw {
			results[i] = ModelPricing{
				Model:                       r.Model,
				InputCostPerToken:           r.InputCost,
				OutputCostPerToken:          r.OutputCost,
				CacheReadInputTokenCost:     r.CacheRead,
				CacheCreationInputTokenCost: r.CacheCreation,
			}
		}
		embeddedPricingCache = results
	})
	return embeddedPricingCache, parseErr
}
