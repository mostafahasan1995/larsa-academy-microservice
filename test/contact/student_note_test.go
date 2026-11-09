package contact

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"larsa-academy-microservice/pkg/services/contact"
	"larsa-academy-microservice/pkg/services/contact/models"
	"larsa-academy-microservice/test/helpers"
	"larsa-academy-microservice/test/setup"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestStudentNoteAuthorization(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	injector, router, _ := setup.SetupFullTestApp(t, dbConfig, authServer.URL())
	_ = injector

	fixture := &helpers.TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        context.Background(),
		AuthServer: authServer,
		DB:         dbConfig.Client.Database(dbConfig.Database),
	}

	// Create two students
	// Note: CreateStudent uses the router, so it will use the current auth context
	student1 := fixture.CreateStudent(t, nil)
	student2 := fixture.CreateStudent(t, nil)

	student1Id := student1.Id.Hex()
	student2Id := student2.Id.Hex()

	t.Run("Student can CRUD their own notes", func(t *testing.T) {
		// Switch to Student 1 context
		fixture.SwitchAuthContext(student1Id, []string{"authenticate"})

		// 1. Create note
		createDto := models.StudentNoteCreateDto{
			Title:   "My Note",
			Content: "This is my private note",
			Tags:    []string{"personal"},
		}
		body, _ := json.Marshal(createDto)
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/students/%s/notes", student1Id), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-access-token", "test-token")
		req.Header.Set("x-client", fixture.DBName)
		w := httptest.NewRecorder()
		fixture.Router.ServeHTTP(w, req)

		require.Equal(t, http.StatusCreated, w.Code)
		var createdNote models.StudentNote
		err := json.Unmarshal(w.Body.Bytes(), &createdNote)
		require.NoError(t, err)
		assert.Equal(t, student1.Id, createdNote.StudentId)

		noteId := createdNote.Id.Hex()

		// 2. Get note
		req = httptest.NewRequest("GET", fmt.Sprintf("/api/students/%s/notes/%s", student1Id, noteId), nil)
		req.Header.Set("x-access-token", "test-token")
		req.Header.Set("x-client", fixture.DBName)
		w = httptest.NewRecorder()
		fixture.Router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		// 3. Update note
		updateTitle := "Updated Title"
		updateDto := models.StudentNoteUpdateDto{
			Title: &updateTitle,
		}
		body, _ = json.Marshal(updateDto)
		req = httptest.NewRequest("PATCH", fmt.Sprintf("/api/students/%s/notes/%s", student1Id, noteId), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-access-token", "test-token")
		req.Header.Set("x-client", fixture.DBName)
		w = httptest.NewRecorder()
		fixture.Router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		// 4. Delete note
		req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/students/%s/notes/%s", student1Id, noteId), nil)
		req.Header.Set("x-access-token", "test-token")
		req.Header.Set("x-client", fixture.DBName)
		w = httptest.NewRecorder()
		fixture.Router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Student cannot access another student's notes", func(t *testing.T) {
		// First, Admin creates a note for Student 2
		fixture.SwitchAuthContext(primitive.NewObjectID().Hex(), []string{"authenticate", contact.CapabilityViewStudentNotes})
		createDto := models.StudentNoteCreateDto{
			Title:   "Student 2 Note",
			Content: "Secret info",
		}
		body, _ := json.Marshal(createDto)
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/students/%s/notes", student2Id), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-access-token", "test-token")
		req.Header.Set("x-client", fixture.DBName)
		w := httptest.NewRecorder()
		fixture.Router.ServeHTTP(w, req)
		require.Equal(t, http.StatusCreated, w.Code)

		var note2 models.StudentNote
		json.Unmarshal(w.Body.Bytes(), &note2)
		note2Id := note2.Id.Hex()

		// Now switch to Student 1 and try to access Note 2
		fixture.SwitchAuthContext(student1Id, []string{"authenticate"})

		// Try to GET Note 2
		req = httptest.NewRequest("GET", fmt.Sprintf("/api/students/%s/notes/%s", student2Id, note2Id), nil)
		req.Header.Set("x-access-token", "test-token")
		req.Header.Set("x-client", fixture.DBName)
		w = httptest.NewRecorder()
		fixture.Router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "access denied")

		// Try to list Student 2 notes
		req = httptest.NewRequest("POST", fmt.Sprintf("/api/students/%s/notes/list", student2Id), bytes.NewBufferString("{}"))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-access-token", "test-token")
		req.Header.Set("x-client", fixture.DBName)
		w = httptest.NewRecorder()
		fixture.Router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "access denied")
	})

	t.Run("Admin with capability can access any student's notes", func(t *testing.T) {
		// Admin creates a note for Student 1
		fixture.SwitchAuthContext(primitive.NewObjectID().Hex(), []string{"authenticate", contact.CapabilityViewStudentNotes})

		createDto := models.StudentNoteCreateDto{
			Title:   "Admin Note for Student 1",
			Content: "Admin observations",
		}
		body, _ := json.Marshal(createDto)
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/students/%s/notes", student1Id), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-access-token", "test-token")
		req.Header.Set("x-client", fixture.DBName)
		w := httptest.NewRecorder()
		fixture.Router.ServeHTTP(w, req)
		require.Equal(t, http.StatusCreated, w.Code)

		var adminNote models.StudentNote
		json.Unmarshal(w.Body.Bytes(), &adminNote)
		adminNoteId := adminNote.Id.Hex()

		// Admin can GET it
		req = httptest.NewRequest("GET", fmt.Sprintf("/api/students/%s/notes/%s", student1Id, adminNoteId), nil)
		req.Header.Set("x-access-token", "test-token")
		req.Header.Set("x-client", fixture.DBName)
		w = httptest.NewRecorder()
		fixture.Router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// Admin can List Student 1 notes
		req = httptest.NewRequest("POST", fmt.Sprintf("/api/students/%s/notes/list", student1Id), bytes.NewBufferString("{}"))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-access-token", "test-token")
		req.Header.Set("x-client", fixture.DBName)
		w = httptest.NewRecorder()
		fixture.Router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("User without capability cannot access notes", func(t *testing.T) {
		fixture.SwitchAuthContext(primitive.NewObjectID().Hex(), []string{"authenticate"})

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/students/%s/notes/list", student1Id), bytes.NewBufferString("{}"))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-access-token", "test-token")
		req.Header.Set("x-client", fixture.DBName)
		w := httptest.NewRecorder()
		fixture.Router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "access denied")
	})
}
