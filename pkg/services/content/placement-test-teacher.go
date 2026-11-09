package content

import (
	"context"
	"larsa-academy-microservice/pkg/services/content/constant"
	"larsa-academy-microservice/pkg/services/content/models"
	messageModels "larsa-academy-microservice/pkg/services/messaging/models"
	"larsa-academy-microservice/pkg/util"
	"time"

	"larsa-academy-microservice/pkg/services/content/enums"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (p *placementtestsvcs) SubmitTeacherEvaluation(ctx context.Context, StudentPlacementTestId string, data *models.TeacherEvaluation) (any, error) {
	_id, err := primitive.ObjectIDFromHex(StudentPlacementTestId)
	if err != nil {
		return nil, constant.ErrInvalidPlacementTestId
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	studentPlacementTest, err := p.studentPlacementTestRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, constant.ErrStudentTestNotFound
	}

	isAdmin, _ := util.HasCapability(ctx, CapabilityManageAllPlacementTests)

	if !isAdmin && (studentPlacementTest.TeacherId.Value == nil || *studentPlacementTest.TeacherId.Value != cfg.User.Id) {
		return nil, constant.ErrPlacementTestEvaluatedAuth
	}

	if studentPlacementTest.Status != enums.StudentPlacementTestStatusCompleted {
		return nil, constant.ErrPlacementTestNotCompleted
	}

	// Calculate result based on score vs PassScore
	result := "fail"
	if data.Score >= float64(studentPlacementTest.PlacementTest.Settings.PassScore) {
		result = "pass"
	}

	filter := bson.M{"_id": _id}
	if !isAdmin {
		filter["teacherId"] = studentPlacementTest.TeacherId.Value
	}

	update := bson.M{"$set": bson.M{
		"score":           data.Score,
		"notes":           data.Notes,
		"quickIndecators": data.QuickIndecators,
		"result":          result,
		"status":          enums.StudentPlacementTestStatusEvaluated,
		"updatedAt":       time.Now(),
	}}

	updated, err := p.studentPlacementTestRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	_ = p.messageSvcs.SendUserNotification(ctx, []primitive.ObjectID{studentPlacementTest.StudentId}, &messageModels.Notification{
		ServiceName: "academy",
		Title:       "Placement test graded",
		Content:     "Your teacher has graded your placement test.",
		Recipient:   studentPlacementTest.StudentId.Hex(),
	})

	return updated, nil

}
