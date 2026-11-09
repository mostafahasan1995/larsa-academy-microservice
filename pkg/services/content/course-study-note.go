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

// GetAllCourseStudyNotes retrieves all study notes for a course
func (c *coursesvcs) GetAllCourseStudyNotes(ctx context.Context, courseId string, query *query.Conditions) ([]models.CourseStudyNote, error) {
	// Add authorization check - users who can view the course can see study notes
	canView, err := c.authSvc.CanViewCourse(ctx, courseId)
	if err != nil {
		return nil, err
	}
	if !canView {
		return nil, errors.New("unauthorized to view this course")
	}

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

	var result []models.CourseStudyNote
	err = c.courseStudyNoteRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// AddCourseStudyNote adds a new study note to a course
func (c *coursesvcs) AddCourseStudyNote(ctx context.Context, courseId string, data *models.CourseStudyNoteDto) (*models.CourseStudyNote, error) {
	// Add authorization check - users who can edit course resources
	canEdit, err := c.authSvc.CanEditCourseResources(ctx, courseId)
	if err != nil {
		return nil, err
	}
	if !canEdit {
		return nil, errors.New("unauthorized to edit course resources")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	_id, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return nil, errors.New("invalid course id")
	}

	studyNote := &models.CourseStudyNote{
		Id:                 primitive.NewObjectID(),
		CourseId:           _id,
		CourseStudyNoteDto: *data,
		CreatedAt:          time.Now(),
		CreatedBy:          cfg.User.Id,
		UpdatedAt:          time.Now(),
		UpdatedBy:          cfg.User.Id,
	}

	if err := c.courseStudyNoteRepo.Add(ctx, studyNote); err != nil {
		return nil, err
	}

	return studyNote, nil
}

// UpdateCourseStudyNote updates an existing course study note
func (c *coursesvcs) UpdateCourseStudyNote(ctx context.Context, courseId, studyNoteId string, data *models.CourseStudyNoteDto) (*models.CourseStudyNote, error) {
	// Add authorization check - users who can edit course resources
	canEdit, err := c.authSvc.CanEditCourseResources(ctx, courseId)
	if err != nil {
		return nil, err
	}
	if !canEdit {
		return nil, errors.New("unauthorized to edit course resources")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	cId, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return nil, errors.New("invalid course id")
	}

	sId, err := primitive.ObjectIDFromHex(studyNoteId)
	if err != nil {
		return nil, errors.New("invalid study note id")
	}

	studyNote := &models.CourseStudyNote{
		Id:                 sId,
		CourseId:           cId,
		CourseStudyNoteDto: *data,
		UpdatedAt:          time.Now(),
		UpdatedBy:          cfg.User.Id,
	}

	filter := bson.M{"courseId": cId, "_id": sId, "trash": false}
	update := bson.M{"$set": studyNote}

	updatedStudyNote, err := c.courseStudyNoteRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	return updatedStudyNote, nil
}

// DeleteCourseStudyNote soft deletes a course study note
func (c *coursesvcs) DeleteCourseStudyNote(ctx context.Context, courseId, studyNoteId string) error {
	// Add authorization check - users who can edit course resources
	canEdit, err := c.authSvc.CanEditCourseResources(ctx, courseId)
	if err != nil {
		return err
	}
	if !canEdit {
		return errors.New("unauthorized to edit course resources")
	}

	cId, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return errors.New("invalid course id")
	}
	sId, err := primitive.ObjectIDFromHex(studyNoteId)
	if err != nil {
		return errors.New("invalid study note id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	filter := bson.M{"courseId": cId, "_id": sId, "trash": false}
	update := bson.M{"$set": bson.M{"trash": true, "updatedAt": time.Now(), "updatedBy": cfg.User.Id}}

	_, err = c.courseStudyNoteRepo.Patch(ctx, filter, update)
	return err
}

// GetAllCourseStudyNoteCategories retrieves all study note categories for a course
func (c *coursesvcs) GetAllCourseStudyNoteCategories(ctx context.Context, courseId string, query *query.Conditions) ([]models.CourseStudyNoteCategory, error) {
	// Add authorization check - users who can view the course can see categories
	canView, err := c.authSvc.CanViewCourse(ctx, courseId)
	if err != nil {
		return nil, err
	}
	if !canView {
		return nil, errors.New("unauthorized to view this course")
	}

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

	var result []models.CourseStudyNoteCategory
	err = c.courseStudyNoteCategoryRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// AddCourseStudyNoteCategory adds a new study note category to a course
func (c *coursesvcs) AddCourseStudyNoteCategory(ctx context.Context, courseId string, data *models.CourseStudyNoteCategoryDto) (*models.CourseStudyNoteCategory, error) {
	// Add authorization check - users who can edit course resources
	canEdit, err := c.authSvc.CanEditCourseResources(ctx, courseId)
	if err != nil {
		return nil, err
	}
	if !canEdit {
		return nil, errors.New("unauthorized to edit course resources")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	_id, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return nil, errors.New("invalid course id")
	}

	category := &models.CourseStudyNoteCategory{
		Id:                         primitive.NewObjectID(),
		CouserId:                   _id,
		CourseStudyNoteCategoryDto: *data,
		CreatedAt:                  time.Now(),
		CreatedBy:                  cfg.User.Id,
		UpdatedAt:                  time.Now(),
		UpdatedBy:                  cfg.User.Id,
	}

	if err := c.courseStudyNoteCategoryRepo.Add(ctx, category); err != nil {
		return nil, err
	}

	return category, nil
}

// UpdateCourseStudyNoteCategory updates an existing study note category
func (c *coursesvcs) UpdateCourseStudyNoteCategory(ctx context.Context, courseId, categoryId string, data *models.CourseStudyNoteCategoryDto) (*models.CourseStudyNoteCategory, error) {
	// Add authorization check - users who can edit course resources
	canEdit, err := c.authSvc.CanEditCourseResources(ctx, courseId)
	if err != nil {
		return nil, err
	}
	if !canEdit {
		return nil, errors.New("unauthorized to edit course resources")
	}

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

	category := &models.CourseStudyNoteCategory{
		Id:                         catId,
		CouserId:                   cId,
		CourseStudyNoteCategoryDto: *data,
		UpdatedAt:                  time.Now(),
		UpdatedBy:                  cfg.User.Id,
	}

	filter := bson.M{"courseId": cId, "_id": catId, "trash": false}
	update := bson.M{"$set": category}

	updatedCategory, err := c.courseStudyNoteCategoryRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	return updatedCategory, nil
}

// DeleteCourseStudyNoteCategory soft deletes a study note category
func (c *coursesvcs) DeleteCourseStudyNoteCategory(ctx context.Context, courseId, categoryId string) error {
	// Add authorization check - users who can edit course resources
	canEdit, err := c.authSvc.CanEditCourseResources(ctx, courseId)
	if err != nil {
		return err
	}
	if !canEdit {
		return errors.New("unauthorized to edit course resources")
	}

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

	_, err = c.courseStudyNoteCategoryRepo.Patch(ctx, filter, update)
	return err
}

