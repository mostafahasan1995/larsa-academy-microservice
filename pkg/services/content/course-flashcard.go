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

// GetAllCourseFlashcards retrieves all flashcards for a course
func (c *coursesvcs) GetAllCourseFlashcards(ctx context.Context, courseId string, query *query.Conditions) ([]models.CourseFlashcard, error) {
	_id, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return nil, errors.New("invalid course id")
	}

	if err := query.CheckValid(); err != nil {
		return nil, errors.New("invalid query")
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, errors.New("invalid convert to mongo filters")
	}

	pipeline := []bson.M{
		{"$match": bson.M{"courseId": _id, "trash": false}},
		{"$match": filter},
	}

	var result []models.CourseFlashcard
	err = c.courseFlashcardRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// AddCourseFlashcard adds a new flashcard to a course
func (c *coursesvcs) AddCourseFlashcard(ctx context.Context, courseId string, data *models.CourseFlashcardDto) (*models.CourseFlashcard, error) {
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
		return nil, errors.New("invalid course id")
	}

	flashcard := &models.CourseFlashcard{
		Id:                 primitive.NewObjectID(),
		CourseId:           _id,
		CourseFlashcardDto: *data,
		CreatedAt:          time.Now(),
		CreatedBy:          cfg.User.Id,
		UpdatedAt:          time.Now(),
		UpdatedBy:          cfg.User.Id,
	}

	if err := c.courseFlashcardRepo.Add(ctx, flashcard); err != nil {
		return nil, err
	}

	return flashcard, nil
}

// UpdateCourseFlashcard updates an existing course flashcard
func (c *coursesvcs) UpdateCourseFlashcard(ctx context.Context, courseId, flashcardId string, data *models.CourseFlashcardDto) (*models.CourseFlashcard, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	cId, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return nil, errors.New("invalid course id")
	}

	fId, err := primitive.ObjectIDFromHex(flashcardId)
	if err != nil {
		return nil, errors.New("invalid flashcard id")
	}

	flashcard := &models.CourseFlashcard{
		Id:                 fId,
		CourseId:           cId,
		CourseFlashcardDto: *data,
		UpdatedAt:          time.Now(),
		UpdatedBy:          cfg.User.Id,
	}

	filter := bson.M{"courseId": cId, "_id": fId, "trash": false}
	update := bson.M{"$set": flashcard}

	updatedFlashcard, err := c.courseFlashcardRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	return updatedFlashcard, nil
}

// DeleteCourseFlashcard soft deletes a course flashcard
func (c *coursesvcs) DeleteCourseFlashcard(ctx context.Context, courseId, flashcardId string) error {
	cId, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return errors.New("invalid course id")
	}
	fId, err := primitive.ObjectIDFromHex(flashcardId)
	if err != nil {
		return errors.New("invalid flashcard id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	filter := bson.M{"courseId": cId, "_id": fId, "trash": false}
	update := bson.M{"$set": bson.M{"trash": true, "updatedAt": time.Now(), "updatedBy": cfg.User.Id}}

	_, err = c.courseFlashcardRepo.Patch(ctx, filter, update)
	return err
}

// GetAllCourseFlashcardCategories retrieves all flashcard categories for a course
func (c *coursesvcs) GetAllCourseFlashcardCategories(ctx context.Context, courseId string, query *query.Conditions) ([]models.CourseFlashcardCategory, error) {
	_id, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return nil, errors.New("invalid course id")
	}

	if err := query.CheckValid(); err != nil {
		return nil, errors.New("invalid query")
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, errors.New("invalid convert to mongo filters")
	}

	pipeline := []bson.M{
		{"$match": bson.M{"courseId": _id, "trash": false}},
		{"$match": filter},
	}

	var result []models.CourseFlashcardCategory
	err = c.courseFlashcardCategoryRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// AddCourseFlashcardCategory adds a new flashcard category to a course
func (c *coursesvcs) AddCourseFlashcardCategory(ctx context.Context, courseId string, data *models.CourseFlashcardCategoryDto) (*models.CourseFlashcardCategory, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	_id, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return nil, errors.New("invalid course id")
	}

	category := &models.CourseFlashcardCategory{
		Id:                         primitive.NewObjectID(),
		CouserId:                   _id,
		CourseFlashcardCategoryDto: *data,
		CreatedAt:                  time.Now(),
		CreatedBy:                  cfg.User.Id,
		UpdatedAt:                  time.Now(),
		UpdatedBy:                  cfg.User.Id,
	}

	if err := c.courseFlashcardCategoryRepo.Add(ctx, category); err != nil {
		return nil, err
	}

	return category, nil
}

// UpdateCourseFlashcardCategory updates an existing flashcard category
func (c *coursesvcs) UpdateCourseFlashcardCategory(ctx context.Context, courseId, categoryId string, data *models.CourseFlashcardCategoryDto) (*models.CourseFlashcardCategory, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	cId, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return nil, errors.New("invalid course id")
	}

	catId, err := primitive.ObjectIDFromHex(categoryId)
	if err != nil {
		return nil, errors.New("invalid category id")
	}

	category := &models.CourseFlashcardCategory{
		Id:                         catId,
		CouserId:                   cId,
		CourseFlashcardCategoryDto: *data,
		UpdatedAt:                  time.Now(),
		UpdatedBy:                  cfg.User.Id,
	}

	filter := bson.M{"courseId": cId, "_id": catId, "trash": false}
	update := bson.M{"$set": category}

	updatedCategory, err := c.courseFlashcardCategoryRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	return updatedCategory, nil
}

// DeleteCourseFlashcardCategory soft deletes a flashcard category
func (c *coursesvcs) DeleteCourseFlashcardCategory(ctx context.Context, courseId, categoryId string) error {
	cId, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return errors.New("invalid course id")
	}

	catId, err := primitive.ObjectIDFromHex(categoryId)
	if err != nil {
		return errors.New("invalid category id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	filter := bson.M{"courseId": cId, "_id": catId, "trash": false}
	update := bson.M{"$set": bson.M{"trash": true, "updatedAt": time.Now(), "updatedBy": cfg.User.Id}}

	_, err = c.courseFlashcardCategoryRepo.Patch(ctx, filter, update)
	return err
}

