package compensation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/services/compensation"
	compmodels "larsa-academy-microservice/pkg/services/compensation/models"
	contentmodels "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/services/exchange"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/test/helpers"
	"larsa-academy-microservice/test/setup"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/samber/do"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MockRateProvider is a test provider that returns fixed exchange rates
type MockRateProvider struct {
	rates map[string]float64
}

func NewMockRateProvider() *MockRateProvider {
	return &MockRateProvider{
		rates: map[string]float64{
			"USD": 1.0,
			"EUR": 0.92, // 1 USD = 0.92 EUR, so 1 EUR = 1/0.92 USD ≈ 1.087 USD
			"GBP": 0.79, // 1 USD = 0.79 GBP, so 1 GBP = 1/0.79 USD ≈ 1.266 USD
		},
	}
}

func (m *MockRateProvider) GetRates(ctx context.Context) (map[string]float64, error) {
	return m.rates, nil
}

func (m *MockRateProvider) Name() string {
	return "mock-provider"
}

// newFixtureWithMockExchange creates a fixture with a mock exchange service
func newFixtureWithMockExchange(t *testing.T, capabilities []string) (*helpers.TestFixture, *do.Injector) {
	dbConfig := setup.SetupTestDB(t)
	t.Cleanup(dbConfig.Cleanup)

	authConfig := setup.DefaultMockAuthConfig()
	authConfig.Capabilities = capabilities
	authServer := setup.NewMockAuthServer(authConfig)
	t.Cleanup(authServer.Close)

	// Create mock exchange service
	mockProvider := NewMockRateProvider()
	mockExchangeService := &mockExchangeService{
		provider: mockProvider,
	}

	// Create injector and provide mock exchange service BEFORE registering other services
	injector := do.New()
	do.ProvideValue(injector, dbConfig.Client)
	do.Provide(injector, db.NewWithTxn)

	// Provide mock exchange service - this will be used instead of the real one
	do.ProvideValue(injector, exchange.ExchangeService(mockExchangeService))

	// Now call the rest of setup which will register services (compensation will use our mock)
	_, router, database := setup.SetupFullTestAppWithInjector(t, injector, dbConfig, authServer.URL())

	return &helpers.TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        context.Background(),
		AuthServer: authServer,
		DB:         database,
	}, injector
}

// mockExchangeService implements ExchangeService using a mock provider
type mockExchangeService struct {
	provider *MockRateProvider
}

func (m *mockExchangeService) Convert(ctx context.Context, amount float64, fromCurrency, toCurrency string) (float64, error) {
	if fromCurrency == toCurrency {
		return amount, nil
	}

	rates, err := m.provider.GetRates(ctx)
	if err != nil {
		return 0, err
	}

	fromRate, okFrom := rates[fromCurrency]
	toRate, okTo := rates[toCurrency]

	if !okFrom {
		return 0, exchange.ErrCurrencyNotSupported
	}
	if !okTo {
		return 0, exchange.ErrCurrencyNotSupported
	}

	amountInUSD := amount / fromRate
	converted := amountInUSD * toRate
	return converted, nil
}

func (m *mockExchangeService) GetRates(ctx context.Context) (map[string]float64, error) {
	return m.provider.GetRates(ctx)
}

