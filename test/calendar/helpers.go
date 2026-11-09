package calendar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"larsa-academy-microservice/pkg/gateway"
	"larsa-academy-microservice/pkg/services/contact/models"
	"larsa-academy-microservice/pkg/services/content"
	contentenums "larsa-academy-microservice/pkg/services/content/enums"
	contentmodels "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"larsa-academy-microservice/test/helpers"
	placement "larsa-academy-microservice/test/placement_test"
	"larsa-academy-microservice/test/setup"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/samber/do"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type MockPaymentGw struct {
}

func (m *MockPaymentGw) PayOrder(ctx context.Context, requestData *gateway.OrderRequest) (*gateway.PaymentInitiationResponseDTO, error) {
	return &gateway.PaymentInitiationResponseDTO{
		OrderId:    primitive.NewObjectID(),
		IsRedirect: false,
	}, nil
}

// SetupCalendarTest sets up the full test application with required mocks for calendar tests
func SetupCalendarTest(t *testing.T, dbConfig *setup.TestDBConfig, authServerURL string) (*do.Injector, *chi.Mux, *mongo.Database, *TestFixture) {
	injector := do.New()
	placement.RegisterMockZoomService(injector)
	injector, router, database := setup.SetupFullTestAppWithInjector(t, injector, dbConfig, authServerURL)

	fixture := &TestFixture{
		Router:   router,
		DBName:   dbConfig.Database,
		Ctx:      context.Background(),
		DB:       database,
		Injector: injector,
	}

	return injector, router, database, fixture
}

// TestFixture embeds the shared TestFixture and adds calendar-specific methods
// Fields are exposed directly for backward compatibility with existing tests
type TestFixture struct {
	*helpers.TestFixture
	Router     *chi.Mux
	DBName     string
	Ctx        context.Context
	AuthServer *setup.MockAuthServer
	DB         *mongo.Database
	Injector   *do.Injector
}

// initSharedFixture initializes the embedded shared TestFixture
func (f *TestFixture) initSharedFixture() {
	if f.TestFixture == nil {
		f.TestFixture = &helpers.TestFixture{
			Router:     f.Router,
			DBName:     f.DBName,
			Ctx:        f.Ctx,
			AuthServer: f.AuthServer,
			DB:         f.DB,
		}
	}
}

// createTeacher delegates to shared helper
func (f *TestFixture) createTeacher(t *testing.T, dto *models.TeacherDto) models.Teacher {
	f.initSharedFixture()
	return f.TestFixture.CreateTeacher(t, dto)
}

// createStudent delegates to shared helper
func (f *TestFixture) createStudent(t *testing.T, dto *models.StudentDto) models.Student {
	f.initSharedFixture()
	return f.TestFixture.CreateStudent(t, dto)
}

// createCourse delegates to shared helper
func (f *TestFixture) createCourse(t *testing.T, dto *contentmodels.CourseDto) contentmodels.Course {
	f.initSharedFixture()
	return f.TestFixture.CreateCourse(t, dto)
}

// createCoaching delegates to shared helper
func (f *TestFixture) createCoaching(t *testing.T, dto *contentmodels.CoachingDto) contentmodels.Coaching {
	f.initSharedFixture()
	return f.TestFixture.CreateCoaching(t, dto)
}

// switchAuthContext delegates to shared helper
func (f *TestFixture) switchAuthContext(userID string, capabilities []string) {
	f.initSharedFixture()
	f.TestFixture.SwitchAuthContext(userID, capabilities)
}

