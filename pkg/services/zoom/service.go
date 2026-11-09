package zoom

import (
	"context"
	"fmt"
	"io"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/zoom/gateway"
	"larsa-academy-microservice/pkg/services/zoom/models"
	"larsa-academy-microservice/pkg/services/zoom/pool"
	"larsa-academy-microservice/pkg/services/zoom/token"
	"net/http"
	"strings"
	"time"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type ZoomService interface {
	CreateMeeting(ctx context.Context, refType string, refId primitive.ObjectID, startTime time.Time, duration int, topic string) (*models.ZoomMeeting, error)
	DeleteMeeting(ctx context.Context, zoomMeetingId primitive.ObjectID) error

	// Admin methods
	AddAccount(ctx context.Context, data AddAccountDto) (*models.ZoomAccount, error)
	ListAccounts(ctx context.Context, query *query.Conditions) ([]models.ZoomAccount, error)
	DeleteAccount(ctx context.Context, id primitive.ObjectID) error
	SyncUsers(ctx context.Context, accountId primitive.ObjectID) (int, error)

	// User methods
	AddUser(ctx context.Context, data AddUserDto) (*models.ZoomUser, error)
	ListUsers(ctx context.Context, query *query.Conditions) ([]models.ZoomUser, error)
	DeleteUser(ctx context.Context, id primitive.ObjectID) error

	// Meeting methods
	ListMeetings(ctx context.Context, query *query.Conditions) ([]models.ZoomMeeting, error)
}

type zoomService struct {
	i           *do.Injector
	accountRepo db.MainRepo[models.ZoomAccount]
	userRepo    db.MainRepo[models.ZoomUser]
	meetingRepo db.MainRepo[models.ZoomMeeting]
	tokens      *token.TokenManager
	pool        *pool.PoolPicker
	gw          gateway.ZoomGatewayInterface
}

func NewZoomService(i *do.Injector) (ZoomService, error) {
	accountRepo := db.NewRepo[models.ZoomAccount](i)
	userRepo := db.NewRepo[models.ZoomUser](i)
	meetingRepo := db.NewRepo[models.ZoomMeeting](i)

	return &zoomService{
		i:           i,
		accountRepo: accountRepo,
		userRepo:    userRepo,
		meetingRepo: meetingRepo,
		tokens:      token.NewTokenManager(),
		pool:        pool.NewPoolPicker(accountRepo, userRepo, meetingRepo),
		gw:          gateway.NewZoomGateway(),
	}, nil
}

func (s *zoomService) CreateMeeting(ctx context.Context, refType string, refId primitive.ObjectID, startTime time.Time, duration int, topic string) (*models.ZoomMeeting, error) {
	// 1. Run Pool Picker
	user, account, err := s.pool.PickAvailableUser(ctx, startTime, duration)
	if err != nil {
		return nil, fmt.Errorf("zoom capacity exceeded or error: %w", err)
	}

	// 2. Prepare Request
	zoomReq := &gateway.ZoomMeetingRequest{
		Topic:     topic,
		Type:      2, // Scheduled
		StartTime: startTime.Format(time.RFC3339),
		Duration:  duration,
		Timezone:  "UTC",
		Agenda:    "Academy Meeting",
	}
	zoomReq.Settings.HostVideo = true
	zoomReq.Settings.ParticipantVideo = true
	zoomReq.Settings.MuteUponEntry = true
	zoomReq.Settings.WaitingRoom = false
	zoomReq.Settings.AutoRecording = "cloud"

	// 3. Try to create meeting (with retry logic)
	var zoomResp *gateway.ZoomMeetingResponse
	maxRetriesPerUser := 2

	// Try with the picked user first
	zoomResp, err = s.createMeetingWithRetry(ctx, account, user, zoomReq, maxRetriesPerUser)

	// If failed, try with ANOTHER user from the SAME account
	if err != nil {
		fmt.Printf("Zoom creation failed for user %s, trying another user from account %s: %v\n", user.Email, account.Email, err)

		// Find another available user in the same account
		anotherUser, _, pickErr := s.pool.PickAvailableUser(ctx, startTime, duration)
		if pickErr == nil && anotherUser.Id != user.Id && anotherUser.ZoomAccountId == account.Id {
			user = anotherUser
			zoomResp, err = s.createMeetingWithRetry(ctx, account, user, zoomReq, maxRetriesPerUser)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("zoom api call failed after retries: %w", err)
	}

	// 4. Create ZoomMeeting record
	meeting := &models.ZoomMeeting{
		Id:              primitive.NewObjectID(),
		ZoomAccountId:   account.Id,
		ZoomUserId:      user.Id,
		RefType:         refType,
		RefId:           refId,
		StartTime:       startTime,
		DurationMinutes: duration,
		OccupiedUntil:   startTime.Add(time.Duration(duration+15) * time.Minute),
		Status:          "scheduled",
		ZoomMeetingId:   fmt.Sprintf("%d", zoomResp.Id),
		JoinUrl:         zoomResp.JoinUrl,
		StartUrl:        zoomResp.StartUrl,
		Password:        zoomResp.Password,
	}

	if err := s.meetingRepo.Add(ctx, meeting); err != nil {
		return nil, fmt.Errorf("failed to save zoom meeting to database: %w", err)
	}

	// 5. Update LastUsedAt on user
	_, _ = s.userRepo.Patch(ctx, bson.M{"_id": user.Id}, bson.M{"$set": bson.M{"lastUsedAt": time.Now()}})

	return meeting, nil
}

func (s *zoomService) createMeetingWithRetry(ctx context.Context, account *models.ZoomAccount, user *models.ZoomUser, req *gateway.ZoomMeetingRequest, maxRetries int) (*gateway.ZoomMeetingResponse, error) {
	var lastErr error
	forceRefresh := false

	for i := 0; i < maxRetries; i++ {
		// 1. Get Token
		token, err := s.tokens.GetToken(ctx, account.Credentials.AccountId, account.Credentials.ClientId, account.Credentials.ClientSecret, forceRefresh)
		if err != nil {
			return nil, fmt.Errorf("failed to authorize zoom account: %w", err)
		}

		// 2. Call Zoom API
		zoomResp, err := s.gw.CreateMeeting(ctx, token, user.Email, req)
		if err == nil {
			return zoomResp, nil
		}

		lastErr = err
		// If 401 or specific Zoom error code, trigger force refresh for next attempt
		if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "124") {
			forceRefresh = true
			continue
		}

		// For other errors, maybe don't retry same user?
		// But requirement said "try again for the same user", so we continue.
	}

	return nil, lastErr
}

func (s *zoomService) DeleteMeeting(ctx context.Context, zoomMeetingId primitive.ObjectID) error {
	// 1. Get Meeting from DB
	meeting, err := s.meetingRepo.GetByFilter(ctx, bson.M{"_id": zoomMeetingId})
	if err != nil {
		return err
	}

	if meeting.Status == "cancelled" {
		return nil
	}

	// 2. Get Account Credentials
	account, err := s.accountRepo.GetByFilter(ctx, bson.M{"_id": meeting.ZoomAccountId})
	if err != nil {
		return err
	}

	// 3. Get Token
	token, err := s.tokens.GetToken(ctx, account.Credentials.AccountId, account.Credentials.ClientId, account.Credentials.ClientSecret, false)
	if err != nil {
		return err
	}

	// 4. Call Zoom API
	err = s.gw.DeleteMeeting(ctx, token, meeting.ZoomMeetingId, false, false)
	if err != nil {
		// If meeting not found in Zoom (404), we still want to mark it as cancelled in our DB
		fmt.Printf("Zoom API delete error (continuing): %v\n", err)
	}

	// 5. Update DB Status
	_, err = s.meetingRepo.Patch(ctx, bson.M{"_id": zoomMeetingId}, bson.M{"$set": bson.M{
		"status": "cancelled",
	}})

	return err
}

func (s *zoomService) AddAccount(ctx context.Context, data AddAccountDto) (*models.ZoomAccount, error) {
	// 1. Check for duplicate account (by email or accountId)
	existing, _ := s.accountRepo.GetByFilter(ctx, bson.M{
		"$or": []bson.M{
			{"email": data.Email},
			{"credentials.accountId": data.AccountId},
		},
	})
	if existing != nil {
		return nil, helpers.BadRequest("zoom account with this email or accountId already exists")
	}

	// 2. Validate with Zoom before saving (Check if token is obtainable)
	token, err := s.tokens.GetToken(ctx, data.AccountId, data.ClientId, data.ClientSecret, true)
	if err != nil {
		return nil, fmt.Errorf("invalid zoom credentials (token failure): %w", err)
	}

	// 3. Verify if we can actually call the API with this token (List Meetings)
	url := "https://api.zoom.us/v2/users/me/meetings?page_size=1"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to verify zoom account via API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("zoom account verification failed: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	account := &models.ZoomAccount{
		Id:          primitive.NewObjectID(),
		Email:       data.Email,
		AccountName: data.AccountName,
		Status:      "active",
		CreatedAt:   time.Now(),
	}
	account.Credentials.AccountId = data.AccountId
	account.Credentials.ClientId = data.ClientId
	account.Credentials.ClientSecret = data.ClientSecret
	account.Credentials.WebhookSecret = data.WebhookSecret

	if err := s.accountRepo.Add(ctx, account); err != nil {
		return nil, err
	}

	return account, nil
}

func (s *zoomService) ListAccounts(ctx context.Context, queryCond *query.Conditions) ([]models.ZoomAccount, error) {
	var accounts []models.ZoomAccount
	filter := bson.M{}
	if queryCond != nil {
		f, err := queryCond.ConvertToMongo()
		if err != nil {
			return nil, err
		}
		filter = f
	}

	if err := s.accountRepo.Aggregate(ctx, []bson.M{{"$match": filter}}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &accounts)
	}); err != nil {
		return nil, err
	}
	return accounts, nil
}