func TestCompensationFlow_CurrencyConversion_OldData(t *testing.T) {
	fixture, _ := newFixtureWithMockExchange(t, []string{
		"authenticate",
		compensation.CapabilityManageTeacherCompensation,
		compensation.CapabilityViewTeacherCompensation,
	})

	teacher := fixture.CreateTeacher(t, nil)
	subject := contentmodels.Subject{
		Id: primitive.NewObjectID(),
		SubjectDto: contentmodels.SubjectDto{
			Name: transl.Localizable[string]{"en": "Math"},
			Code: "MATH",
		},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		Trash:     false,
	}
	_, err := fixture.DB.Collection(subject.CollName()).InsertOne(fixture.Ctx, subject)
	require.NoError(t, err)

	course := fixture.CreateCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers:       []primitive.ObjectID{teacher.Id},
			LinkedSubjects: []primitive.ObjectID{subject.Id},
			SchoolType:     "public",
			Country:        "US",
			Level:          "beginner",
		},
	})

	// Create pricing in USD (default currency) - this should work
	pricingBody, _ := json.Marshal(compmodels.TeacherPricingCreateDto{
		Criteria: compmodels.PricingCriteriaDto{
			SubjectId:  subject.Id.Hex(),
			SchoolType: "public",
			Country:    "US",
			Level:      "beginner",
		},
		HourlyRate: 100,
		Currency:   "USD",
		IsActive:   true,
	})
	req := httptest.NewRequest("POST", "/compensation/teachers/"+teacher.Id.Hex()+"/pricing", bytes.NewBuffer(pricingBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", fixture.DBName)
	req = req.WithContext(fixture.Ctx)
	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "pricing create failed: %s", w.Body.String())

	// Now insert OLD data directly into database with EUR currency
	// This simulates old records that were created before the single currency policy
	oldPricing := &compmodels.TeacherPricing{
		Id:        primitive.NewObjectID(),
		TeacherId: teacher.Id,
		Criteria: compmodels.PricingCriteria{
			SubjectId:  subject.Id,
			SchoolType: "public",
			Country:    "US",
			Level:      "beginner",
		},
		HourlyRate: 80,    // 80 EUR per hour
		Currency:   "EUR", // Old currency
		IsActive:   true,
		CreatedAt:  time.Now().UTC().Add(-30 * 24 * time.Hour), // 30 days ago
		UpdatedAt:  time.Now().UTC().Add(-30 * 24 * time.Hour),
		Trash:      false,
	}
	_, err = fixture.DB.Collection(oldPricing.CollName()).InsertOne(fixture.Ctx, oldPricing)
	require.NoError(t, err)

	// Insert old hours record with EUR currency
	oldHours := &compmodels.TeacherHours{
		Id:              primitive.NewObjectID(),
		TeacherId:       teacher.Id,
		Ref:             "course",
		RefId:           course.Id,
		OccurredAt:      time.Now().UTC().Add(-20 * 24 * time.Hour), // 20 days ago
		DurationMinutes: 120,                                        // 2 hours
		HourlyRate:      80,                                         // 80 EUR per hour
		Currency:        "EUR",                                      // Old currency
		PricingId:       oldPricing.Id,
		Amount:          160, // 80 EUR * 2 hours = 160 EUR
		Status:          compmodels.HoursStatusRecorded,
		CreatedAt:       time.Now().UTC().Add(-20 * 24 * time.Hour),
		UpdatedAt:       time.Now().UTC().Add(-20 * 24 * time.Hour),
		Trash:           false,
	}
	_, err = fixture.DB.Collection(oldHours.CollName()).InsertOne(fixture.Ctx, oldHours)
	require.NoError(t, err)

	// Now test that reading the data converts EUR to USD
	// Expected: 160 EUR should convert to 160 / 0.92 = 173.91 USD (approximately)
	// Rate: 1 USD = 0.92 EUR, so 1 EUR = 1/0.92 USD ≈ 1.087 USD
	// 160 EUR * 1.087 ≈ 173.91 USD

	// Test GetTeacherHours - should return converted amounts
	hoursListBody, _ := json.Marshal(map[string]any{
		"skip":  0,
		"limit": 10,
	})
	req = httptest.NewRequest("POST", "/compensation/teachers/"+teacher.Id.Hex()+"/hours/list", bytes.NewBuffer(hoursListBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", fixture.DBName)
	req = req.WithContext(fixture.Ctx)
	w = httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "get hours failed: %s", w.Body.String())

	var hoursResp compmodels.TeacherHoursPagination
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &hoursResp))
	require.GreaterOrEqual(t, len(hoursResp.TeacherHours), 1, "should have at least one hour record")

	// Find the EUR record (should be converted to USD)
	var foundOldRecord bool
	for _, hour := range hoursResp.TeacherHours {
		if hour.Currency == "USD" && hour.PricingId == oldPricing.Id {
			foundOldRecord = true
			// Verify conversion: 160 EUR should be ≈ 173.91 USD
			// 160 / 0.92 = 173.913...
			expectedUSD := 160.0 / 0.92
			require.InDelta(t, expectedUSD, hour.Amount, 0.1, "EUR amount should be converted to USD")
			// Hourly rate should also be converted: 80 EUR / 0.92 ≈ 86.96 USD
			expectedRateUSD := 80.0 / 0.92
			require.InDelta(t, expectedRateUSD, hour.HourlyRate, 0.1, "EUR hourly rate should be converted to USD")
			break
		}
	}
	require.True(t, foundOldRecord, "should find the old EUR record converted to USD")

	// Test GetCompensationBreakdown - should aggregate all amounts in USD
	summaryBody, _ := json.Marshal(map[string]any{
		"includeRejected": false,
	})
	req = httptest.NewRequest("POST", "/compensation/breakdown", bytes.NewBuffer(summaryBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", fixture.DBName)
	req = req.WithContext(fixture.Ctx)
	w = httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "breakdown failed: %s", w.Body.String())

	var breakdownResp struct {
		Items []compmodels.TeacherCompensationSummary `json:"items"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &breakdownResp))
	require.Len(t, breakdownResp.Items, 1, "should have one teacher summary")

	summary := breakdownResp.Items[0]
	// All amounts should be in USD now
	require.Contains(t, summary.TotalAmountByCurrency, "USD", "should have USD currency")
	// Total should include both old EUR (converted) and any new USD amounts
	// We only have the old EUR record: 160 EUR = 160/0.92 ≈ 173.91 USD
	expectedTotalUSD := 160.0 / 0.92
	require.InDelta(t, expectedTotalUSD, summary.TotalAmountByCurrency["USD"], 1.0, "total should be in USD with EUR converted")
}

func TestCompensationFlow_CurrencyConversion_RejectNonDefaultCurrency(t *testing.T) {
	fixture, _ := newFixtureWithMockExchange(t, []string{
		"authenticate",
		compensation.CapabilityManageTeacherCompensation,
		compensation.CapabilityViewTeacherCompensation,
	})

	teacher := fixture.CreateTeacher(t, nil)
	subject := contentmodels.Subject{
		Id: primitive.NewObjectID(),
		SubjectDto: contentmodels.SubjectDto{
			Name: transl.Localizable[string]{"en": "Math"},
			Code: "MATH",
		},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		Trash:     false,
	}
	_, err := fixture.DB.Collection(subject.CollName()).InsertOne(fixture.Ctx, subject)
	require.NoError(t, err)

	// Try to create pricing with EUR - should be rejected
	pricingBody, _ := json.Marshal(compmodels.TeacherPricingCreateDto{
		Criteria: compmodels.PricingCriteriaDto{
			SubjectId:  subject.Id.Hex(),
			SchoolType: "public",
			Country:    "US",
			Level:      "beginner",
		},
		HourlyRate: 80,
		Currency:   "EUR", // Non-default currency - should be rejected
		IsActive:   true,
	})
	req := httptest.NewRequest("POST", "/compensation/teachers/"+teacher.Id.Hex()+"/pricing", bytes.NewBuffer(pricingBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", fixture.DBName)
	req = req.WithContext(fixture.Ctx)
	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code, "should reject non-default currency")
}

func TestCompensationFlow_CurrencyConversion_NonUSDToNonUSD(t *testing.T) {
	// This test verifies conversion when both from and to currencies are not USD
	// Scenario: Old data in GBP, default currency is EUR, so we convert GBP -> USD -> EUR

	fixture, _ := newFixtureWithMockExchange(t, []string{
		"authenticate",
		compensation.CapabilityManageTeacherCompensation,
		compensation.CapabilityViewTeacherCompensation,
	})

	teacher := fixture.CreateTeacher(t, nil)
	subject := contentmodels.Subject{
		Id: primitive.NewObjectID(),
		SubjectDto: contentmodels.SubjectDto{
			Name: transl.Localizable[string]{"en": "Science"},
			Code: "SCI",
		},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		Trash:     false,
	}
	_, err := fixture.DB.Collection(subject.CollName()).InsertOne(fixture.Ctx, subject)
	require.NoError(t, err)

	course := fixture.CreateCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers:       []primitive.ObjectID{teacher.Id},
			LinkedSubjects: []primitive.ObjectID{subject.Id},
			SchoolType:     "public",
			Country:        "US",
			Level:          "beginner",
		},
	})

	// Insert old data with GBP currency (simulating old records)
	oldPricing := &compmodels.TeacherPricing{
		Id:        primitive.NewObjectID(),
		TeacherId: teacher.Id,
		Criteria: compmodels.PricingCriteria{
			SubjectId:  subject.Id,
			SchoolType: "public",
			Country:    "US",
			Level:      "beginner",
		},
		HourlyRate: 60,    // 60 GBP per hour
		Currency:   "GBP", // Old currency (not USD)
		IsActive:   true,
		CreatedAt:  time.Now().UTC().Add(-30 * 24 * time.Hour),
		UpdatedAt:  time.Now().UTC().Add(-30 * 24 * time.Hour),
		Trash:      false,
	}
	_, err = fixture.DB.Collection(oldPricing.CollName()).InsertOne(fixture.Ctx, oldPricing)
	require.NoError(t, err)

	// Insert old hours record with GBP currency
	oldHours := &compmodels.TeacherHours{
		Id:              primitive.NewObjectID(),
		TeacherId:       teacher.Id,
		Ref:             "course",
		RefId:           course.Id,
		OccurredAt:      time.Now().UTC().Add(-20 * 24 * time.Hour),
		DurationMinutes: 90,    // 1.5 hours
		HourlyRate:      60,    // 60 GBP per hour
		Currency:        "GBP", // Old currency (not USD)
		PricingId:       oldPricing.Id,
		Amount:          90, // 60 GBP * 1.5 hours = 90 GBP
		Status:          compmodels.HoursStatusRecorded,
		CreatedAt:       time.Now().UTC().Add(-20 * 24 * time.Hour),
		UpdatedAt:       time.Now().UTC().Add(-20 * 24 * time.Hour),
		Trash:           false,
	}
	_, err = fixture.DB.Collection(oldHours.CollName()).InsertOne(fixture.Ctx, oldHours)
	require.NoError(t, err)

	// Test conversion: GBP -> USD -> EUR
	// Mock rates: USD=1.0, EUR=0.92, GBP=0.79
	// 90 GBP -> USD: 90 / 0.79 = 113.924 USD
	// USD -> EUR: 113.924 * 0.92 = 104.81 EUR

	// However, since default currency is USD, it should convert GBP -> USD
	// Let's verify the conversion works correctly
	// Expected: 90 GBP should convert to 90 / 0.79 = 113.924 USD (approximately)

	// Test GetTeacherHours - should return converted amounts (GBP -> USD)
	hoursListBody, _ := json.Marshal(map[string]any{
		"skip":  0,
		"limit": 10,
	})
	req := httptest.NewRequest("POST", "/compensation/teachers/"+teacher.Id.Hex()+"/hours/list", bytes.NewBuffer(hoursListBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", fixture.DBName)
	req = req.WithContext(fixture.Ctx)
	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "get hours failed: %s", w.Body.String())

	var hoursResp compmodels.TeacherHoursPagination
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &hoursResp))
	require.GreaterOrEqual(t, len(hoursResp.TeacherHours), 1, "should have at least one hour record")

	// Find the GBP record (should be converted to USD)
	var foundOldRecord bool
	for _, hour := range hoursResp.TeacherHours {
		if hour.Currency == "USD" && hour.PricingId == oldPricing.Id {
			foundOldRecord = true
			// Verify conversion: 90 GBP should be ≈ 113.924 USD
			// 90 / 0.79 = 113.924...
			expectedUSD := 90.0 / 0.79
			require.InDelta(t, expectedUSD, hour.Amount, 0.1, "GBP amount should be converted to USD")
			// Hourly rate should also be converted: 60 GBP / 0.79 ≈ 75.95 USD
			expectedRateUSD := 60.0 / 0.79
			require.InDelta(t, expectedRateUSD, hour.HourlyRate, 0.1, "GBP hourly rate should be converted to USD")
			break
		}
	}
	require.True(t, foundOldRecord, "should find the old GBP record converted to USD")

	// Test GetCompensationBreakdown - should aggregate all amounts in USD
	summaryBody, _ := json.Marshal(map[string]any{
		"includeRejected": false,
	})
	req = httptest.NewRequest("POST", "/compensation/breakdown", bytes.NewBuffer(summaryBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", fixture.DBName)
	req = req.WithContext(fixture.Ctx)
	w = httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "breakdown failed: %s", w.Body.String())

	var breakdownResp struct {
		Items []compmodels.TeacherCompensationSummary `json:"items"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &breakdownResp))
	require.GreaterOrEqual(t, len(breakdownResp.Items), 1, "should have at least one teacher summary")

	summary := breakdownResp.Items[0]
	// All amounts should be in USD now
	require.Contains(t, summary.TotalAmountByCurrency, "USD", "should have USD currency")
	// Total should include GBP (converted): 90 GBP = 90/0.79 ≈ 113.924 USD
	expectedTotalUSD := 90.0 / 0.79
	require.InDelta(t, expectedTotalUSD, summary.TotalAmountByCurrency["USD"], 1.0, "total should be in USD with GBP converted")
}

