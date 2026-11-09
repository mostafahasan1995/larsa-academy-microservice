package helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"larsa-academy-microservice/pkg/services/contact/enums"
	"larsa-academy-microservice/pkg/services/contact/models"
	contentmodels "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/test/setup"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CreateTeacher creates a teacher with random values for missing fields
func (f *TestFixture) CreateTeacher(t *testing.T, dto *models.TeacherDto) models.Teacher {
	if dto == nil {
		dto = &models.TeacherDto{}
	}

	if len(dto.FirstName) == 0 {
		dto.FirstName = transl.Localizable[string]{
			"en": "Teacher" + RandomString(4),
		}
	}
	if len(dto.LastName) == 0 {
		dto.LastName = transl.Localizable[string]{
			"en": "Test" + RandomString(4),
		}
	}
	if dto.Email == "" {
		dto.Email = RandomEmail()
	}
	if dto.Mobile == "" {
		dto.Mobile = RandomPhone()
	}
	if dto.NationalID == "" {
		dto.NationalID = RandomNationalID()
	}
	if dto.Gender == "" {
		genders := []string{"male", "female"}
		dto.Gender = genders[RandomInt(0, len(genders)-1)]
	}
	if dto.Nationality == "" {
		dto.Nationality = "US"
	}
	if dto.EmploymentStatus == "" {
		dto.EmploymentStatus = "full-time"
	}
	if dto.DateOfBirth.IsZero() {
		dto.DateOfBirth = time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, RandomInt(0, 365*10))
	}
	if dto.HireDate.IsZero() {
		dto.HireDate = time.Now().AddDate(0, 0, -RandomInt(1, 365))
	}
	if dto.JoiningDate.IsZero() {
		dto.JoiningDate = time.Now().AddDate(0, 0, -RandomInt(1, 365))
	}

	if dto.AccountSettings.Email == "" {
		dto.AccountSettings.Email = dto.Email
	}
	if dto.AccountSettings.InitialPass == "" {
		dto.AccountSettings.InitialPass = "password123"
	}
	if dto.AccountSettings.ConfirmPass == "" {
		dto.AccountSettings.ConfirmPass = dto.AccountSettings.InitialPass
	}
	if dto.AccountSettings.JoinStatus == "" {
		dto.AccountSettings.JoinStatus = "active"
	}
	if !dto.AccountSettings.IsActive {
		dto.AccountSettings.IsActive = true
	}

	if len(dto.Availability.DefaultWorkingHours) == 0 {
		dto.Availability.DefaultWorkingHours = []models.Shift{
			{From: "09:00", To: "17:00"},
		}
	}
	if dto.Availability.Scheduling.SessionDuration.Value == 0 {
		dto.Availability.Scheduling.SessionDuration = models.SchedulingUnit{
			Value: 60,
			Unit:  "minutes",
		}
	}

	body, _ := json.Marshal(dto)
	req := httptest.NewRequest("POST", "/teachers", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)

	if !(w.Code == http.StatusCreated || w.Code == http.StatusOK) {
		t.Fatalf("Failed to create teacher: status %d, body: %s", w.Code, w.Body.String())
	}

	var teacher models.Teacher
	if err := json.Unmarshal(w.Body.Bytes(), &teacher); err != nil {
		t.Fatalf("Failed to unmarshal teacher: %v", err)
	}
	return teacher
}