// createBundle creates a bundle with the given content
func (f *TestFixture) createBundle(t *testing.T, dto *contentmodels.BundleDto) contentmodels.Bundle {
	// Switch to admin context with bundle management capabilities
	originalConfig := f.AuthServer.GetCurrentConfig()
	adminConfig := setup.DefaultMockAuthConfig()
	adminConfig.UserID = originalConfig.UserID
	adminConfig.Capabilities = []string{"authenticate", "ManageAllBundles"}
	adminConfig.ShouldAuthenticate = true
	adminConfig.ShouldAuthorize = true
	f.AuthServer.UpdateConfig(adminConfig)
	defer f.AuthServer.UpdateConfig(originalConfig) // Restore original config

	require.NotNil(t, dto, "Bundle DTO is required")

	// Generate random values for missing fields
	if len(dto.Name) == 0 {
		dto.Name = transl.Localizable[string]{
			"ar": "باقة " + helpers.RandomString(4),
			"en": "Bundle " + helpers.RandomString(4),
		}
	}
	if dto.Status == "" {
		dto.Status = "active"
	}

	body, _ := json.Marshal(dto)
	req := httptest.NewRequest("POST", "/bundles", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "Failed to create bundle: body: %s", w.Body.String())

	var bundle contentmodels.Bundle
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &bundle), "Failed to unmarshal bundle")
	return bundle
}

// createPlacementTest creates a placement test
func (f *TestFixture) createPlacementTest(t *testing.T, dto *contentmodels.PlacementTestDto) contentmodels.PlacementTest {
	// Switch to admin context with placement test management capabilities
	originalConfig := f.AuthServer.GetCurrentConfig()
	adminConfig := setup.DefaultMockAuthConfig()
	adminConfig.UserID = originalConfig.UserID
	adminConfig.Capabilities = []string{"authenticate", "ManageAllPlacementTests"}
	adminConfig.ShouldAuthenticate = true
	adminConfig.ShouldAuthorize = true
	f.AuthServer.UpdateConfig(adminConfig)
	defer f.AuthServer.UpdateConfig(originalConfig) // Restore original config

	if dto == nil {
		dto = &contentmodels.PlacementTestDto{}
	}

	if dto.Title["en"] == "" {
		dto.Title["en"] = "Placement Test " + helpers.RandomString(4)
	}
	if dto.Status == "" {
		dto.Status = "active"
	}
	if dto.Type == "" {
		dto.Type = "online"
	}
	if dto.TestMark == 0 {
		dto.TestMark = 100
	}
	if dto.Settings.NumOfAttempts == 0 {
		dto.Settings.NumOfAttempts = 1
	}
	if dto.Settings.PassScore == 0 {
		dto.Settings.PassScore = 60
	}

	body, _ := json.Marshal(dto)
	req := httptest.NewRequest("POST", "/placement-tests", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "Failed to create placement test: body: %s", w.Body.String())

	var placementTest contentmodels.PlacementTest
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &placementTest), "Failed to unmarshal placement test")
	return placementTest
}

