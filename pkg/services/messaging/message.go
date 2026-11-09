package messaging

import (
	"context"
	"errors"
	"larsa-academy-microservice/pkg/nats"
	"larsa-academy-microservice/pkg/services/messaging/enums"
	"larsa-academy-microservice/pkg/services/messaging/models"
	"larsa-academy-microservice/pkg/services/messaging/repo"
	"larsa-academy-microservice/pkg/services/messaging/template"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"strings"
	"time"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type MessageSvcs interface {
	GetAll(ctx context.Context) ([]models.Message, error)
	SendEmail(ctx context.Context, msg *models.Message) error
	GetTemplateMessage(ctx context.Context, msgType enums.MsgTyps, data any) (message string, subject string, err error)
	SendNotification(ctx context.Context, notification *models.Notification) error
	SendUserNotification(ctx context.Context, userIds []primitive.ObjectID, notification *models.Notification) error
}

type messagesvcs struct {
	repo repo.MessageRepo
}

func NewMessageSvcs(i *do.Injector) (MessageSvcs, error) {
	return &messagesvcs{
		repo: do.MustInvoke[repo.MessageRepo](i),
	}, nil
}

func (m *messagesvcs) GetAll(ctx context.Context) ([]models.Message, error) {
	pipeline := []bson.M{
		{"$match": bson.M{}},
	}

	var result []models.Message
	err := m.repo.Aggregate(ctx, pipeline, func(cursor *mongo.Cursor) error {
		return cursor.All(ctx, &result)
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (m *messagesvcs) firstOrUpdate(ctx context.Context, msg *models.Message) error {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	if cfg.User != nil {
		msg.CreatedBy = cfg.User.Id
	} else {
		msg.CreatedBy = primitive.NilObjectID
	}

	if msg.Id == primitive.NilObjectID {
		msg.Id = primitive.NewObjectID()
		msg.CreatedAt = time.Now()
		//	Check if used is authenticated
		if cfg.User != nil {
			msg.CreatedBy = cfg.User.Id
		} else {
			msg.CreatedBy = primitive.NilObjectID
		}

		if err := m.repo.Add(ctx, msg); err != nil {
			return err
		}

	} else {
		filter := bson.M{"_id": msg.Id}
		update := bson.M{"$set": msg}

		if _, err := m.repo.Patch(ctx, filter, update); err != nil {
			return err
		}
	}

	return nil
}

func (m *messagesvcs) SendEmail(ctx context.Context, msg *models.Message) error {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	if err := m.firstOrUpdate(ctx, msg); err != nil {
		return err
	}

	email := &models.Email{
		To:      msg.Email,
		Subject: msg.Subject,
		Message: msg.MessageHtml,
		Headers: cfg.Hp,
		Others:  msg.Others,
	}

	nats.Publish("email:send", email)
	return nil
}

// must use in all places where we need to send email or sms
func (m *messagesvcs) GetTemplateMessage(ctx context.Context, msgType enums.MsgTyps, data any) (message string, subject string, err error) {
	tpl, ok := template.Templates[msgType]
	if !ok {
		return "", "", errors.New("template not found")
	}

	return tpl.Construct(ctx, data)
}

func (m *messagesvcs) SendNotification(ctx context.Context, notification *models.Notification) error {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	result := strings.TrimPrefix(notification.Recipient, "ObjectID(\"")
	result = strings.TrimSuffix(result, "\")")

	topic := "en_user_" + result

	n2 := &types.NotificationMsg{
		XClient:         cfg.Db,
		XService:        notification.ServiceName,
		Header:          notification.Title,
		Content:         notification.Content,
		Type:            "fcm",
		Data:            notification.Data,
		Topic:           topic,
		Image:           notification.Image,
		Icon:            notification.Icon,
		Badge:           &notification.Badge,
		AndroidColor:    notification.AndroidColor,
		AndroidSound:    notification.AndroidSound,
		AndroidPriority: notification.AndroidPriority,
		IOSSound:        notification.IOSSound,
		ClickActionURL:  notification.Url,
	}

	nats.Publish("go-notification:automation", n2)

	return nil

}

// SendUserNotification publishes a user-based notification using the go-notification:create pipeline.
// This path is used for per-user events where the notification service resolves FCM tokens
// based on userIds (createdBy) and type == "fcm".
func (m *messagesvcs) SendUserNotification(ctx context.Context, userIds []primitive.ObjectID, notification *models.Notification) error {
	n := &types.Notification{
		Header:   notification.Title,
		Content:  notification.Content,
		Type:     "fcm",
		Service:  notification.ServiceName,
		Data:     notification.Data,
		UserIds:  userIds,
		Topic:    "",
		Status:   "",
		Touched:  false,
		Hidden:   false,
		IsAction: false,
	}

	// User-based pipeline: the notification service will use userIds and type == "fcm"
	// to find FCM tokens and send pushes.
	nats.Publish("go-notification:create", n)

	return nil

}
