package pool

import (
	"context"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/services/zoom/models"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type PoolPicker struct {
	accountRepo db.MainRepo[models.ZoomAccount]
	userRepo    db.MainRepo[models.ZoomUser]
	meetingRepo db.MainRepo[models.ZoomMeeting]
}

func NewPoolPicker(accountRepo db.MainRepo[models.ZoomAccount], userRepo db.MainRepo[models.ZoomUser], meetingRepo db.MainRepo[models.ZoomMeeting]) *PoolPicker {
	return &PoolPicker{
		accountRepo: accountRepo,
		userRepo:    userRepo,
		meetingRepo: meetingRepo,
	}
}

func (p *PoolPicker) PickAvailableUser(ctx context.Context, startTime time.Time, durationMinutes int) (*models.ZoomUser, *models.ZoomAccount, error) {
	// 1. Calculate Window
	slotStart := startTime
	// Buffer: 15 minutes
	slotOccupiedUntil := startTime.Add(time.Duration(durationMinutes+15) * time.Minute)

	// 2. Identify Busy Users
	// Overlap Condition: Existing.StartTime < SlotOccupiedUntil AND Existing.OccupiedUntil > SlotStart
	overlapFilter := bson.M{
		"status":        bson.M{"$ne": "cancelled"},
		"startTime":     bson.M{"$lt": slotOccupiedUntil},
		"occupiedUntil": bson.M{"$gt": slotStart},
	}

	var busyMeetings []models.ZoomMeeting
	err := p.meetingRepo.Aggregate(ctx, []bson.M{{"$match": overlapFilter}}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &busyMeetings)
	})
	if err != nil && err != mongo.ErrNoDocuments {
		return nil, nil, err
	}

	busyUserIds := make([]any, 0, len(busyMeetings))
	for _, m := range busyMeetings {
		busyUserIds = append(busyUserIds, m.ZoomUserId)
	}

	// 3. Select Available User
	userFilter := bson.M{
		"status": "active",
		"_id":    bson.M{"$nin": busyUserIds},
	}

	var availableUsers []models.ZoomUser
	// Round-robin by picking the one with the oldest LastUsedAt
	pipeline := []bson.M{
		{"$match": userFilter},
		{"$sort": bson.M{"lastUsedAt": 1}},
		{"$limit": 1},
	}

	err = p.userRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &availableUsers)
	})
	if err != nil {
		return nil, nil, err
	}

	if len(availableUsers) == 0 {
		return nil, nil, mongo.ErrNoDocuments
	}

	user := availableUsers[0]

	// 4. Get the associated Account (Connection) for credentials
	account, err := p.accountRepo.GetByFilter(ctx, bson.M{"_id": user.ZoomAccountId})
	if err != nil {
		return nil, nil, err
	}

	return &user, account, nil
}

type Slot struct {
	StartTime time.Time
	Duration  int
}

func (p *PoolPicker) PickAvailableUsersBatch(ctx context.Context, slots []Slot) ([]*models.ZoomUser, []*models.ZoomAccount, error) {
	// 1. Get all active users
	var allUsers []models.ZoomUser
	err := p.userRepo.Aggregate(ctx, []bson.M{{"$match": bson.M{"status": "active"}}}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &allUsers)
	})
	if err != nil {
		return nil, nil, err
	}

	if len(allUsers) == 0 {
		return nil, nil, mongo.ErrNoDocuments
	}

	// 2. For each user, get their existing meetings in the timeframe of the batch
	if len(slots) == 0 {
		return nil, nil, nil
	}

	minStart := slots[0].StartTime
	maxEnd := slots[0].StartTime.Add(time.Duration(slots[0].Duration+15) * time.Minute)
	for _, s := range slots {
		if s.StartTime.Before(minStart) {
			minStart = s.StartTime
		}
		end := s.StartTime.Add(time.Duration(s.Duration+15) * time.Minute)
		if end.After(maxEnd) {
			maxEnd = end
		}
	}

	overlapFilter := bson.M{
		"status":        bson.M{"$ne": "cancelled"},
		"startTime":     bson.M{"$lt": maxEnd},
		"occupiedUntil": bson.M{"$gt": minStart},
	}

	var existingMeetings []models.ZoomMeeting
	err = p.meetingRepo.Aggregate(ctx, []bson.M{{"$match": overlapFilter}}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &existingMeetings)
	})
	if err != nil && err != mongo.ErrNoDocuments {
		return nil, nil, err
	}

	// userMeetings maps userId -> list of busy intervals
	userMeetings := make(map[primitive.ObjectID][]interval)
	for _, m := range existingMeetings {
		userMeetings[m.ZoomUserId] = append(userMeetings[m.ZoomUserId], interval{
			start: m.StartTime,
			end:   m.OccupiedUntil,
		})
	}

	// 3. Select users for each slot
	selectedUsers := make([]*models.ZoomUser, len(slots))
	selectedAccounts := make([]*models.ZoomAccount, len(slots))

	// Cache accounts to avoid redundant DB calls
	accountCache := make(map[primitive.ObjectID]*models.ZoomAccount)

	for i, slot := range slots {
		slotStart := slot.StartTime
		slotEnd := slot.StartTime.Add(time.Duration(slot.Duration+15) * time.Minute)

		found := false
		// Sort users by LastUsedAt (simplified round-robin within batch)
		// Note: we'd need to update LastUsedAt in DB later, here we just simulate
		for _, u := range allUsers {
			isBusy := false
			for _, busy := range userMeetings[u.Id] {
				if slotStart.Before(busy.end) && slotEnd.After(busy.start) {
					isBusy = true
					break
				}
			}

			if !isBusy {
				// Pick this user
				selectedUsers[i] = &u
				// Mark them busy for this new slot in our simulation
				userMeetings[u.Id] = append(userMeetings[u.Id], interval{
					start: slotStart,
					end:   slotEnd,
				})

				// Get account
				if acc, ok := accountCache[u.ZoomAccountId]; ok {
					selectedAccounts[i] = acc
				} else {
					acc, err := p.accountRepo.GetByFilter(ctx, bson.M{"_id": u.ZoomAccountId})
					if err == nil {
						accountCache[u.ZoomAccountId] = acc
						selectedAccounts[i] = acc
					}
				}
				found = true
				break
			}
		}

		if !found {
			return nil, nil, mongo.ErrNoDocuments // Capacity exceeded for this batch
		}
	}

	return selectedUsers, selectedAccounts, nil
}

type interval struct {
	start time.Time
	end   time.Time
}
