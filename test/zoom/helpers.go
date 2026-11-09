package zoom

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/zoom/gateway"
	"larsa-academy-microservice/pkg/services/zoom/models"
	"larsa-academy-microservice/test/helpers"
	"larsa-academy-microservice/test/setup"
	"net/http/httptest"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type TestFixture struct {
	*helpers.TestFixture
	Router     *chi.Mux
	DBName     string
	Ctx        context.Context
	AuthServer *setup.MockAuthServer
	DB         *mongo.Database
	Injector   interface{}
}

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

type MockZoomGateway struct {
	CreateMeetingFunc      func(ctx context.Context, token string, userEmail string, req *gateway.ZoomMeetingRequest) (*gateway.ZoomMeetingResponse, error)
	ListUsersFunc          func(ctx context.Context, token string) ([]gateway.ZoomUserResponse, error)
	DeleteMeetingFunc      func(ctx context.Context, token string, meetingId string, scheduleForReminder, cancelMeetingReminder bool) error
	CreateMeetingCalls     []CreateMeetingCall
	ListUsersCalls         []ListUsersCall
	DeleteMeetingCalls     []DeleteMeetingCall
	ShouldFailCreate       bool
	ShouldFailListUsers    bool
	ShouldFailDelete       bool
	NextCreateMeetingError error
	NextListUsersError     error
	NextDeleteError        error
}

type CreateMeetingCall struct {
	Token     string
	UserEmail string
	Request   *gateway.ZoomMeetingRequest
}

type ListUsersCall struct {
	Token string
}

type DeleteMeetingCall struct {
	Token                 string
	MeetingId             string
	ScheduleForReminder   bool
	CancelMeetingReminder bool
}

func (m *MockZoomGateway) CreateMeeting(ctx context.Context, token string, userEmail string, req *gateway.ZoomMeetingRequest) (*gateway.ZoomMeetingResponse, error) {
	m.CreateMeetingCalls = append(m.CreateMeetingCalls, CreateMeetingCall{
		Token:     token,
		UserEmail: userEmail,
		Request:   req,
	})

	if m.ShouldFailCreate || m.NextCreateMeetingError != nil {
		if m.NextCreateMeetingError != nil {
			return nil, m.NextCreateMeetingError
		}
		return nil, fmt.Errorf("mock CreateMeeting error")
	}

	if m.CreateMeetingFunc != nil {
		return m.CreateMeetingFunc(ctx, token, userEmail, req)
	}

	return &gateway.ZoomMeetingResponse{
		Id:        1234567890,
		JoinUrl:   "https://zoom.us/j/1234567890",
		StartUrl:  "https://zoom.us/s/1234567890?zak=xxx",
		Password:  "abc123",
		Topic:     req.Topic,
		StartTime: req.StartTime,
		Duration:  req.Duration,
	}, nil
}

func (m *MockZoomGateway) ListUsers(ctx context.Context, token string) ([]gateway.ZoomUserResponse, error) {
	m.ListUsersCalls = append(m.ListUsersCalls, ListUsersCall{Token: token})

	if m.ShouldFailListUsers || m.NextListUsersError != nil {
		if m.NextListUsersError != nil {
			return nil, m.NextListUsersError
		}
		return nil, fmt.Errorf("mock ListUsers error")
	}

	if m.ListUsersFunc != nil {
		return m.ListUsersFunc(ctx, token)
	}

	return []gateway.ZoomUserResponse{
		{
			Id:        "zoom_user_1",
			FirstName: "Test",
			LastName:  "User1",
			Email:     "user1@test.com",
			Status:    "active",
		},
		{
			Id:        "zoom_user_2",
			FirstName: "Test",
			LastName:  "User2",
			Email:     "user2@test.com",
			Status:    "active",
		},
	}, nil
}