// CreateStudent creates a student with random values for missing fields
func (f *TestFixture) CreateStudent(t *testing.T, dto *models.StudentDto) models.Student {
	if dto == nil {
		dto = &models.StudentDto{}
	}

	if len(dto.GeneralInfo.FirstName) == 0 {
		dto.GeneralInfo.FirstName = transl.Localizable[string]{
			"ar": "طالب",
			"en": "Student" + RandomString(4),
		}
	}
	if len(dto.GeneralInfo.LastName) == 0 {
		dto.GeneralInfo.LastName = transl.Localizable[string]{
			"ar": "اختبار",
			"en": "Test" + RandomString(4),
		}
	}
	if dto.GeneralInfo.Gender == "" {
		genders := []string{"male", "female"}
		dto.GeneralInfo.Gender = genders[RandomInt(0, len(genders)-1)]
	}
	if dto.GeneralInfo.Nationality == "" {
		dto.GeneralInfo.Nationality = "US"
	}
	if dto.GeneralInfo.Status == "" {
		dto.GeneralInfo.Status = enums.StudentStatusActive
	}
	if dto.GeneralInfo.DateOfBirth.IsZero() {
		dto.GeneralInfo.DateOfBirth = time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, RandomInt(0, 365*5))
	}

	if dto.ContactInfo.MobilePhoneNumber.Prefix == "" {
		dto.ContactInfo.MobilePhoneNumber.Prefix = "+1"
	}
	if dto.ContactInfo.MobilePhoneNumber.Content == "" {
		dto.ContactInfo.MobilePhoneNumber.Content = fmt.Sprintf("%09d", RandomInt(100000000, 999999999))
	}

	if dto.AcademicInfo.Stage == "" {
		stages := []string{"elementary", "middle", "high"}
		dto.AcademicInfo.Stage = stages[RandomInt(0, len(stages)-1)]
	}
	if dto.AcademicInfo.Class == "" {
		dto.AcademicInfo.Class = fmt.Sprintf("%d", RandomInt(1, 12))
	}
	if dto.AcademicInfo.EnrollmentDate.IsZero() {
		dto.AcademicInfo.EnrollmentDate = time.Now().AddDate(0, 0, -RandomInt(1, 365))
	}

	if dto.AccountStudentSetting.Email == "" {
		dto.AccountStudentSetting.Email = RandomEmail()
	}
	if dto.AccountStudentSetting.InitialPass == "" {
		dto.AccountStudentSetting.InitialPass = "password123"
	}
	if dto.AccountStudentSetting.ConfirmPass == "" {
		dto.AccountStudentSetting.ConfirmPass = dto.AccountStudentSetting.InitialPass
	}
	if dto.AccountStudentSetting.JoinStatus == "" {
		dto.AccountStudentSetting.JoinStatus = "active"
	}
	if !dto.AccountStudentSetting.IsActive {
		dto.AccountStudentSetting.IsActive = true
	}

	body, _ := json.Marshal(dto)
	req := httptest.NewRequest("POST", "/students", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to create student: status %d, body: %s", w.Code, w.Body.String())
	}

	var student models.Student
	if err := json.Unmarshal(w.Body.Bytes(), &student); err != nil {
		t.Fatalf("Failed to unmarshal student: %v", err)
	}
	return student
}

// CreateParent creates a parent with random values for missing fields
func (f *TestFixture) CreateParent(t *testing.T, dto *models.ParentDto) models.Parent {
	if dto == nil {
		dto = &models.ParentDto{}
	}

	if len(dto.ParentGeneralInfo.FirstName) == 0 {
		dto.ParentGeneralInfo.FirstName = transl.Localizable[string]{
			"ar": "ولي أمر",
			"en": "Parent" + RandomString(4),
		}
	}
	if len(dto.ParentGeneralInfo.LastName) == 0 {
		dto.ParentGeneralInfo.LastName = transl.Localizable[string]{
			"ar": "اختبار",
			"en": "Test" + RandomString(4),
		}
	}
	if dto.ParentGeneralInfo.Gender == "" {
		genders := []string{"male", "female"}
		dto.ParentGeneralInfo.Gender = genders[RandomInt(0, len(genders)-1)]
	}
	if dto.ParentGeneralInfo.Nationality == "" {
		dto.ParentGeneralInfo.Nationality = "US"
	}
	if dto.ParentGeneralInfo.Status == "" {
		dto.ParentGeneralInfo.Status = "active"
	}
	if dto.ParentGeneralInfo.DateOfBirth.IsZero() {
		dto.ParentGeneralInfo.DateOfBirth = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, RandomInt(0, 365*10))
	}

	if dto.ParentContactInfo.MobilePhoneNumber.Prefix == "" {
		dto.ParentContactInfo.MobilePhoneNumber.Prefix = "+1"
	}
	if dto.ParentContactInfo.MobilePhoneNumber.Content == "" {
		dto.ParentContactInfo.MobilePhoneNumber.Content = fmt.Sprintf("%09d", RandomInt(100000000, 999999999))
	}

	if dto.AccountParentSetting.Email == "" {
		dto.AccountParentSetting.Email = RandomEmail()
	}
	if dto.AccountParentSetting.InitialPass == "" {
		dto.AccountParentSetting.InitialPass = "password123"
	}
	if dto.AccountParentSetting.ConfirmPass == "" {
		dto.AccountParentSetting.ConfirmPass = dto.AccountParentSetting.InitialPass
	}
	if dto.AccountParentSetting.JoinStatus == "" {
		dto.AccountParentSetting.JoinStatus = "active"
	}
	if !dto.AccountParentSetting.IsActive {
		dto.AccountParentSetting.IsActive = true
	}

	body, _ := json.Marshal(dto)
	req := httptest.NewRequest("POST", "/parents", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to create parent: status %d, body: %s", w.Code, w.Body.String())
	}

	var parent models.Parent
	if err := json.Unmarshal(w.Body.Bytes(), &parent); err != nil {
		t.Fatalf("Failed to unmarshal parent: %v", err)
	}
	return parent
}

