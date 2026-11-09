package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"larsa-academy-microservice/pkg/services/content"
	contentenums "larsa-academy-microservice/pkg/services/content/enums"
	contentmodels "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/services/exchange"
	salesmodels "larsa-academy-microservice/pkg/services/sales/models"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"larsa-academy-microservice/test/helpers"
	"larsa-academy-microservice/test/setup"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/samber/do"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestOrderFlowE2E(t *testing.T) {
	// 1. Setup Environment
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	injector, router, db := setup.SetupFullTestApp(t, dbConfig, authServer.URL())
	orderService := do.MustInvoke[content.OrderSvcs](injector)
	ctx := context.Background()

	fixture := &helpers.TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        ctx,
		AuthServer: authServer,
		DB:         db,
	}

	// 2. Seed Data
	// Create Student
	student := fixture.CreateStudent(t, nil)
	fixture.SwitchAuthContext(student.Id.Hex(), []string{"authenticate"})

	// Create Teacher (for coaching)
	teacher := fixture.CreateTeacher(t, nil)

	// Create Course
	course := fixture.CreateCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
		},
	})

	// Create Course Pricing
	coursePrice := 100.0
	_, err := db.Collection("academyCoursePricings").InsertOne(ctx, contentmodels.CoursePricing{
		Id:       primitive.NewObjectID(),
		CourseId: course.Id,
		CoursePricingDto: contentmodels.CoursePricingDto{
			Price: coursePrice,
			Config: contentmodels.PricingConfig{
				IsDefault: true,
			},
		},
	})
	require.NoError(t, err)

	// Create Coaching
	coaching := fixture.CreateCoaching(t, &contentmodels.CoachingDto{
		Coachs:        []primitive.ObjectID{teacher.Id},
		MinSessionNum: 1,
		MaxSessionNum: 10,
	})

	// Create Coaching Pricing
	pricePerSession := 50.0
	_, err = db.Collection("academyCoachingPricings").InsertOne(ctx, contentmodels.CoachingPricing{
		Id:         primitive.NewObjectID(),
		CoachingId: coaching.Id,
		CoachingPricingDto: contentmodels.CoachingPricingDto{
			PricePerSession: pricePerSession,
			Active:          true,
		},
	})
	require.NoError(t, err)

	// Create Coupon (Fixed 10 off general coupon)
	couponCode := "SAVE10"
	discountValue := 10.0
	_, err = db.Collection("academySalesCoupons").InsertOne(ctx, salesmodels.Coupon{
		ID:                 primitive.NewObjectID(),
		Code:               couponCode,
		Status:             "active",
		Type:               "general", // Important for invoice-level coupon
		InfiniteRedemption: true,
		DiscountType:       "fixed",
		DiscountValue:      discountValue,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	})
	require.NoError(t, err)

	// 3. Cart Flow
	// Add Course to Cart
	addCourseReq := contentmodels.CartItemDto{
		Type:  contentenums.CartTypeCourse,
		RefId: course.Id,
		Data:  contentmodels.CourseSubscribe{},
	}
	w := callAPI(fixture, "POST", "/api/cart", addCourseReq)
	if !assert.Equal(t, http.StatusOK, w.Code) {
		t.Logf("Add Course Error: %s", w.Body.String())
	}

	// Add Coaching to Cart
	addCoachingReq := contentmodels.CartItemDto{
		Type:  contentenums.CartTypeCoaching,
		RefId: coaching.Id,
		Data: contentmodels.CoachingSubscribe{
			CoachingProgramId: coaching.Id,
			NumOfSessions:     2,
		},
	}
	w = callAPI(fixture, "POST", "/api/cart", addCoachingReq)
	if !assert.Equal(t, http.StatusOK, w.Code) {
		t.Logf("Add Coaching Error: %s", w.Body.String())
	}

	// Check Cart
	w = callAPI(fixture, "GET", "/api/cart", nil)
	if !assert.Equal(t, http.StatusOK, w.Code) {
		t.Logf("Get Cart Error: %s", w.Body.String())
	}
	var cart contentmodels.Cart
	err = json.Unmarshal(w.Body.Bytes(), &cart)
	require.NoError(t, err)
	assert.Len(t, cart.Items, 2)

	expectedTotalBeforeCoupon := coursePrice + (pricePerSession * 2)
	assert.Equal(t, expectedTotalBeforeCoupon, cart.TotalPrice)

	// 4. Checkout Flow
	w = callAPI(fixture, "POST", "/api/cart/checkout", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	var order contentmodels.Order
	err = json.Unmarshal(w.Body.Bytes(), &order)
	require.NoError(t, err)
	assert.Equal(t, "pending", string(order.Status))

	// Verify Invoice was created during checkout
	assert.False(t, order.InvoiceId.IsZero())
	assert.Equal(t, expectedTotalBeforeCoupon, order.TotalPrice)

	// Apply Coupon to Order (Invoice level)
	_, err = db.Collection("academyOrders").UpdateOne(ctx, bson.M{"_id": order.Id}, bson.M{"$set": bson.M{"couponCode": couponCode}})
	require.NoError(t, err)

	// 5. Payment Flow (PayOrder)
	// This will trigger Invoice creation and Price Sync
	w = callAPI(fixture, "POST", fmt.Sprintf("/api/orders/%s/pay?paymentGateway=stripe", order.Id.Hex()), nil)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify Invoice
	var updatedOrder contentmodels.Order
	err = db.Collection("academyOrders").FindOne(ctx, bson.M{"_id": order.Id}).Decode(&updatedOrder)
	require.NoError(t, err)
	assert.False(t, updatedOrder.InvoiceId.IsZero())

	var invoice salesmodels.Invoice
	err = db.Collection("academySalesInvoices").FindOne(ctx, bson.M{"_id": updatedOrder.InvoiceId}).Decode(&invoice)
	require.NoError(t, err)

	expectedTotalAfterCoupon := expectedTotalBeforeCoupon - discountValue
	assert.Equal(t, expectedTotalAfterCoupon, invoice.TotalAmount)
	assert.Equal(t, expectedTotalAfterCoupon, updatedOrder.TotalPrice) // Verify sync

	// 6. Payment Success (Mock direct call to HandlePaymentSuccess)
	// Add AppCfg to context since HandlePaymentSuccess might need it for DB operations
	payCtx := context.WithValue(ctx, util.ReqAppCfg, &types.AppCfg{
		Db: dbConfig.Database,
		User: &types.User{
			Id: student.Id,
		},
	})
	_, err = orderService.HandlePaymentSuccess(payCtx, order.Id.Hex(), "test-payment-id")
	require.NoError(t, err)

	// 7. Validation
	// Verify Order Status
	err = db.Collection("academyOrders").FindOne(ctx, bson.M{"_id": order.Id}).Decode(&updatedOrder)
	require.NoError(t, err)
	t.Logf("Debug - Order Status after enrollment: %s", updatedOrder.Status)
	// Let's check what the actual value is if it fails
	if !assert.Equal(t, "paid", strings.ToLower(string(updatedOrder.Status))) {
		t.Logf("Actual Order Status: %s", updatedOrder.Status)
	}

	// Verify Enrollments
	count, _ := db.Collection("academyCourseStudents").CountDocuments(ctx, bson.M{"studentId": student.Id, "courseId": course.Id})
	assert.Equal(t, int64(1), count)

	count, _ = db.Collection("academyCoachingStudentSessions").CountDocuments(ctx, bson.M{"studentId": student.Id, "coachingProgramId": coaching.Id})
	assert.Equal(t, int64(2), count)

	// Verify NO duplicate enrollments
	count, _ = db.Collection("academyCourseStudents").CountDocuments(ctx, bson.M{"studentId": student.Id, "courseId": course.Id})
	assert.Equal(t, int64(1), count)

	count, _ = db.Collection("academyCoachingStudentSessions").CountDocuments(ctx, bson.M{"studentId": student.Id, "coachingProgramId": coaching.Id})
	assert.Equal(t, int64(2), count)

	// Verify Payment Record
	var payment salesmodels.Payment
	err = db.Collection("academySalesPayments").FindOne(ctx, bson.M{"invoiceId": updatedOrder.InvoiceId}).Decode(&payment)
	require.NoError(t, err)
	assert.Equal(t, updatedOrder.TotalPrice, payment.Amount)
	assert.Equal(t, student.Id, payment.CustomerId)

	// 8. Idempotency Check
	// Call HandlePaymentSuccess again
	_, err = orderService.HandlePaymentSuccess(payCtx, order.Id.Hex(), "test-payment-id")
	require.NoError(t, err)

	// Verify NO duplicate enrollments
	count, _ = db.Collection("academyCourseStudents").CountDocuments(ctx, bson.M{"studentId": student.Id, "courseId": course.Id})
	assert.Equal(t, int64(1), count)

	count, _ = db.Collection("academyCoachingStudentSessions").CountDocuments(ctx, bson.M{"studentId": student.Id, "coachingProgramId": coaching.Id})
	assert.Equal(t, int64(2), count)
}

