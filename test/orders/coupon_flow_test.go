package orders

import (
	"context"
	"encoding/json"
	"fmt"
	contentenums "larsa-academy-microservice/pkg/services/content/enums"
	contentmodels "larsa-academy-microservice/pkg/services/content/models"
	salesmodels "larsa-academy-microservice/pkg/services/sales/models"
	"larsa-academy-microservice/test/helpers"
	"larsa-academy-microservice/test/setup"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestCouponFlowE2E(t *testing.T) {
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

	couponCode := "DISCOUNT50"
	discountValue := 50.0
	_, err = db.Collection("academySalesCoupons").InsertOne(ctx, salesmodels.Coupon{
		ID:                 primitive.NewObjectID(),
		Code:               couponCode,
		Status:             "active",
		Type:               "general",
		InfiniteRedemption: true,
		DiscountType:       "fixed",
		DiscountValue:      discountValue,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	})
	require.NoError(t, err)

	// 3. Cart Coupon Flow
	// Add item to cart
	addCourseReq := contentmodels.CartItemDto{
		Type:  contentenums.CartTypeCourse,
		RefId: course.Id,
		Data:  contentmodels.CourseSubscribe{},
	}
	callAPI(fixture, "POST", "/api/cart", addCourseReq)

	// Verify cart price before coupon
	w := callAPI(fixture, "GET", "/api/cart", nil)
	var cart contentmodels.Cart
	json.Unmarshal(w.Body.Bytes(), &cart)
	assert.Equal(t, coursePrice, cart.TotalPrice)
	assert.Equal(t, 0.0, cart.Discount)

	// Apply coupon to cart
	applyReq := struct {
		Code string `json:"code"`
	}{Code: couponCode}
	w = callAPI(fixture, "POST", "/api/cart/coupon", applyReq)
	assert.Equal(t, http.StatusOK, w.Code)
	json.Unmarshal(w.Body.Bytes(), &cart)
	assert.Equal(t, coursePrice-discountValue, cart.TotalPrice)
	assert.Equal(t, discountValue, cart.Discount)
	assert.Equal(t, couponCode, cart.CouponCode)

	// Remove coupon from cart
	w = callAPI(fixture, "DELETE", "/api/cart/coupon", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	json.Unmarshal(w.Body.Bytes(), &cart)
	assert.Equal(t, coursePrice, cart.TotalPrice)
	assert.Equal(t, 0.0, cart.Discount)
	assert.Empty(t, cart.CouponCode)

	// Re-apply coupon for checkout
	callAPI(fixture, "POST", "/api/cart/coupon", applyReq)

	// 4. Checkout Flow
	w = callAPI(fixture, "POST", "/api/cart/checkout", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	var order contentmodels.Order
	json.Unmarshal(w.Body.Bytes(), &order)
	assert.Equal(t, couponCode, order.CouponCode)
	assert.Equal(t, discountValue, order.Discount)
	assert.Equal(t, coursePrice-discountValue, order.TotalPrice)

	// 5. Order Coupon Flow
	// Create another order without coupon
	callAPI(fixture, "POST", "/api/cart", addCourseReq)
	w = callAPI(fixture, "POST", "/api/cart/checkout", nil)
	var order2 contentmodels.Order
	json.Unmarshal(w.Body.Bytes(), &order2)
	assert.Empty(t, order2.CouponCode)

	// Apply coupon to order2
	w = callAPI(fixture, "POST", fmt.Sprintf("/api/orders/%s/coupon", order2.Id.Hex()), applyReq)
	assert.Equal(t, http.StatusOK, w.Code)
	var updatedOrder2 contentmodels.Order
	json.Unmarshal(w.Body.Bytes(), &updatedOrder2)
	assert.Equal(t, couponCode, updatedOrder2.CouponCode)
	assert.Equal(t, discountValue, updatedOrder2.Discount)
	assert.InDelta(t, coursePrice-discountValue, updatedOrder2.TotalPrice, 0.01)

	// Verify Invoice associated with order2
	assert.False(t, updatedOrder2.InvoiceId.IsZero())
	var invoice salesmodels.Invoice
	err = db.Collection("academySalesInvoices").FindOne(ctx, bson.M{"_id": updatedOrder2.InvoiceId}).Decode(&invoice)
	require.NoError(t, err)
	assert.Equal(t, couponCode, invoice.CouponCode)
	assert.Equal(t, coursePrice-discountValue, invoice.TotalAmount)
}
