package placement

import (
	"context"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/zoom"
	"larsa-academy-microservice/pkg/services/zoom/models"
	"time"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type MockZoomService struct {
	CreateMeetingFunc func(ctx context.Context, refType string, refId primitive.ObjectID, startTime time.Time, duration int, topic string) (*models.ZoomMeeting, error)
	DeleteMeetingFunc func(ctx context.Context, zoomMeetingId primitive.ObjectID) error

	CreateMeetingCalls []CreateMeetingCall
}

type CreateMeetingCall struct {
	RefType   string
	RefId     primitive.ObjectID
	StartTime time.Time
	Duration  int
	Topic     string
}

func (m *MockZoomService) CreateMeeting(ctx context.Context, refType string, refId primitive.ObjectID, startTime time.Time, duration int, topic string) (*models.ZoomMeeting, error) {
	m.CreateMeetingCalls = append(m.CreateMeetingCalls, CreateMeetingCall{
		RefType:   refType,
		RefId:     refId,
		StartTime: startTime,
		Duration:  duration,
		Topic:     topic,
	})
	if m.CreateMeetingFunc != nil {
		return m.CreateMeetingFunc(ctx, refType, refId, startTime, duration, topic)
	}
	return &models.ZoomMeeting{
		Id:            primitive.NewObjectID(),
		ZoomMeetingId: "123456789",
		JoinUrl:       "https://zoom.us/j/123456789",
	}, nil
}

func (m *MockZoomService) DeleteMeeting(ctx context.Context, zoomMeetingId primitive.ObjectID) error {
	if m.DeleteMeetingFunc != nil {
		return m.DeleteMeetingFunc(ctx, zoomMeetingId)
	}
	return nil
}

func (m *MockZoomService) AddAccount(ctx context.Context, data zoom.AddAccountDto) (*models.ZoomAccount, error) {
	return nil, nil
}
func (m *MockZoomService) ListAccounts(ctx context.Context, query *query.Conditions) ([]models.ZoomAccount, error) {
	return nil, nil
}
func (m *MockZoomService) DeleteAccount(ctx context.Context, id primitive.ObjectID) error {
	return nil
}
func (m *MockZoomService) SyncUsers(ctx context.Context, accountId primitive.ObjectID) (int, error) {
	return 0, nil
}
func (m *MockZoomService) AddUser(ctx context.Context, data zoom.AddUserDto) (*models.ZoomUser, error) {
	return nil, nil
}
func (m *MockZoomService) ListUsers(ctx context.Context, query *query.Conditions) ([]models.ZoomUser, error) {
	return nil, nil
}
func (m *MockZoomService) DeleteUser(ctx context.Context, id primitive.ObjectID) error {
	return nil
}
func (m *MockZoomService) ListMeetings(ctx context.Context, query *query.Conditions) ([]models.ZoomMeeting, error) {
	return nil, nil
}

func RegisterMockZoomService(i *do.Injector) *MockZoomService {
	m := &MockZoomService{}
	do.Override(i, func(i *do.Injector) (zoom.ZoomService, error) {
		return m, nil
	})
	return m
}