// bookCoachingSessions subscribes student to coaching and books sessions
func (f *TestFixture) bookCoachingSessions(t *testing.T, studentID string, coachingID primitive.ObjectID, teacherID primitive.ObjectID, dates []contentmodels.BookingDate) {
	// 1. Create coaching pricing as admin
	originalConfig := f.AuthServer.GetCurrentConfig()
	adminConfig := setup.DefaultMockAuthConfig()
	adminConfig.UserID = originalConfig.UserID
	adminConfig.Capabilities = []string{"authenticate", "ManageCoachingPricing"}
	adminConfig.ShouldAuthenticate = true
	adminConfig.ShouldAuthorize = true
	f.AuthServer.UpdateConfig(adminConfig)

	// Ensure coaching has an active pricing so order payment succeeds
	pricingDto := contentmodels.CoachingPricingDto{
		Name: transl.Localizable[string]{
			"en": "Test Pricing",
			"ar": "تسعير تجريبي",
		},
		SubscriptionType: "paid",
		PricePerSession:  50,
		Currency:         "USD",
		Frequency:        "once",
		Active:           true,
	}
	pricingBody, _ := json.Marshal(pricingDto)
	pricingReq := httptest.NewRequest("POST", fmt.Sprintf("/coaching/%s/pricing", coachingID.Hex()), bytes.NewBuffer(pricingBody))
	pricingReq.Header.Set("Content-Type", "application/json")
	pricingReq.Header.Set("x-access-token", "test-token")
	pricingReq.Header.Set("x-client", f.DBName)
	pricingReq = pricingReq.WithContext(f.Ctx)
	pricingW := httptest.NewRecorder()
	f.Router.ServeHTTP(pricingW, pricingReq)
	require.Equal(t, http.StatusCreated, pricingW.Code, "Failed to create coaching pricing: body: %s", pricingW.Body.String())

	// 2. Switch to student context for booking
	studentAuthConfig := setup.DefaultMockAuthConfig()
	studentAuthConfig.UserID = studentID
	studentAuthConfig.Capabilities = []string{"authenticate"}
	studentAuthConfig.ShouldAuthenticate = true
	f.AuthServer.UpdateConfig(studentAuthConfig)
	defer f.AuthServer.UpdateConfig(originalConfig)

	// Add to cart using unified cart endpoint (CartItemDto replaces old CartDto)
	subscribe := contentmodels.CoachingSubscribe{
		CoachingProgramId: coachingID,
		NumOfSessions:     4, // Must be even number
		RenewMonthly:      false,
		Price:             200.0,
	}

	cartDto := contentmodels.CartItemDto{
		Type:  contentenums.CartTypeCoaching,
		RefId: coachingID,
		Data:  subscribe,
	}

	cartBody, _ := json.Marshal(cartDto)
	cartReq := httptest.NewRequest("POST", "/api/cart", bytes.NewBuffer(cartBody))
	cartReq.Header.Set("Content-Type", "application/json")
	cartReq.Header.Set("x-access-token", "test-token")
	cartReq.Header.Set("x-client", f.DBName)
	cartReq = cartReq.WithContext(f.Ctx)
	cartW := httptest.NewRecorder()
	f.Router.ServeHTTP(cartW, cartReq)

	require.Equal(t, http.StatusOK, cartW.Code, "Failed to add coaching to cart: body: %s", cartW.Body.String())

	// Parse the cart response to verify it was created
	var cartResponse contentmodels.Cart
	if err := json.Unmarshal(cartW.Body.Bytes(), &cartResponse); err == nil {
		require.NotEmpty(t, cartResponse.Id, "Cart item should have an ID after creation")
	} else {
		// If unmarshaling fails, the response might be in a different format, but 200 status means it succeeded
		// Continue to checkout which will validate the cart
	}

	// Checkout to create order
	checkoutReq := httptest.NewRequest("POST", "/api/cart/checkout", nil)
	checkoutReq.Header.Set("Content-Type", "application/json")
	checkoutReq.Header.Set("x-access-token", "test-token")
	checkoutReq.Header.Set("x-client", f.DBName)
	checkoutReq = checkoutReq.WithContext(f.Ctx)
	checkoutW := httptest.NewRecorder()
	f.Router.ServeHTTP(checkoutW, checkoutReq)

	require.Equal(t, http.StatusOK, checkoutW.Code, "Failed to checkout: body: %s", checkoutW.Body.String())

	// New flow: pay and enroll the created order before booking sessions
	var checkoutOrder contentmodels.Order
	require.NoError(t, json.Unmarshal(checkoutW.Body.Bytes(), &checkoutOrder), "Failed to unmarshal checkout response")
	require.NotEqual(t, primitive.NilObjectID, checkoutOrder.Id, "checkout should return an order id")

	// Pay order
	payReq := httptest.NewRequest("POST", fmt.Sprintf("/api/orders/%s/pay", checkoutOrder.Id.Hex()), nil)
	payReq.Header.Set("Content-Type", "application/json")
	payReq.Header.Set("x-access-token", "test-token")
	payReq.Header.Set("x-client", f.DBName)
	payReq = payReq.WithContext(f.Ctx)
	payW := httptest.NewRecorder()
	f.Router.ServeHTTP(payW, payReq)
	require.Equal(t, http.StatusOK, payW.Code, "Failed to pay order: body: %s", payW.Body.String())

	// Mock payment success by calling orderService.HandlePaymentSuccess directly
	orderService := do.MustInvoke[content.OrderSvcs](f.Injector)
	// Add DB name to context since HandlePaymentSuccess might need it for DB operations
	ctx := context.WithValue(f.Ctx, util.ReqAppCfg, &types.AppCfg{Db: f.DBName})
	_, handleErr := orderService.HandlePaymentSuccess(ctx, checkoutOrder.Id.Hex(), primitive.NewObjectID().Hex())
	require.NoError(t, handleErr, "Failed to handle payment success")

	// Book sessions
	booking := contentmodels.BookingSessions{
		CoachingProgramId: coachingID,
		TeacherId:         teacherID,
		Dates:             dates,
	}

	bookingBody, _ := json.Marshal(booking)
	bookingReq := httptest.NewRequest("POST", "/api/coaching/booking", bytes.NewBuffer(bookingBody))
	bookingReq.Header.Set("Content-Type", "application/json")
	bookingReq.Header.Set("x-access-token", "test-token")
	bookingReq.Header.Set("x-client", f.DBName)
	bookingReq = bookingReq.WithContext(f.Ctx)
	bookingW := httptest.NewRecorder()
	f.Router.ServeHTTP(bookingW, bookingReq)

	require.Equal(t, http.StatusOK, bookingW.Code, "Failed to book sessions: body: %s", bookingW.Body.String())
}