// CreateParentStudentRelation creates a relation between a parent and a student
func (f *TestFixture) CreateParentStudentRelation(t *testing.T, parentID, studentID primitive.ObjectID) models.ParentStudentRelation {
	dto := models.ParentStudentRelationDto{
		FirstId:    parentID,
		FirstType:  enums.PersonTypeParent,
		SecondId:   studentID,
		SecondType: enums.PersonTypeStudent,
	}

	body, _ := json.Marshal(dto)
	req := httptest.NewRequest("POST", "/parentStudentRelations", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("Failed to create parent-student relation: status %d, body: %s", w.Code, w.Body.String())
	}

	var relation models.ParentStudentRelation
	if err := json.Unmarshal(w.Body.Bytes(), &relation); err != nil {
		t.Fatalf("Failed to unmarshal parent-student relation: %v", err)
	}
	return relation
}

// CreateCourse creates a course with random values for missing fields
func (f *TestFixture) CreateCourse(t *testing.T, dto *contentmodels.CourseDto) contentmodels.Course {
	// Switch to admin context with course management capabilities
	originalConfig := f.AuthServer.GetCurrentConfig()
	adminConfig := setup.DefaultMockAuthConfig()
	adminConfig.UserID = originalConfig.UserID
	adminConfig.Capabilities = []string{"authenticate", "ManageAllCourses"}
	adminConfig.ShouldAuthenticate = true
	adminConfig.ShouldAuthorize = true
	f.AuthServer.UpdateConfig(adminConfig)
	defer f.AuthServer.UpdateConfig(originalConfig) // Restore original config
	if dto == nil {
		dto = &contentmodels.CourseDto{}
	}

	if len(dto.BasicInfo.Name) == 0 {
		dto.BasicInfo.Name = transl.Localizable[string]{
			"ar": "دورة " + RandomString(4),
			"en": "Course " + RandomString(4),
		}
	}
	if dto.BasicInfo.Type == "" {
		dto.BasicInfo.Type = "curriculum"
	}
	if dto.BasicInfo.SchoolType == "" {
		dto.BasicInfo.SchoolType = "public"
	}
	if dto.BasicInfo.Level == "" {
		dto.BasicInfo.Level = "beginner"
	}
	if dto.BasicInfo.Status == "" {
		dto.BasicInfo.Status = "active"
	}
	if len(dto.BasicInfo.Teachers) == 0 {
		t.Fatal("Course requires at least one teacher")
	}
	if dto.BasicInfo.Duration == 0 {
		dto.BasicInfo.Duration = 120
	}
	if dto.BasicInfo.MaxStudents == 0 {
		dto.BasicInfo.MaxStudents = 30
	}

	// Ensure Content is initialized as empty array (not null) to avoid aggregation errors
	if dto.Content == nil {
		dto.Content = []contentmodels.Section{}
	}

	body, _ := json.Marshal(dto)
	req := httptest.NewRequest("POST", "/courses", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to create course: status %d, body: %s", w.Code, w.Body.String())
	}

	var course contentmodels.Course
	if err := json.Unmarshal(w.Body.Bytes(), &course); err != nil {
		t.Fatalf("Failed to unmarshal course: %v", err)
	}
	return course
}