func (m *MockZoomGateway) DeleteMeeting(ctx context.Context, token string, meetingId string, scheduleForReminder, cancelMeetingReminder bool) error {
	m.DeleteMeetingCalls = append(m.DeleteMeetingCalls, DeleteMeetingCall{
		Token:                 token,
		MeetingId:             meetingId,
		ScheduleForReminder:   scheduleForReminder,
		CancelMeetingReminder: cancelMeetingReminder,
	})

	if m.ShouldFailDelete || m.NextDeleteError != nil {
		if m.NextDeleteError != nil {
			return m.NextDeleteError
		}
		return fmt.Errorf("mock DeleteMeeting error")
	}

	return nil
}

type MockTokenManager struct {
	GetTokenFunc    func(ctx context.Context, accountId, clientId, clientSecret string, forceRefresh bool) (string, error)
	ShouldFailToken bool
	NextTokenError  error
	ReturnedToken   string
	GetTokenCalls   []GetTokenCall
}

type GetTokenCall struct {
	AccountId    string
	ClientId     string
	ClientSecret string
	ForceRefresh bool
}

func (m *MockTokenManager) GetToken(ctx context.Context, accountId, clientId, clientSecret string, forceRefresh bool) (string, error) {
	m.GetTokenCalls = append(m.GetTokenCalls, GetTokenCall{
		AccountId:    accountId,
		ClientId:     clientId,
		ClientSecret: clientSecret,
		ForceRefresh: forceRefresh,
	})

	if m.ShouldFailToken || m.NextTokenError != nil {
		if m.NextTokenError != nil {
			return "", m.NextTokenError
		}
		return "", fmt.Errorf("mock token error")
	}

	if m.GetTokenFunc != nil {
		return m.GetTokenFunc(ctx, accountId, clientId, clientSecret, forceRefresh)
	}

	if m.ReturnedToken != "" {
		return m.ReturnedToken, nil
	}

	return "mock_zoom_token_" + accountId, nil
}

func NewMockTokenManager() *MockTokenManager {
	return &MockTokenManager{
		ReturnedToken: "mock_zoom_token",
	}
}

func (f *TestFixture) createZoomAccount(t require.TestingT, email string, accountId string, clientId string, clientSecret string, webhookSecret string) *models.ZoomAccount {
	account := &models.ZoomAccount{
		Id:          primitive.NewObjectID(),
		Email:       email,
		AccountName: "Test Account",
		Status:      "active",
		CreatedAt:   time.Now(),
	}
	account.Credentials.AccountId = accountId
	account.Credentials.ClientId = clientId
	account.Credentials.ClientSecret = clientSecret
	account.Credentials.WebhookSecret = webhookSecret

	_, err := f.DB.Collection("academyZoomAccounts").InsertOne(f.Ctx, account)
	require.NoError(t, err, "Failed to create zoom account")
	return account
}

func (f *TestFixture) createZoomUser(t require.TestingT, accountId primitive.ObjectID, email string, zoomUserId string, firstName string, lastName string, status string) *models.ZoomUser {
	user := &models.ZoomUser{
		Id:            primitive.NewObjectID(),
		ZoomAccountId: accountId,
		Email:         email,
		ZoomUserId:    zoomUserId,
		FirstName:     firstName,
		LastName:      lastName,
		Status:        status,
		LastUsedAt:    time.Now().Add(-1 * time.Hour),
		CreatedAt:     time.Now(),
	}

	_, err := f.DB.Collection("academyZoomUsers").InsertOne(f.Ctx, user)
	require.NoError(t, err, "Failed to create zoom user")
	return user
}

