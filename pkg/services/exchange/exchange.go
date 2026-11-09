package exchange

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/samber/do"
)

var (
	ErrCurrencyNotSupported = errors.New("currency not supported")
	ErrExchangeFailed       = errors.New("exchange failed")
)

type ExchangeService interface {
	// Convert converts an amount from one currency to another.
	Convert(ctx context.Context, amount float64, fromCurrency, toCurrency string) (float64, error)
	// GetRates returns all available rates relative to USD.
	GetRates(ctx context.Context) (map[string]float64, error)
}

type exchangeService struct {
	providers []RateProvider
}

func NewExchangeService(i *do.Injector) (ExchangeService, error) {
	// Initialize providers in order of preference (first to try, last as fallback)
	providers := []RateProvider{
		NewFxRatesAPIProvider(),
	}

	return &exchangeService{
		providers: providers,
	}, nil
}

// GetRates tries each provider in order until one succeeds
func (s *exchangeService) GetRates(ctx context.Context) (map[string]float64, error) {
	var lastErr error

	for _, provider := range s.providers {
		rates, err := provider.GetRates(ctx)
		if err == nil {
			slog.Info("Successfully fetched rates", "provider", provider.Name())
			return rates, nil
		}

		lastErr = err
		slog.Warn("Failed to fetch rates from provider",
			"provider", provider.Name(),
			"error", err)
	}

	return nil, fmt.Errorf("all providers failed, last error: %w", lastErr)
}

func (s *exchangeService) Convert(ctx context.Context, amount float64, fromCurrency, toCurrency string) (float64, error) {
	if fromCurrency == toCurrency {
		return amount, nil
	}

	// Get rates from available sources (with fallback)
	rates, err := s.GetRates(ctx)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrExchangeFailed, err)
	}

	// Normalize to base (USD) then to target
	// All rates are in relation to USD
	fromRate, okFrom := rates[fromCurrency]
	toRate, okTo := rates[toCurrency]

	if !okFrom {
		return 0, fmt.Errorf("%w: from currency %s not supported", ErrCurrencyNotSupported, fromCurrency)
	}
	if !okTo {
		return 0, fmt.Errorf("%w: to currency %s not supported", ErrCurrencyNotSupported, toCurrency)
	}

	// Convert: amount in fromCurrency -> USD -> toCurrency
	// If fromCurrency is USD, fromRate = 1.0, so amountInUSD = amount / 1.0 = amount
	// If toCurrency is USD, toRate = 1.0, so result = amountInUSD * 1.0 = amountInUSD
	amountInUSD := amount / fromRate
	converted := amountInUSD * toRate

	return converted, nil
}
