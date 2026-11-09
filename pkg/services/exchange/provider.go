package exchange

import (
	"context"
)

// RateProvider defines the interface for exchange rate providers
type RateProvider interface {
	// GetRates fetches exchange rates. Returns rates map where key is currency code
	// and value is the rate relative to USD (e.g., EUR: 0.92 means 1 USD = 0.92 EUR)
	GetRates(ctx context.Context) (map[string]float64, error)
	
	// Name returns the provider name for logging/debugging
	Name() string
}