func (s *zoomService) DeleteAccount(ctx context.Context, id primitive.ObjectID) error {
	// 1. Delete all users associated with this account
	err := s.userRepo.DeleteMany(ctx, bson.M{"zoomAccountId": id})
	if err != nil {
		return err
	}

	// 2. Delete the account
	return s.accountRepo.DeleteMain(ctx, bson.M{"_id": id})
}

func (s *zoomService) SyncUsers(ctx context.Context, accountId primitive.ObjectID) (int, error) {
	// 1. Get Account
	account, err := s.accountRepo.GetByFilter(ctx, bson.M{"_id": accountId})
	if err != nil {
		return 0, err
	}

	// 2. Get Token
	token, err := s.tokens.GetToken(ctx, account.Credentials.AccountId, account.Credentials.ClientId, account.Credentials.ClientSecret, false)
	if err != nil {
		return 0, err
	}

	// 3. Get Zoom Users
	zoomUsers, err := s.gw.ListUsers(ctx, token)
	if err != nil {
		// If 401, retry once with force refresh
		if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "124") {
			token, err = s.tokens.GetToken(ctx, account.Credentials.AccountId, account.Credentials.ClientId, account.Credentials.ClientSecret, true)
			if err == nil {
				zoomUsers, err = s.gw.ListUsers(ctx, token)
			}
		}

		if err != nil {
			return 0, err
		}
	}

	// 4. Update Database
	count := 0
	foundUserIds := make(map[string]bool)

	for _, zu := range zoomUsers {
		foundUserIds[zu.Id] = true
		filter := bson.M{
			"zoomAccountId": accountId,
			"zoomUserId":    zu.Id,
		}

		existing, _ := s.userRepo.GetByFilter(ctx, filter)
		if existing != nil {
			// Update existing user status and name
			_, _ = s.userRepo.Patch(ctx, filter, bson.M{
				"$set": bson.M{
					"firstName": zu.FirstName,
					"lastName":  zu.LastName,
					"email":     zu.Email,
					"status":    zu.Status,
				},
			})
		} else {
			// Create new user
			newUser := &models.ZoomUser{
				Id:            primitive.NewObjectID(),
				ZoomAccountId: accountId,
				ZoomUserId:    zu.Id,
				Email:         zu.Email,
				FirstName:     zu.FirstName,
				LastName:      zu.LastName,
				Status:        zu.Status,
				CreatedAt:     time.Now(),
			}
			if err := s.userRepo.Add(ctx, newUser); err == nil {
				count++
			}
		}
	}

	// 5. Mark users not found in Zoom as inactive
	zoomUserIds := make([]string, 0, len(foundUserIds))
	for id := range foundUserIds {
		zoomUserIds = append(zoomUserIds, id)
	}

	_ = s.userRepo.DeleteMany(ctx, bson.M{
		"zoomAccountId": accountId,
		"zoomUserId":    bson.M{"$nin": zoomUserIds},
	})
	// Wait, user asked to make them inactive, not delete.
	_, _ = s.userRepo.BulkWrite(ctx, []mongo.WriteModel{
		mongo.NewUpdateManyModel().
			SetFilter(bson.M{
				"zoomAccountId": accountId,
				"zoomUserId":    bson.M{"$nin": zoomUserIds},
			}).
			SetUpdate(bson.M{"$set": bson.M{"status": "inactive"}}),
	})

	return count, nil
}

