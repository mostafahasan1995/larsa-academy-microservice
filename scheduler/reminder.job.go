package scheduler

import (
	"context"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/services/content/enums"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/services/messaging"
	messageModels "larsa-academy-microservice/pkg/services/messaging/models"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"time"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"github.com/rs/zerolog/log"
	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// ReminderJob sends daily reminders for today's sessions and placement tests.
type ReminderJob struct {
	locker                          *Locker
	studentSessionRepo              db.MainRepo[models.StudentSession]
	onlinePlacementTestScheduleRepo db.MainRepo[models.OnlinePlacementTestSchedule]
	messageSvcs                     messaging.MessageSvcs
}

// NewReminderJob creates a new ReminderJob.
func NewReminderJob(i *do.Injector, locker *Locker) *ReminderJob {
	return &ReminderJob{
		locker:                          locker,
		studentSessionRepo:              db.NewRepo[models.StudentSession](i),
		onlinePlacementTestScheduleRepo: db.NewRepo[models.OnlinePlacementTestSchedule](i),
		messageSvcs:                     do.MustInvoke[messaging.MessageSvcs](i),
	}
}

// Run implements cron.Job.
func (r *ReminderJob) Run() {
	ctx := context.Background()

	lock, err := r.locker.Lock(ctx, "reminder_job")
	if err != nil {
		log.Error().Msg("Reminder job: error acquiring lock: " + err.Error())
		return
	}
	defer func() {
		if lock != nil {
			if err := r.locker.Unlock(ctx, lock.Id); err != nil {
				log.Error().Msg("Reminder job: error releasing lock: " + err.Error())
			}
		}
	}()

	log.Info().Msg("Reminder job started")

	dbs := []string{
		"604674966fe156529e3e03e9", // development; same as renewal job
		"d30ib4k5drjs73d29sk0",     // production
	}

	for _, dbName := range dbs {
		cfg := &types.AppCfg{
			Db: dbName,
			Hp: &common.HeaderParams{},
		}
		serviceToken, err := common.GetServiceToken("academy", cfg.Hp)
		if err != nil {
			log.Error().Msg("Reminder job: error getting service token: " + err.Error())
			continue
		}
		cfg.Hp.ServiceToken = serviceToken
		runCtx := context.WithValue(ctx, util.ReqAppCfg, cfg)

		if err := r.sendSessionReminders(runCtx); err != nil {
			log.Error().Err(err).Msg("Reminder job: error sending session reminders for db: " + dbName)
		}
		if err := r.sendPlacementTestReminders(runCtx); err != nil {
			log.Error().Err(err).Msg("Reminder job: error sending placement test reminders for db: " + dbName)
		}
		log.Info().Msg("Reminder job completed for db: " + dbName)
	}
}

func (r *ReminderJob) sendSessionReminders(ctx context.Context) error {
	now := time.Now().UTC()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, time.UTC)

	pipeline := []bson.M{
		{"$match": bson.M{
			"date": bson.M{"$gte": startOfDay, "$lte": endOfDay},
			"status": bson.M{"$in": []enums.StudentSessionStatus{
				enums.StudentSessionStatusScheduled,
				enums.StudentSessionStatusRescheduled,
			}},
			"trash": false,
		}},
		{"$group": bson.M{
			"_id":      nil,
			"students": bson.M{"$addToSet": "$studentId"},
			"teachers": bson.M{"$addToSet": "$teacherId"},
		}},
	}

	var result []struct {
		Students []primitive.ObjectID `bson:"students"`
		Teachers []primitive.ObjectID `bson:"teachers"`
	}
	err := r.studentSessionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return err
	}
	if len(result) == 0 {
		return nil
	}

	seen := make(map[primitive.ObjectID]bool)
	for _, id := range result[0].Students {
		if id.IsZero() || seen[id] {
			continue
		}
		seen[id] = true
		n := &messageModels.Notification{
			ServiceName: "academy",
			Title:       "Session reminder",
			Content:     "You have a session today.",
			Recipient:   id.Hex(),
		}
		_ = r.messageSvcs.SendNotification(ctx, n)
	}
	for _, id := range result[0].Teachers {
		if id.IsZero() || seen[id] {
			continue
		}
		seen[id] = true
		n := &messageModels.Notification{
			ServiceName: "academy",
			Title:       "Session reminder",
			Content:     "You have a session today.",
			Recipient:   id.Hex(),
		}
		_ = r.messageSvcs.SendNotification(ctx, n)
	}
	return nil
}

func (r *ReminderJob) sendPlacementTestReminders(ctx context.Context) error {
	now := time.Now().UTC()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, time.UTC)

	pipeline := []bson.M{
		{"$match": bson.M{
			"date":   bson.M{"$gte": startOfDay, "$lte": endOfDay},
			"status": enums.StudentPlacementTestStatusScheduled,
		}},
	}

	var schedules []models.OnlinePlacementTestSchedule
	err := r.onlinePlacementTestScheduleRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &schedules)
	})
	if err != nil {
		return err
	}

	seen := make(map[primitive.ObjectID]bool)
	for _, s := range schedules {
		if !s.StudentId.IsZero() && !seen[s.StudentId] {
			seen[s.StudentId] = true
			n := &messageModels.Notification{
				ServiceName: "academy",
				Title:       "Placement test reminder",
				Content:     "You have a placement test today.",
				Recipient:   s.StudentId.Hex(),
			}
			_ = r.messageSvcs.SendNotification(ctx, n)
		}
		if s.TeacherId.Value != nil && !seen[*s.TeacherId.Value] {
			seen[*s.TeacherId.Value] = true
			n := &messageModels.Notification{
				ServiceName: "academy",
				Title:       "Placement test reminder",
				Content:     "You have a placement test today.",
				Recipient:   s.TeacherId.Value.Hex(),
			}
			_ = r.messageSvcs.SendNotification(ctx, n)
		}
	}
	return nil
}
