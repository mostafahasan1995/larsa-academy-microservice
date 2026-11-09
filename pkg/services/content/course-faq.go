package content

import (
	"context"
	"errors"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/util"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// GetAllCourseFaqs retrieves all FAQs for a course
func (c *coursesvcs) GetAllCourseFaqs(ctx context.Context, courseId string, query *query.Conditions) ([]models.CourseFaq, error) {
	_id, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return nil, errors.New("invalid course id")
	}

	if err := query.CheckValid(); err != nil {
		return nil, err
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, err
	}

	pipeline := []bson.M{
		{"$match": bson.M{"courseId": _id, "trash": false}},
		{"$match": filter},
	}

	var result []models.CourseFaq
	errAg := c.courseFaqRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if errAg != nil {
		return nil, errAg
	}
	return result, nil
}

// GetCourseFaq retrieves a single FAQ by ID
func (c *coursesvcs) GetCourseFaq(ctx context.Context, courseId, faqId string) (*models.CourseFaq, error) {
	cId, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return nil, errors.New("invalid course id")
	}

	fId, err := primitive.ObjectIDFromHex(faqId)
	if err != nil {
		return nil, errors.New("invalid faq id")
	}

	faq, err := c.courseFaqRepo.GetByFilter(ctx, bson.M{"courseId": cId, "_id": fId, "trash": false})
	if err != nil {
		return nil, err
	}
	return faq, nil
}

// AddCourseFaq adds a new FAQ to a course
func (c *coursesvcs) AddCourseFaq(ctx context.Context, courseId string, data *models.CourseFaqDto) (*models.CourseFaq, error) {
	_id, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return nil, errors.New("invalid course id")
	}

	// Add authorization check
	canEdit, err := c.authSvc.CanEditCourseResources(ctx, courseId)
	if err != nil {
		return nil, err
	}
	if !canEdit {
		return nil, errors.New("unauthorized to edit resources for this course")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	courseFaq := &models.CourseFaq{
		Id:           primitive.NewObjectID(),
		CourseId:     _id,
		CourseFaqDto: *data,
		CreatedAt:    time.Now(),
		CreatedBy:    cfg.User.Id,
		UpdatedAt:    time.Now(),
		UpdatedBy:    cfg.User.Id,
	}

	if err := c.courseFaqRepo.Add(ctx, courseFaq); err != nil {
		return nil, err
	}

	return courseFaq, nil
}

// UpdateCourseFaq updates an existing course FAQ
func (c *coursesvcs) UpdateCourseFaq(ctx context.Context, courseId, faqId string, data *models.CourseFaqDto) (*models.CourseFaq, error) {
	cId, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return nil, errors.New("invalid course id")
	}

	fId, err := primitive.ObjectIDFromHex(faqId)
	if err != nil {
		return nil, errors.New("invalid faq id")
	}

	// Add authorization check
	canEdit, err := c.authSvc.CanEditCourseResources(ctx, courseId)
	if err != nil {
		return nil, err
	}
	if !canEdit {
		return nil, errors.New("unauthorized to edit resources for this course")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	courseFaq := &models.CourseFaq{
		Id:           fId,
		CourseId:     cId,
		CourseFaqDto: *data,
		UpdatedAt:    time.Now(),
		UpdatedBy:    cfg.User.Id,
	}

	filter := bson.M{"courseId": cId, "_id": fId, "trash": false}
	update := bson.M{"$set": courseFaq}

	updatedCourseFaq, err := c.courseFaqRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}
	return updatedCourseFaq, nil
}

// DeleteCourseFaq soft deletes a course FAQ
func (c *coursesvcs) DeleteCourseFaq(ctx context.Context, courseId, faqId string) error {
	cId, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return errors.New("invalid course id")
	}

	fId, err := primitive.ObjectIDFromHex(faqId)
	if err != nil {
		return errors.New("invalid faq id")
	}

	// Add authorization check
	canEdit, err := c.authSvc.CanEditCourseResources(ctx, courseId)
	if err != nil {
		return err
	}
	if !canEdit {
		return errors.New("unauthorized to edit resources for this course")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	filter := bson.M{"courseId": cId, "_id": fId, "trash": false}
	update := bson.M{"$set": bson.M{"trash": true, "updatedAt": time.Now(), "updatedBy": cfg.User.Id}}

	_, err = c.courseFaqRepo.Patch(ctx, filter, update)
	return err
}