func (f *TestFixture) createZoomMeeting(t require.TestingT, accountId, userId primitive.ObjectID, refType string, refId primitive.ObjectID, startTime time.Time, duration int, status string) *models.ZoomMeeting {
	meetingId := primitive.NewObjectID().Hex()
	meeting := &models.ZoomMeeting{
		Id:              primitive.NewObjectID(),
		ZoomAccountId:   accountId,
		ZoomUserId:      userId,
		RefType:         refType,
		RefId:           refId,
		StartTime:       startTime,
		DurationMinutes: duration,
		OccupiedUntil:   startTime.Add(time.Duration(duration+15) * time.Minute),
		Status:          status,
		ZoomMeetingId:   meetingId,
		JoinUrl:         "https://zoom.us/j/1234567890",
		StartUrl:        "https://zoom.us/s/1234567890?zak=xxx",
		Password:        "abc123",
	}

	_, err := f.DB.Collection("academyZoomMeetings").InsertOne(f.Ctx, meeting)
	require.NoError(t, err, "Failed to create zoom meeting")
	return meeting
}

func (f *TestFixture) callCreateMeeting(t require.TestingT, refType string, refId string, startTime time.Time, duration int, topic string) *httptest.ResponseRecorder {
	dto := map[string]interface{}{
		"refType":   refType,
		"refId":     refId,
		"startTime": startTime.Format(time.RFC3339),
		"duration":  duration,
		"topic":     topic,
	}
	body, _ := json.Marshal(dto)
	req := httptest.NewRequest("POST", "/api/zoom/meetings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)
	return w
}

func (f *TestFixture) callListAccounts(t require.TestingT) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/api/zoom/accounts", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)
	return w
}

func (f *TestFixture) callAddAccount(t require.TestingT, email string, accountName string, accountId string, clientId string, clientSecret string, webhookSecret string) *httptest.ResponseRecorder {
	dto := map[string]interface{}{
		"email":         email,
		"accountName":   accountName,
		"accountId":     accountId,
		"clientId":      clientId,
		"clientSecret":  clientSecret,
		"webhookSecret": webhookSecret,
	}
	body, _ := json.Marshal(dto)
	req := httptest.NewRequest("POST", "/api/zoom/accounts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)
	return w
}

func (f *TestFixture) callDeleteAccount(t require.TestingT, accountId string) *httptest.ResponseRecorder {
	url := fmt.Sprintf("/api/zoom/accounts/%s", accountId)
	req := httptest.NewRequest("DELETE", url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)
	return w
}

func (f *TestFixture) callSyncUsers(t require.TestingT, accountId string) *httptest.ResponseRecorder {
	url := fmt.Sprintf("/api/zoom/accounts/%s/sync-users", accountId)
	req := httptest.NewRequest("POST", url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)
	return w
}

func (f *TestFixture) callListUsers(t require.TestingT, accountId string) *httptest.ResponseRecorder {
	url := "/api/zoom/users"
	if accountId != "" {
		url += "?accountId=" + accountId
	}
	req := httptest.NewRequest("GET", url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)
	return w
}

func (f *TestFixture) callAddUser(t require.TestingT, zoomAccountId string, email string, zoomUserId string, firstName string, lastName string) *httptest.ResponseRecorder {
	dto := map[string]interface{}{
		"zoomAccountId": zoomAccountId,
		"email":         email,
		"zoomUserId":    zoomUserId,
		"firstName":     firstName,
		"lastName":      lastName,
	}
	body, _ := json.Marshal(dto)
	req := httptest.NewRequest("POST", "/api/zoom/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)
	return w
}

func (f *TestFixture) callDeleteUser(t require.TestingT, userId string) *httptest.ResponseRecorder {
	url := fmt.Sprintf("/api/zoom/users/%s", userId)
	req := httptest.NewRequest("DELETE", url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)
	return w
}

func (f *TestFixture) callDeleteMeeting(t require.TestingT, meetingId string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("DELETE", "/api/zoom/meetings/"+meetingId, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)
	return w
}

func (f *TestFixture) callListMeetings(t require.TestingT, queryCond *query.Conditions) *httptest.ResponseRecorder {
	url := "/api/zoom/meetings/list"
	var body []byte
	if queryCond != nil {
		body, _ = json.Marshal(queryCond)
	}

	req := httptest.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)
	return w
}

func init() {
	_ = require.New
}