// schedulePlacementTest schedules a placement test for a student and returns the created schedule
func (f *TestFixture) schedulePlacementTest(t *testing.T, studentID string, dto contentmodels.OnlinePlacementTestScheduleDto) contentmodels.OnlinePlacementTestSchedule {
	// Switch to student context with permission to view public coachings
	originalConfig := f.AuthServer.GetCurrentConfig()
	studentAuthConfig := setup.DefaultMockAuthConfig()
	studentAuthConfig.UserID = studentID
	studentAuthConfig.Capabilities = []string{"authenticate", "ViewPublicCoaching"}
	f.AuthServer.UpdateConfig(studentAuthConfig)
	defer f.AuthServer.UpdateConfig(originalConfig)

	body, _ := json.Marshal(dto)
	fmt.Printf("schedulePlacementTest payload: %s\n", string(body))
	req := httptest.NewRequest("POST", "/api/placement-tests/student/schedule", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "Failed to schedule placement test: body: %s", w.Body.String())

	var schedule contentmodels.OnlinePlacementTestSchedule
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &schedule))
	return schedule
}

// getCalendarEvents calls the calendar API and returns the response
func (f *TestFixture) getCalendarEvents(t *testing.T, filters contentmodels.CalendarFilters) contentmodels.CalendarEventsResponse {
	return f.getCalendarEventsWithView(t, filters, "agenda", "", "")
}

// getCalendarEventsWithView calls the calendar API with specific view and date parameters
func (f *TestFixture) getCalendarEventsWithView(t *testing.T, filters contentmodels.CalendarFilters, view, startDate, endDate string) contentmodels.CalendarEventsResponse {
	calendarRequest := contentmodels.CalendarEventsRequest{
		Filters: filters,
	}
	calendarBody, _ := json.Marshal(calendarRequest)

	url := fmt.Sprintf("/api/calendar/events?view=%s", view)
	if startDate != "" {
		url += "&startDate=" + startDate
	}
	if endDate != "" {
		url += "&endDate=" + endDate
	}

	calendarReq := httptest.NewRequest("POST", url, bytes.NewBuffer(calendarBody))
	calendarReq.Header.Set("Content-Type", "application/json")
	calendarReq.Header.Set("x-access-token", "test-token")
	calendarReq.Header.Set("x-client", f.DBName)
	calendarReq = calendarReq.WithContext(f.Ctx)
	calendarW := httptest.NewRecorder()
	f.Router.ServeHTTP(calendarW, calendarReq)

	require.Equal(t, http.StatusOK, calendarW.Code, "Failed to get calendar events: body: %s", calendarW.Body.String())

	var calendarResponse contentmodels.CalendarEventsResponse
	require.NoError(t, json.Unmarshal(calendarW.Body.Bytes(), &calendarResponse), "Failed to unmarshal calendar response")
	return calendarResponse
}

// Assertion helpers

// assertEventsFromBothTeachers checks that events exist from both teachers
func assertEventsFromBothTeachers(t *testing.T, events []contentmodels.EnrichedCalendarEvent, teacher1ID, teacher2ID primitive.ObjectID) {
	hasTeacher1 := false
	hasTeacher2 := false
	for _, event := range events {
		if event.TeacherId == teacher1ID {
			hasTeacher1 = true
		}
		if event.TeacherId == teacher2ID {
			hasTeacher2 = true
		}
	}
	assert.True(t, hasTeacher1, "Should have events from teacher1")
	assert.True(t, hasTeacher2, "Should have events from teacher2")
}

