package exchange

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"larsa-academy-microservice/pkg/caching"
)

const (
	cacheTTL = 4 * time.Hour
)

// FxRatesAPIProvider implements RateProvider using fxratesapi.com
type FxRatesAPIProvider struct {
	httpClient *http.Client
	baseURL    string
}

// NewFxRatesAPIProvider creates a new FxRatesAPIProvider
func NewFxRatesAPIProvider() *FxRatesAPIProvider {
	return &FxRatesAPIProvider{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL: "https://api.fxratesapi.com/latest?base=USD&resolution=1m&amount=1&places=6&format=json",
	}
}

// Name returns the provider name
func (p *FxRatesAPIProvider) Name() string {
	return "fxratesapi.com"
}

// FxRatesAPIResponse represents the response from fxratesapi.com
type FxRatesAPIResponse struct {
	Success   bool               `json:"success"`
	Timestamp int64              `json:"timestamp"`
	Base      string             `json:"base"`
	Rates     map[string]float64 `json:"rates"`
}

// buildCacheKey builds a cache key for exchange rates (independent of client ID)
func buildCacheKey() string {
	return "exchange:rates:fxratesapi"
}

// GetRates fetches exchange rates from fxratesapi.com with 4-hour caching
func (p *FxRatesAPIProvider) GetRates(ctx context.Context) (map[string]float64, error) {
	cacheKey := buildCacheKey()

	// Try to get from cache first
	if cached, err := caching.Rdb.GetByKey(cacheKey); err == nil {
		var rates map[string]float64
		if err := json.Unmarshal([]byte(cached), &rates); err == nil {
			return rates, nil
		}
	}

	// Cache miss or invalid cache, fetch from API
	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch rates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var apiResp FxRatesAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("API returned success=false")
	}

	if apiResp.Base != "USD" {
		return nil, fmt.Errorf("expected base currency USD, got %s", apiResp.Base)
	}

	if apiResp.Rates == nil {
		return nil, fmt.Errorf("rates map is nil")
	}

	// Ensure USD is in the rates map
	apiResp.Rates["USD"] = 1.0

	// Cache the result for 4 hours
	caching.Rdb.CacheByKey(cacheKey, apiResp.Rates, cacheTTL)

	return apiResp.Rates, nil
}
