package content

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/test/setup"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestBundleAuthorization_AdminCanViewAllBundles(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authConfig.Capabilities = []string{"authenticate", "ManageAllBundles"}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, database := setup.SetupFullTestApp(t, dbConfig, authServer.URL())

	fixture := &TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        context.Background(),
		AuthServer: authServer,
		DB:         database,
	}

	// Create test data
	bundle := fixture.createBundle(t, &models.BundleDto{
		Name:   transl.Localizable[string]{"en": "Test Bundle"},
		Status: "active",
	})

	// Switch to admin context with ViewAllBundles capability
	adminUserId := primitive.NewObjectID().Hex()
	fixture.switchAuthContext(adminUserId, []string{"authenticate", "ViewAllBundles"})
	w := fixture.callGetBundleById(t, bundle.Id.Hex())

	require.Equal(t, http.StatusOK, w.Code, "Admin should be able to view bundle: %s", w.Body.String())
	var response models.Bundle
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, bundle.Id, response.Id)
}

func TestBundleAuthorization_StudentCanViewPublicBundles(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authConfig.Capabilities = []string{"authenticate"}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, database := setup.SetupFullTestApp(t, dbConfig, authServer.URL())

	fixture := &TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        context.Background(),
		AuthServer: authServer,
		DB:         database,
	}

	activeBundle := fixture.createBundle(t, &models.BundleDto{
		Name:   transl.Localizable[string]{"en": "Active Bundle"},
		Status: "active",
	})
	draftBundle := fixture.createBundle(t, &models.BundleDto{
		Name:   transl.Localizable[string]{"en": "Draft Bundle"},
		Status: "draft",
	})

	studentId := primitive.NewObjectID().Hex()

	// Test without ViewPublicBundles permission
	fixture.switchAuthContext(studentId, []string{"authenticate"})
	w := fixture.callGetBundleById(t, activeBundle.Id.Hex())
	assert.Equal(t, http.StatusForbidden, w.Code, "Student without permissions should not view active bundle")

	// Test with ViewPublicBundles permission
	fixture.switchAuthContext(studentId, []string{"authenticate", "ViewPublicBundles"})

	w = fixture.callGetBundleById(t, activeBundle.Id.Hex())
	assert.Equal(t, http.StatusOK, w.Code, "Student with ViewPublicBundles should view active bundle")

	w = fixture.callGetBundleById(t, draftBundle.Id.Hex())
	assert.Equal(t, http.StatusForbidden, w.Code, "Student with ViewPublicBundles should NOT view draft bundle")
}

func TestBundleAuthorization_StudentCanViewEnrolledBundles(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, database := setup.SetupFullTestApp(t, dbConfig, authServer.URL())

	fixture := &TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        context.Background(),
		AuthServer: authServer,
		DB:         database,
	}

	draftBundle := fixture.createBundle(t, &models.BundleDto{
		Name:   transl.Localizable[string]{"en": "Draft Bundle"},
		Status: "draft",
	})

	studentId := primitive.NewObjectID().Hex()
	studentObjId, _ := primitive.ObjectIDFromHex(studentId)

	// Enroll student in draft bundle
	fixture.enrollStudentInBundle(t, studentObjId, draftBundle.Id)

	// Test enrolled student can view even if not public
	fixture.switchAuthContext(studentId, []string{"authenticate", "ViewPublicBundles"})
	w := fixture.callGetBundleById(t, draftBundle.Id.Hex())
	assert.Equal(t, http.StatusOK, w.Code, "Enrolled student should be able to view their bundle even if it's draft")
}

func TestBundleAuthorization_ManageBundlePricing(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, database := setup.SetupFullTestApp(t, dbConfig, authServer.URL())

	fixture := &TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        context.Background(),
		AuthServer: authServer,
		DB:         database,
	}

	bundle := fixture.createBundle(t, &models.BundleDto{
		Name:   transl.Localizable[string]{"en": "Price Test Bundle"},
		Status: "active",
	})

	pricingDto := &models.BundlePricingDto{
		Name:     transl.Localizable[string]{"en": "Standard"},
		PlanType: "one-time payment",
		Price:    100,
		Currency: "USD",
		Active:   true,
	}

	// User without ManageBundlePricing
	fixture.switchAuthContext(primitive.NewObjectID().Hex(), []string{"authenticate", "ViewAllBundles"})
	w := callAddBundlePricing(t, fixture, bundle.Id.Hex(), pricingDto)
	require.Equal(t, http.StatusForbidden, w.Code)

	// User with ManageBundlePricing
	fixture.switchAuthContext(primitive.NewObjectID().Hex(), []string{"authenticate", "ManageBundlePricing"})
	w = callAddBundlePricing(t, fixture, bundle.Id.Hex(), pricingDto)
	assert.Equal(t, http.StatusOK, w.Code)
}

func callAddBundlePricing(t *testing.T, fixture *TestFixture, bundleId string, dto *models.BundlePricingDto) *httptest.ResponseRecorder {
	body, _ := json.Marshal(dto)
	url := "/bundles/" + bundleId + "/pricing"
	req := httptest.NewRequest("POST", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", fixture.DBName)
	req = req.WithContext(fixture.Ctx)
	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)
	return w
}

func (f *TestFixture) callGetBundleById(t *testing.T, bundleId string) *httptest.ResponseRecorder {
	url := fmt.Sprintf("/bundles/%s", bundleId)
	req := httptest.NewRequest("GET", url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)
	return w
}