// assertAllEnrichmentPresent checks that all events have proper enrichment
func assertAllEnrichmentPresent(t *testing.T, events []contentmodels.EnrichedCalendarEvent) {
	for _, event := range events {
		assert.NotNil(t, event.Teacher, "Event %s missing teacher enrichment", event.Id.Hex())
		if event.Type == "course_live_session" {
			assert.NotNil(t, event.Course, "Course event %s missing course enrichment", event.Id.Hex())
		}
		if event.Type == "coaching_session" {
			assert.NotNil(t, event.CoachingProgram, "Coaching event %s missing coaching program enrichment", event.Id.Hex())
			assert.NotEmpty(t, event.Students, "Coaching event %s missing student enrichment", event.Id.Hex())
		}
	}
}

// assertEventsSorted checks that events are sorted by start time
func assertEventsSorted(t *testing.T, events []contentmodels.EnrichedCalendarEvent) {
	for i := 1; i < len(events); i++ {
		assert.False(t, events[i].StartTime.Before(events[i-1].StartTime),
			"Events are not sorted by start time: event %d starts before event %d", i, i-1)
	}
}

// assertOnlyTeacherEvents checks that all events belong to the specified teacher
func assertOnlyTeacherEvents(t *testing.T, events []contentmodels.EnrichedCalendarEvent, teacherID primitive.ObjectID) {
	for _, event := range events {
		assert.Equal(t, teacherID, event.TeacherId, "Event %s has wrong teacher ID", event.Id.Hex())
	}
}

// assertBundleFilter checks bundle filtering results
func assertBundleFilter(t *testing.T, events []contentmodels.EnrichedCalendarEvent, filter string, wantBundled, wantIndependent bool) {
	hasBundled := false
	hasIndependent := false

	for _, event := range events {
		if event.BundleId != nil {
			hasBundled = true
		} else {
			hasIndependent = true
		}
	}

	if wantBundled {
		assert.True(t, hasBundled, "Bundle filter '%s': expected bundled events, got none", filter)
	} else {
		assert.False(t, hasBundled, "Bundle filter '%s': expected no bundled events, but found some", filter)
	}
	if wantIndependent {
		assert.True(t, hasIndependent, "Bundle filter '%s': expected independent events, got none", filter)
	} else {
		assert.False(t, hasIndependent, "Bundle filter '%s': expected no independent events, but found some", filter)
	}
}

// enrollStudent enrolls a student in a course by directly creating a StudentCourse record
func (f *TestFixture) enrollStudent(t *testing.T, studentID, courseID string) {
	sID, err := primitive.ObjectIDFromHex(studentID)
	require.NoError(t, err)
	cID, err := primitive.ObjectIDFromHex(courseID)
	require.NoError(t, err)

	enrollment := contentmodels.StudentCourse{
		Id:                primitive.NewObjectID(),
		StudentId:         sID,
		CourseId:          cID,
		StartDate:         time.Now(),
		Status:            "active",
		CertificateStatus: "pending",
		Progress:          0,
		Logins:            0,
		LastActivity:      time.Now(),
		CreatedAt:         time.Now(),
		CreatedBy:         sID,
		UpdatedAt:         time.Now(),
		UpdatedBy:         sID,
		Trash:             false,
	}

	// Use the DB instance from the fixture
	db := f.TestFixture.DB
	require.NotNil(t, db, "Database instance in TestFixture is nil")
	_, err = db.Collection("academyCourseStudents").InsertOne(f.Ctx, enrollment)
	require.NoError(t, err, "Failed to insert student course enrollment")
}

// Helper to create booking dates
func createBookingDates(dates []time.Time, from, to string) []contentmodels.BookingDate {
	result := make([]contentmodels.BookingDate, len(dates))
	for i, date := range dates {
		result[i] = contentmodels.BookingDate{
			Date:    date,
			Weekday: date.Weekday().String(),
			From:    from,
			To:      to,
		}
	}
	return result
}
