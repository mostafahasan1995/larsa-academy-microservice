package orders

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"larsa-academy-microservice/pkg/services/content/models"
	salesEnums "larsa-academy-microservice/pkg/services/sales/enums"
	salesModels "larsa-academy-microservice/pkg/services/sales/models"
	"larsa-academy-microservice/test/helpers"
	"larsa-academy-microservice/test/setup"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestOrderAccess(t *testing.T) {
	// 1. Setup Environment
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	// Add parent role to RolesMap
	parentRoleId := primitive.NewObjectID()
	authConfig.RolesMap["parent"] = parentRoleId
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
	// Create Students
	student1 := fixture.CreateStudent(t, nil)
	student2 := fixture.CreateStudent(t, nil)

	// Create Parent for student1
	parent1 := fixture.CreateParent(t, nil)
	fixture.CreateParentStudentRelation(t, parent1.Id, student1.Id)

	// Create Orders
	order1Id := primitive.NewObjectID()
	order1 := models.Order{
		Id:         order1Id,
		StudentId:  student1.Id,
		TotalPrice: 100,
		Status:     "paid",
		Trash:      false,
	}
	_, err := db.Collection("academyOrders").InsertOne(ctx, order1)
	require.NoError(t, err)

	order2Id := primitive.NewObjectID()
	order2 := models.Order{
		Id:         order2Id,
		StudentId:  student2.Id,
		TotalPrice: 200,
		Status:     "paid",
		Trash:      false,
	}
	_, err = db.Collection("academyOrders").InsertOne(ctx, order2)
	require.NoError(t, err)

	// 3. Test Cases

	t.Run("Student Can Access Own Order", func(t *testing.T) {
		fixture.SwitchAuthContext(student1.Id.Hex(), []string{"authenticate"})
		w := callAPI(fixture, "GET", fmt.Sprintf("/api/orders/%s", order1Id.Hex()), nil)
		assert.Equal(t, http.StatusOK, w.Code)

		var result models.Order
		err := json.Unmarshal(w.Body.Bytes(), &result)
		require.NoError(t, err)
		assert.Equal(t, order1Id, result.Id)
	})

	t.Run("Student Can Access Own Order List", func(t *testing.T) {
		fixture.SwitchAuthContext(student1.Id.Hex(), []string{"authenticate"})
		w := callAPI(fixture, "POST", fmt.Sprintf("/api/orders/student/%s", student1.Id.Hex()), nil)
		assert.Equal(t, http.StatusOK, w.Code)

		var result models.OrderPagination
		err := json.Unmarshal(w.Body.Bytes(), &result)
		require.NoError(t, err)
		assert.Len(t, result.Items, 1)
		assert.Equal(t, order1Id, result.Items[0].Id)
	})

	t.Run("Parent Can Access Child Order", func(t *testing.T) {
		fixture.SwitchAuthContext(parent1.Id.Hex(), []string{"authenticate"})
		w := callAPI(fixture, "GET", fmt.Sprintf("/api/orders/%s", order1Id.Hex()), nil)
		assert.Equal(t, http.StatusOK, w.Code)

		var result models.Order
		err := json.Unmarshal(w.Body.Bytes(), &result)
		require.NoError(t, err)
		assert.Equal(t, order1Id, result.Id)
	})

	t.Run("Parent Can Access Child Order List", func(t *testing.T) {
		fixture.SwitchAuthContext(parent1.Id.Hex(), []string{"authenticate"})
		w := callAPI(fixture, "POST", fmt.Sprintf("/api/orders/student/%s", student1.Id.Hex()), nil)
		assert.Equal(t, http.StatusOK, w.Code)

		var result models.OrderPagination
		err := json.Unmarshal(w.Body.Bytes(), &result)
		require.NoError(t, err)
		assert.Len(t, result.Items, 1)
		assert.Equal(t, order1Id, result.Items[0].Id)
	})

	t.Run("Student Cannot Access Other Student Order", func(t *testing.T) {
		fixture.SwitchAuthContext(student1.Id.Hex(), []string{"authenticate"})
		w := callAPI(fixture, "GET", fmt.Sprintf("/api/orders/%s", order2Id.Hex()), nil)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("Student Cannot Access Other Student Order List", func(t *testing.T) {
		fixture.SwitchAuthContext(student1.Id.Hex(), []string{"authenticate"})
		w := callAPI(fixture, "POST", fmt.Sprintf("/api/orders/student/%s", student2.Id.Hex()), nil)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("Parent Cannot Access Unrelated Student Order", func(t *testing.T) {
		fixture.SwitchAuthContext(parent1.Id.Hex(), []string{"authenticate"})
		w := callAPI(fixture, "GET", fmt.Sprintf("/api/orders/%s", order2Id.Hex()), nil)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("Random User Cannot Access Any Order", func(t *testing.T) {
		randomUserId := primitive.NewObjectID()
		fixture.SwitchAuthContext(randomUserId.Hex(), []string{"authenticate"})
		w := callAPI(fixture, "GET", fmt.Sprintf("/api/orders/%s", order1Id.Hex()), nil)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

func TestOrderPaymentDataFromInvoice(t *testing.T) {
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authConfig.RolesMap["parent"] = primitive.NewObjectID()
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

	student := fixture.CreateStudent(t, nil)
	invoiceId := primitive.NewObjectID()
	orderId := primitive.NewObjectID()

	// Invoice is source of truth: total 99.5, currency EUR (e.g. after coupon)
	_, err := db.Collection("academySalesInvoices").InsertOne(ctx, salesModels.Invoice{
		ID:          invoiceId,
		InvoiceNum:  "INV-TEST-001",
		CustomerID:  student.Id,
		TotalAmount: 99.5,
		Currency:    "EUR",
		Status:      salesEnums.InvoiceStatusPaid,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	})
	require.NoError(t, err)

	// Order document has stale total (e.g. 100); API should return invoice total
	_, err = db.Collection("academyOrders").InsertOne(ctx, models.Order{
		Id:         orderId,
		StudentId:  student.Id,
		InvoiceId:  invoiceId,
		TotalPrice: 100, // stale
		Status:     "paid",
		Trash:      false,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	})
	require.NoError(t, err)

	fixture.SwitchAuthContext(student.Id.Hex(), []string{"authenticate"})

	t.Run("GET order by id returns invoice total and currency", func(t *testing.T) {
		w := callAPI(fixture, "GET", fmt.Sprintf("/api/orders/%s", orderId.Hex()), nil)
		require.Equal(t, http.StatusOK, w.Code)
		var result models.Order
		err := json.Unmarshal(w.Body.Bytes(), &result)
		require.NoError(t, err)
		assert.Equal(t, 99.5, result.TotalPrice, "totalPrice should come from invoice")
		assert.Equal(t, "EUR", result.Currency)
	})

	t.Run("POST student orders list returns invoice total and currency", func(t *testing.T) {
		w := callAPI(fixture, "POST", fmt.Sprintf("/api/orders/student/%s", student.Id.Hex()), nil)
		require.Equal(t, http.StatusOK, w.Code)
		var result models.OrderPagination
		err := json.Unmarshal(w.Body.Bytes(), &result)
		require.NoError(t, err)
		require.Len(t, result.Items, 1)
		assert.Equal(t, 99.5, result.Items[0].TotalPrice, "totalPrice should come from invoice")
		assert.Equal(t, "EUR", result.Items[0].Currency)
	})
}

func callAPI(f *helpers.TestFixture, method, url string, body any) *httptest.ResponseRecorder {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, _ := json.Marshal(body)
		bodyReader = bytes.NewBuffer(jsonBody)
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
