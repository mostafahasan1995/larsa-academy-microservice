package helpers

import (
	"context"
	"fmt"
	"sync"
)

// MockExchangeService implements ExchangeService for testing
type MockExchangeService struct {
	rates map[string]float64
	mu    sync.RWMutex
}

// NewMockExchangeService creates a new mock exchange service with USD as base (rate 1.0)
func NewMockExchangeService() *MockExchangeService {
	return &MockExchangeService{
		rates: map[string]float64{
			"USD": 1.0,
		},
	}
}

// SetRate sets the rate for a currency relative to USD
// rate: how many units of 'currency' equals 1 USD
// Example: if 1 USD = 0.9 EUR, SetRate("EUR", 0.9)
// Note: our system's pricingsvcs uses: amountInUSD = amount / fromRate
// So fromRate should be units of currency per 1 USD.
func (m *MockExchangeService) SetRate(currency string, rate float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rates[currency] = rate
}

func (m *MockExchangeService) Convert(ctx context.Context, amount float64, fromCurrency, toCurrency string) (float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if fromCurrency == toCurrency {
		return amount, nil
	}

	fromRate, okFrom := m.rates[fromCurrency]
	toRate, okTo := m.rates[toCurrency]

	if !okFrom {
		return 0, fmt.Errorf("mock exchange: from currency %s not supported", fromCurrency)
	}
	if !okTo {
		return 0, fmt.Errorf("mock exchange: to currency %s not supported", toCurrency)
	}

	// amount (from) -> USD -> to
	amountInUSD := amount / fromRate
	converted := amountInUSD * toRate

	return converted, nil
}

func (m *MockExchangeService) GetRates(ctx context.Context) (map[string]float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// Return a copy to avoid race conditions
	ratesCopy := make(map[string]float64)
	for k, v := range m.rates {
		ratesCopy[k] = v
	}
	return ratesCopy, nil
}