// CreateCoaching creates a coaching program with random values for missing fields
func (f *TestFixture) CreateCoaching(t *testing.T, dto *contentmodels.CoachingDto) contentmodels.Coaching {
	// Switch to admin context with coaching management capabilities
	originalConfig := f.AuthServer.GetCurrentConfig()
	adminConfig := setup.DefaultMockAuthConfig()
	adminConfig.UserID = originalConfig.UserID
	adminConfig.Capabilities = []string{"authenticate", "ManageAllCoachingPrograms"}
	adminConfig.ShouldAuthenticate = true
	adminConfig.ShouldAuthorize = true
	f.AuthServer.UpdateConfig(adminConfig)
	defer f.AuthServer.UpdateConfig(originalConfig) // Restore original config

	if dto == nil {
		dto = &contentmodels.CoachingDto{}
	}

	if len(dto.Title) == 0 {
		dto.Title = transl.Localizable[string]{
			"ar": "برنامج تدريبي " + RandomString(4),
			"en": "Coaching Program " + RandomString(4),
		}
	}

	if dto.ProgramType == "" {
		dto.ProgramType = "foundational"
	}

	if dto.SchoolType == "" {
		dto.SchoolType = "public"
	}

	if len(dto.Class) == 0 {
		dto.Class = []string{"Grade 1"}
	}
	if dto.Level == "" {
		dto.Level = "beginner"
	}

	if len(dto.Coachs) == 0 {
		t.Fatal("Coaching program requires at least one coach (Teacher ID)")
	}

	if len(dto.Description) == 0 {
		dto.Description = transl.Localizable[string]{
			"ar": "وصف البرنامج",
			"en": "Program Description",
		}
	}

	if dto.MinSessionNum == 0 {
		dto.MinSessionNum = 4
	}
	if dto.MaxSessionNum == 0 {
		dto.MaxSessionNum = 8
	}
	if dto.SessionDuration == 0 {
		dto.SessionDuration = 60
	}

	if len(dto.Sessions) == 0 {
		dto.Sessions = []contentmodels.CoachingSession{
			{SessionNum: 0, SessionName: transl.Localizable[string]{"ar": "جلسة 1", "en": "Session 1"}},
			{SessionNum: 1, SessionName: transl.Localizable[string]{"ar": "جلسة 2", "en": "Session 2"}},
			{SessionNum: 2, SessionName: transl.Localizable[string]{"ar": "جلسة 3", "en": "Session 3"}},
			{SessionNum: 3, SessionName: transl.Localizable[string]{"ar": "جلسة 4", "en": "Session 4"}},
		}
	}

	body, _ := json.Marshal(dto)
	req := httptest.NewRequest("POST", "/coaching", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to create coaching: status %d, body: %s", w.Code, w.Body.String())
	}

	var coaching contentmodels.Coaching
	if err := json.Unmarshal(w.Body.Bytes(), &coaching); err != nil {
		t.Fatalf("Failed to unmarshal coaching: %v", err)
	}
	return coaching
}

// CreatePlacementTest creates a placement test with random values for missing fields
func (f *TestFixture) CreatePlacementTest(t *testing.T, dto *contentmodels.PlacementTestDto) contentmodels.PlacementTest {
	// Switch to admin context with placement test management capabilities
	originalConfig := f.AuthServer.GetCurrentConfig()
	adminConfig := setup.DefaultMockAuthConfig()
	adminConfig.UserID = originalConfig.UserID
	adminConfig.Capabilities = []string{"authenticate", "ManageAllPlacementTests", "ViewAllCourses", "ViewAllCoachingPrograms"}
	adminConfig.ShouldAuthenticate = true
	adminConfig.ShouldAuthorize = true
	f.AuthServer.UpdateConfig(adminConfig)
	defer f.AuthServer.UpdateConfig(originalConfig) // Restore original config

	if dto == nil {
		dto = &contentmodels.PlacementTestDto{}
	}
	if dto.Title["en"] == "" {
		dto.Title["en"] = "Placement Test " + RandomString(4)
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

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to create placement test: status %d, body: %s", w.Code, w.Body.String())
	}

	var pt contentmodels.PlacementTest
	if err := json.Unmarshal(w.Body.Bytes(), &pt); err != nil {
		t.Fatalf("Failed to unmarshal placement test: %v", err)
	}
	return pt
}

// SwitchAuthContext switches the auth server to a different user
func (f *TestFixture) SwitchAuthContext(userID string, capabilities []string) {
	// Get current config to preserve existing settings
	currentConfig := f.AuthServer.GetCurrentConfig()
	authConfig := setup.DefaultMockAuthConfig()

	// Preserve existing role mappings and other settings if they exist
	if currentConfig.RolesMap != nil {
		authConfig.RolesMap = currentConfig.RolesMap
		authConfig.Roles = currentConfig.Roles
		authConfig.RoleCapabilities = currentConfig.RoleCapabilities
	}

	// Update user-specific settings
	authConfig.UserID = userID
	authConfig.Capabilities = capabilities
	authConfig.ShouldAuthenticate = true
	authConfig.ShouldAuthorize = true

	f.AuthServer.UpdateConfig(authConfig)
}
