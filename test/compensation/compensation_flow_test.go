package compensation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"larsa-academy-microservice/pkg/services/compensation"
	compmodels "larsa-academy-microservice/pkg/services/compensation/models"
	contentmodels "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/test/helpers"
	"larsa-academy-microservice/test/setup"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// helper to build a fixture using shared helpers
func newFixture(t *testing.T, capabilities []string) *helpers.TestFixture {
	dbConfig := setup.SetupTestDB(t)
	t.Cleanup(dbConfig.Cleanup)

	authConfig := setup.DefaultMockAuthConfig()
	authConfig.Capabilities = capabilities
	authServer := setup.NewMockAuthServer(authConfig)
	t.Cleanup(authServer.Close)

	_, router, database := setup.SetupFullTestApp(t, dbConfig, authServer.URL())

	return &helpers.TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        context.Background(),
		AuthServer: authServer,
		DB:         database,
	}
}

func TestCompensationFlow_RecordAndSummarize(t *testing.T) {
	fixture := newFixture(t, []string{
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
			Level:          "beginner",
			Teachers:       []primitive.ObjectID{teacher.Id},
			LinkedSubjects: []primitive.ObjectID{subject.Id},
			SchoolType:     "public",
			Country:        "US",
		},
	})

	pricingBody, _ := json.Marshal(compmodels.TeacherPricingCreateDto{
		Criteria: compmodels.PricingCriteriaDto{
			SubjectId:  subject.Id.Hex(),
			SchoolType: "public",
			Country:    "US",
			Level:      "beginner",
		},
		HourlyRate: 80,
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

	occurredAt := time.Now().UTC()
	hoursBody, _ := json.Marshal(compmodels.TeacherHoursCreateDto{
		Ref:             "course",
		RefId:           course.Id.Hex(),
		OccurredAt:      occurredAt,
		DurationMinutes: 90, // 1.5 hours
	})
	req = httptest.NewRequest("POST", "/compensation/teachers/"+teacher.Id.Hex()+"/hours", bytes.NewBuffer(hoursBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", fixture.DBName)
	req = req.WithContext(fixture.Ctx)
	w = httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "hours create failed: %s", w.Body.String())

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
	require.Equal(t, http.StatusOK, w.Code, "summary failed: %s", w.Body.String())

	var resp struct {
		Items []compmodels.TeacherCompensationSummary `json:"items"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Items, 1)
	s := resp.Items[0]
	require.InDelta(t, 1.5, s.TotalHours, 0.001)
	require.InDelta(t, 120.0, s.TotalAmountByCurrency["USD"], 0.01)
	require.Len(t, s.PricingSummaries, 1)
	require.Equal(t, subject.Id, s.PricingSummaries[0].Criteria.SubjectId)
	require.InDelta(t, 1.5, s.PricingSummaries[0].HoursRecorded, 0.001)
	require.InDelta(t, 120.0, s.PricingSummaries[0].TotalAmount, 0.01)
}

func TestCompensationFlow_ForbiddenWithoutCapability(t *testing.T) {
	fixture := newFixture(t, []string{"authenticate"}) // no manage capability

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

	pricingBody, _ := json.Marshal(compmodels.TeacherPricingCreateDto{
		Criteria: compmodels.PricingCriteriaDto{
			SubjectId:  subject.Id.Hex(),
			SchoolType: "public",
			Country:    "US",
			Level:      "grade-10",
		},
		HourlyRate: 50,
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

	require.Equal(t, http.StatusForbidden, w.Code, "expected forbidden without capability")
}

func pricingIDFromResponseOrDB(t *testing.T, fixture *helpers.TestFixture, teacherID, subjectID primitive.ObjectID, schoolType, country, level string, body []byte) primitive.ObjectID {
	t.Helper()

	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err == nil {
		idHex := ""
		switch v := raw["_id"].(type) {
		case string:
			idHex = v
		case map[string]any:
			if oid, ok := v["$oid"].(string); ok {
				idHex = oid
			}
		}

		if idHex != "" {
			oid, err := primitive.ObjectIDFromHex(idHex)
			require.NoError(t, err, "invalid pricing id hex in response")
			if !oid.IsZero() {
				return oid
			}
		}
	}

	// Fallback: read the newly created pricing directly from the database.
	var pricing compmodels.TeacherPricing
	filter := bson.M{
		"teacherId":           teacherID,
		"criteria.subjectId":  subjectID,
		"criteria.schoolType": schoolType,
		"criteria.country":    country,
		"criteria.level":      level,
		"trash":               false,
	}
	err := fixture.DB.Collection(pricing.CollName()).FindOne(fixture.Ctx, filter).Decode(&pricing)
	require.NoError(t, err, "failed to load pricing from db fallback")
	require.False(t, pricing.Id.IsZero(), "pricing id missing from db record")
	return pricing.Id
}

func TestCompensationFlow_DeletePricing_SucceedsWhenUnused(t *testing.T) {
	fixture := newFixture(t, []string{
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

	// Create course to satisfy pricing criteria, though we won't record hours.
	_ = fixture.CreateCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers:       []primitive.ObjectID{teacher.Id},
			LinkedSubjects: []primitive.ObjectID{subject.Id},
			SchoolType:     "public",
			Country:        "US",
			Level:          "beginner",
		},
	})

	// Create pricing
	pricingBody, _ := json.Marshal(compmodels.TeacherPricingCreateDto{
		Criteria: compmodels.PricingCriteriaDto{
			SubjectId:  subject.Id.Hex(),
			SchoolType: "public",
			Country:    "US",
			Level:      "beginner",
		},
		HourlyRate: 60,
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

	pricingID := pricingIDFromResponseOrDB(t, fixture, teacher.Id, subject.Id, "public", "US", "beginner", w.Body.Bytes())
	require.False(t, pricingID.IsZero(), "resolved pricingId should not be zero")

	// Delete pricing (no hours reference it)
	req = httptest.NewRequest("DELETE", "/compensation/teachers/"+teacher.Id.Hex()+"/pricing/"+pricingID.Hex(), nil)
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", fixture.DBName)
	req = req.WithContext(fixture.Ctx)
	w = httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "pricing delete failed: %s", w.Body.String())

	var stored compmodels.TeacherPricing
	err = fixture.DB.Collection(stored.CollName()).FindOne(fixture.Ctx, bson.M{"_id": pricingID}).Decode(&stored)
	require.NoError(t, err)
	require.True(t, stored.Trash, "pricing should be soft-deleted")
}

func TestCompensationFlow_DeletePricing_BlockedWhenReferenced(t *testing.T) {
	fixture := newFixture(t, []string{
		"authenticate",
		compensation.CapabilityManageTeacherCompensation,
		compensation.CapabilityViewTeacherCompensation,
	})

	teacher := fixture.CreateTeacher(t, nil)
	subject := contentmodels.Subject{
		Id: primitive.NewObjectID(),
		SubjectDto: contentmodels.SubjectDto{
			Name: transl.Localizable[string]{"en": "Physics"},
			Code: "PHY",
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
			Level:          "intermediate",
		},
	})

	// Create pricing
	pricingBody, _ := json.Marshal(compmodels.TeacherPricingCreateDto{
		Criteria: compmodels.PricingCriteriaDto{
			SubjectId:  subject.Id.Hex(),
			SchoolType: "public",
			Country:    "US",
			Level:      "intermediate",
		},
		HourlyRate: 70,
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

	pricingID := pricingIDFromResponseOrDB(t, fixture, teacher.Id, subject.Id, "public", "US", "intermediate", w.Body.Bytes())
	require.False(t, pricingID.IsZero(), "resolved pricingId should not be zero")

	// Record hours to create reference
	occurredAt := time.Now().UTC()
	hoursBody, _ := json.Marshal(compmodels.TeacherHoursCreateDto{
		Ref:             "course",
		RefId:           course.Id.Hex(),
		OccurredAt:      occurredAt,
		DurationMinutes: 60,
	})
	req = httptest.NewRequest("POST", "/compensation/teachers/"+teacher.Id.Hex()+"/hours", bytes.NewBuffer(hoursBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", fixture.DBName)
	req = req.WithContext(fixture.Ctx)
	w = httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "hours create failed: %s", w.Body.String())

	// Try delete pricing; expect blocked
	req = httptest.NewRequest("DELETE", "/compensation/teachers/"+teacher.Id.Hex()+"/pricing/"+pricingID.Hex(), nil)
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", fixture.DBName)
	req = req.WithContext(fixture.Ctx)
	w = httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code, "pricing delete should be blocked when referenced")

	// Ensure still not trashed
	var stored compmodels.TeacherPricing
	err = fixture.DB.Collection(stored.CollName()).FindOne(fixture.Ctx, bson.M{"_id": pricingID}).Decode(&stored)
	require.NoError(t, err)
	require.False(t, stored.Trash, "pricing should remain active when delete is blocked")
}