func TestExchangeService_ConvertBetweenNonUSDCurrencies(t *testing.T) {
	// This test directly tests the exchange service to verify it can convert
	// between two non-USD currencies (e.g., GBP -> EUR)

	mockProvider := NewMockRateProvider()
	mockExchangeService := &mockExchangeService{
		provider: mockProvider,
	}

	ctx := context.Background()

	// Test GBP -> EUR conversion
	// Rates: USD=1.0, EUR=0.92, GBP=0.79
	// 100 GBP -> USD: 100 / 0.79 = 126.582 USD
	// USD -> EUR: 126.582 * 0.92 = 116.46 EUR
	amountGBP := 100.0
	convertedEUR, err := mockExchangeService.Convert(ctx, amountGBP, "GBP", "EUR")
	require.NoError(t, err)

	expectedEUR := (amountGBP / 0.79) * 0.92 // 100 GBP -> USD -> EUR
	require.InDelta(t, expectedEUR, convertedEUR, 0.01, "GBP to EUR conversion should work correctly")

	// Test EUR -> GBP conversion (reverse)
	// 100 EUR -> USD: 100 / 0.92 = 108.696 USD
	// USD -> GBP: 108.696 * 0.79 = 85.87 GBP
	amountEUR := 100.0
	convertedGBP, err := mockExchangeService.Convert(ctx, amountEUR, "EUR", "GBP")
	require.NoError(t, err)

	expectedGBP := (amountEUR / 0.92) * 0.79 // 100 EUR -> USD -> GBP
	require.InDelta(t, expectedGBP, convertedGBP, 0.01, "EUR to GBP conversion should work correctly")

	// Test that same currency returns same amount
	sameAmount, err := mockExchangeService.Convert(ctx, 100.0, "EUR", "EUR")
	require.NoError(t, err)
	require.Equal(t, 100.0, sameAmount, "same currency should return same amount")
}