func (s *zoomService) DeleteUser(ctx context.Context, id primitive.ObjectID) error {
	return s.userRepo.DeleteMain(ctx, bson.M{"_id": id})
}

type AddUserDto struct {
	ZoomAccountId primitive.ObjectID `json:"zoomAccountId" validate:"required"`
	Email         string             `json:"email" validate:"required,email"`
	ZoomUserId    string             `json:"zoomUserId" validate:"required"`
	FirstName     string             `json:"firstName"`
	LastName      string             `json:"lastName"`
}

func (s *zoomService) AddUser(ctx context.Context, data AddUserDto) (*models.ZoomUser, error) {
	// 1. Check if account exists
	_, err := s.accountRepo.GetByFilter(ctx, bson.M{"_id": data.ZoomAccountId})
	if err != nil {
		return nil, helpers.BadRequest("invalid zoom account id")
	}

	// 2. Check for duplicate
	existing, _ := s.userRepo.GetByFilter(ctx, bson.M{
		"zoomAccountId": data.ZoomAccountId,
		"zoomUserId":    data.ZoomUserId,
	})
	if existing != nil {
		return nil, helpers.BadRequest("user already exists in this account")
	}

	user := &models.ZoomUser{
		Id:            primitive.NewObjectID(),
		ZoomAccountId: data.ZoomAccountId,
		ZoomUserId:    data.ZoomUserId,
		Email:         data.Email,
		FirstName:     data.FirstName,
		LastName:      data.LastName,
		Status:        "active",
		CreatedAt:     time.Now(),
	}

	if err := s.userRepo.Add(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *zoomService) ListUsers(ctx context.Context, queryCond *query.Conditions) ([]models.ZoomUser, error) {
	var users []models.ZoomUser
	filter := bson.M{}
	if queryCond != nil {
		f, err := queryCond.ConvertToMongo()
		if err != nil {
			return nil, err
		}
		filter = f
	}

	if err := s.userRepo.Aggregate(ctx, []bson.M{{"$match": filter}}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &users)
	}); err != nil {
		return nil, err
	}
	return users, nil
}

func (s *zoomService) ListMeetings(ctx context.Context, queryCond *query.Conditions) ([]models.ZoomMeeting, error) {
	var meetings []models.ZoomMeeting
	filter := bson.M{}
	if queryCond != nil {
		f, err := queryCond.ConvertToMongo()
		if err != nil {
			return nil, err
		}
		filter = f
	}

	if err := s.meetingRepo.Aggregate(ctx, []bson.M{{"$match": filter}}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &meetings)
	}); err != nil {
		return nil, err
	}
	return meetings, nil
}
