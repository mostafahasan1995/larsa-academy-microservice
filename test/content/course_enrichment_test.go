package content

import (
	"context"
	"encoding/json"
	"fmt"
	contentmodels "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/test/setup"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestCourseEnrichment_GetEnriched(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authConfig.Capabilities = []string{"authenticate", "ManageAllCourses", "ViewAllCourses"}
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

	// 1. Create Subjects
	subject1 := contentmodels.Subject{
		Id: primitive.NewObjectID(),
		SubjectDto: contentmodels.SubjectDto{
			Name: transl.Localizable[string]{"en": "Mathematics"},
			Code: "MATH101",
		},
	}
	subject2 := contentmodels.Subject{
		Id: primitive.NewObjectID(),
		SubjectDto: contentmodels.SubjectDto{
			Name: transl.Localizable[string]{"en": "Science"},
			Code: "SCI101",
		},
	}
	_, err := database.Collection("academySubjects").InsertMany(fixture.Ctx, []interface{}{subject1, subject2})
	require.NoError(t, err)

	// 2. Create Course linked to Subjects
	teacher := fixture.createTeacher(t, nil)
	course := fixture.createCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Name:           transl.Localizable[string]{"en": "Enriched Course"},
			Teachers:       []primitive.ObjectID{teacher.Id},
			LinkedSubjects: []primitive.ObjectID{subject1.Id, subject2.Id},
			Status:         "active",
		},
	})

	// 3. Enroll students
	student1 := fixture.createStudent(t, nil)
	student2 := fixture.createStudent(t, nil)
	fixture.enrollStudentInCourse(t, student1.Id, course.Id, "active", 50)
	fixture.enrollStudentInCourse(t, student2.Id, course.Id, "active", 20)

	t.Run("Enrich Subjects and Student Count", func(t *testing.T) {
		url := fmt.Sprintf("/courses/list?enrichSubjects=true&includeStudentCount=true")
		body := bson.M{
			"columns": []bson.M{
				{
					"name":  "basicInfo.status",
					"exp":   "eq",
					"value": "active",
				},
			},
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", url, strings.NewReader(string(jsonBody)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-access-token", "test-token")
		req.Header.Set("x-client", fixture.DBName)
		req = req.WithContext(fixture.Ctx)

		w := httptest.NewRecorder()
		fixture.Router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Response body: %s", w.Body.String())
		}
		require.Equal(t, http.StatusOK, w.Code)

		var result contentmodels.CoursePagination
		err := json.Unmarshal(w.Body.Bytes(), &result)
		require.NoError(t, err)
		require.Len(t, result.Courses, 1)

		enrichedCourse := result.Courses[0]

		// Verify Subjects
		assert.Len(t, enrichedCourse.Subjects, 2)
		subjectIds := []primitive.ObjectID{enrichedCourse.Subjects[0].Id, enrichedCourse.Subjects[1].Id}
		assert.Contains(t, subjectIds, subject1.Id)
		assert.Contains(t, subjectIds, subject2.Id)

		// Verify Student Count
		require.NotNil(t, enrichedCourse.StudentCount)
		assert.Equal(t, int64(2), *enrichedCourse.StudentCount)
	})

	t.Run("No Enrichment", func(t *testing.T) {
		url := "/courses/list"
		body := bson.M{
			"columns": []bson.M{
				{
					"name":  "basicInfo.status",
					"exp":   "eq",
					"value": "active",
				},
			},
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", url, strings.NewReader(string(jsonBody)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-access-token", "test-token")
		req.Header.Set("x-client", fixture.DBName)
		req = req.WithContext(fixture.Ctx)

		w := httptest.NewRecorder()
		fixture.Router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Response body: %s", w.Body.String())
		}
		require.Equal(t, http.StatusOK, w.Code)

		var result contentmodels.CoursePagination
		err := json.Unmarshal(w.Body.Bytes(), &result)
		require.NoError(t, err)
		require.Len(t, result.Courses, 1)

		courseResult := result.Courses[0]
		assert.Empty(t, courseResult.Subjects)
		assert.Nil(t, courseResult.StudentCount)
	})
}