func TestOrderFlowCurrencyConversionE2E(t *testing.T) {
	// 1. Setup Environment
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	// Initialize Mock Exchange Service
	mockExchange := helpers.NewMockExchangeService()
	// Set rates: 1 USD = 0.9 EUR, 1 USD = 0.8 GBP
	// This means 1 EUR = 1.11 USD, 1 GBP = 1.25 USD
	eurRate := 0.9
	gbpRate := 0.8
	mockExchange.SetRate("EUR", eurRate)
	mockExchange.SetRate("GBP", gbpRate)

	// Create injector and override exchange service BEFORE full app setup
	injector := do.New()
	do.OverrideValue[exchange.ExchangeService](injector, mockExchange)

	injector, router, db := setup.SetupFullTestAppWithInjector(t, injector, dbConfig, authServer.URL())
	ctx := context.Background()

	fixture := &helpers.TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        ctx,
		AuthServer: authServer,
		DB:         db,
	}

	// 2. Seed Data
	student := fixture.CreateStudent(t, nil)
	fixture.SwitchAuthContext(student.Id.Hex(), []string{"authenticate"})

	teacher := fixture.CreateTeacher(t, nil)

	// Create Course priced in EUR
	course := fixture.CreateCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
		},
	})
	coursePriceEUR := 100.0
	_, err := db.Collection("academyCoursePricings").InsertOne(ctx, contentmodels.CoursePricing{
		Id:       primitive.NewObjectID(),
		CourseId: course.Id,
		CoursePricingDto: contentmodels.CoursePricingDto{
			Price:    coursePriceEUR,
			Currency: "EUR",
			Config: contentmodels.PricingConfig{
				IsDefault: true,
			},
		},
	})
	require.NoError(t, err)

	// Create Coaching priced in GBP
	coaching := fixture.CreateCoaching(t, &contentmodels.CoachingDto{
		Coachs:        []primitive.ObjectID{teacher.Id},
		MinSessionNum: 1,
		MaxSessionNum: 10,
	})
	pricePerSessionGBP := 50.0
	_, err = db.Collection("academyCoachingPricings").InsertOne(ctx, contentmodels.CoachingPricing{
		Id:         primitive.NewObjectID(),
		CoachingId: coaching.Id,
		CoachingPricingDto: contentmodels.CoachingPricingDto{
			PricePerSession: pricePerSessionGBP,
			Currency:        "GBP",
			Active:          true,
		},
	})
	require.NoError(t, err)

	// 3. Cart Flow
	// Add Course
	addCourseReq := contentmodels.CartItemDto{
		Type:  contentenums.CartTypeCourse,
		RefId: course.Id,
		Data:  contentmodels.CourseSubscribe{},
	}
	callAPI(fixture, "POST", "/api/cart", addCourseReq)

	// Add Coaching (2 sessions)
	addCoachingReq := contentmodels.CartItemDto{
		Type:  contentenums.CartTypeCoaching,
		RefId: coaching.Id,
		Data: contentmodels.CoachingSubscribe{
			CoachingProgramId: coaching.Id,
			NumOfSessions:     2,
		},
	}
	callAPI(fixture, "POST", "/api/cart", addCoachingReq)

	// Check Cart
	w := callAPI(fixture, "GET", "/api/cart", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	var cart contentmodels.Cart
	err = json.Unmarshal(w.Body.Bytes(), &cart)
	require.NoError(t, err)

	// Calculate Expected Prices in USD
	expectedCoursePriceUSD := coursePriceEUR / eurRate
	expectedCoachingPriceUSD := (pricePerSessionGBP * 2) / gbpRate
	expectedTotalUSD := expectedCoursePriceUSD + expectedCoachingPriceUSD

	assert.InDelta(t, expectedTotalUSD, cart.TotalPrice, 0.001)
	assert.Equal(t, "USD", cart.Currency)

	// 4. Checkout & Verify Order/Invoice
	w = callAPI(fixture, "POST", "/api/cart/checkout", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	var order contentmodels.Order
	err = json.Unmarshal(w.Body.Bytes(), &order)
	require.NoError(t, err)

	assert.InDelta(t, expectedTotalUSD, order.TotalPrice, 0.001)

	// Initial PayOrder to trigger invoice refresh/verification
	w = callAPI(fixture, "POST", fmt.Sprintf("/api/orders/%s/pay?paymentGateway=stripe", order.Id.Hex()), nil)
	assert.Equal(t, http.StatusOK, w.Code)

	var invoice salesmodels.Invoice
	err = db.Collection("academySalesInvoices").FindOne(ctx, bson.M{"_id": order.InvoiceId}).Decode(&invoice)
	require.NoError(t, err)

	assert.InDelta(t, expectedTotalUSD, invoice.TotalAmount, 0.001)
	assert.Equal(t, "USD", invoice.Currency)
}

func TestInvoiceRefreshFlowE2E(t *testing.T) {
	// 1. Setup Environment
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, db := setup.SetupFullTestApp(t, dbConfig, authServer.URL())
	ctx := context.Background()

	fixture := &helpers.TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        ctx,
		AuthServer: authServer,
		DB:         db,
	}

	// 2. Seed Data
	student := fixture.CreateStudent(t, nil)
	fixture.SwitchAuthContext(student.Id.Hex(), []string{"authenticate"})

	teacher := fixture.CreateTeacher(t, nil)
	course := fixture.CreateCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
		},
	})
	coursePrice := 100.0
	pricingId := primitive.NewObjectID()
	_, err := db.Collection("academyCoursePricings").InsertOne(ctx, contentmodels.CoursePricing{
		Id:       pricingId,
		CourseId: course.Id,
		CoursePricingDto: contentmodels.CoursePricingDto{
			Price: coursePrice,
			Config: contentmodels.PricingConfig{
				IsDefault: true,
			},
		},
	})
	require.NoError(t, err)

	// 3. Cart & Checkout
	addCourseReq := contentmodels.CartItemDto{
		Type:  contentenums.CartTypeCourse,
		RefId: course.Id,
		Data:  contentmodels.CourseSubscribe{},
	}
	callAPI(fixture, "POST", "/api/cart", addCourseReq)
	w := callAPI(fixture, "POST", "/api/cart/checkout", nil)
	var order contentmodels.Order
	json.Unmarshal(w.Body.Bytes(), &order)

	// 4. Initial PayOrder (Creates Invoice)
	w = callAPI(fixture, "POST", fmt.Sprintf("/api/orders/%s/pay?paymentGateway=stripe", order.Id.Hex()), nil)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify Initial Price
	var updatedOrder contentmodels.Order
	db.Collection("academyOrders").FindOne(ctx, bson.M{"_id": order.Id}).Decode(&updatedOrder)
	assert.Equal(t, coursePrice, updatedOrder.TotalPrice)

	// 5. Change Product Price in DB
	newCoursePrice := 150.0
	_, err = db.Collection("academyCoursePricings").UpdateOne(ctx, bson.M{"_id": pricingId}, bson.M{"$set": bson.M{"price": newCoursePrice}})
	require.NoError(t, err)

	// 6. Second PayOrder (Should Refresh Invoice)
	w = callAPI(fixture, "POST", fmt.Sprintf("/api/orders/%s/pay?paymentGateway=stripe", order.Id.Hex()), nil)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify Updated Price synced from Invoice
	db.Collection("academyOrders").FindOne(ctx, bson.M{"_id": order.Id}).Decode(&updatedOrder)
	assert.Equal(t, newCoursePrice, updatedOrder.TotalPrice)

	var invoice salesmodels.Invoice
	db.Collection("academySalesInvoices").FindOne(ctx, bson.M{"_id": updatedOrder.InvoiceId}).Decode(&invoice)
	assert.Equal(t, newCoursePrice, invoice.TotalAmount)
}

func callAPI(f *helpers.TestFixture, method, url string, body any) *httptest.ResponseRecorder {
	var bodyReader *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewBuffer(b)
	} else {
		bodyReader = bytes.NewBuffer([]byte{})
	}

	req := httptest.NewRequest(method, url, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)
	return w
}
