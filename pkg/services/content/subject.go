package content

import (
	"context"
	"errors"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"math"
	"time"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type SubjectSvcs interface {
	GetById(ctx context.Context, subjectId string) (*models.Subject, error)
	Get(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.SubjectPagination, error)
	GetAll(ctx context.Context, query *query.Conditions) ([]models.Subject, error)
	Add(ctx context.Context, data *models.SubjectDto) (*models.Subject, error)
	Update(ctx context.Context, subjectId string, data *models.SubjectDto) (*models.Subject, error)
	Delete(ctx context.Context, subjectId string) error
	//subject study note
	// GetAllSubjectStudyNotes(ctx context.Context, subjectId string, query *query.Conditions) ([]models.SubjectStudyNote, error)
	// GetSubjectStudyNote(ctx context.Context, subjectId, subjectStudyNoteId string) (*models.SubjectStudyNote, error)
	// AddSubjectStudyNote(ctx context.Context, subjectId string, data *models.SubjectStudyNoteDto) (*models.SubjectStudyNote, error)
	// UpdateSubjectStudyNote(ctx context.Context, subjectId, subjectStudyNoteId string, data *models.SubjectStudyNoteDto) (*models.SubjectStudyNote, error)
	// DeleteSubjectStudyNote(ctx context.Context, subjectId, subjectStudyNoteId string) error
	// //subject study note category
	// GetAllSubjectStudyNoteCategories(ctx context.Context, subjectId string, query *query.Conditions) ([]models.SubjectStudyNoteCategory, error)
	// AddSubjectStudyNoteCategory(ctx context.Context, subjectId string, data *models.SubjectStudyNoteCategoryDto) (*models.SubjectStudyNoteCategory, error)
	// UpdateSubjectStudyNoteCategory(ctx context.Context, subjectId, subjectStudyNoteCategoryId string, data *models.SubjectStudyNoteCategoryDto) (*models.SubjectStudyNoteCategory, error)
	// DeleteSubjectStudyNoteCategory(ctx context.Context, subjectId, subjectStudyNoteCategoryId string) error
	// //subject flashcard
	// GetAllSubjectFlashcards(ctx context.Context, subjectId string, query *query.Conditions) ([]models.SubjectFlashcard, error)
	// GetSubjectFlashcard(ctx context.Context, subjectId, flashcardId string) (*models.SubjectFlashcard, error)
	// AddSubjectFlashcard(ctx context.Context, subjectId string, data *models.SubjectFlashcardDto) (*models.SubjectFlashcard, error)
	// UpdateSubjectFlashcard(ctx context.Context, subjectId, flashcardId string, data *models.SubjectFlashcardDto) (*models.SubjectFlashcard, error)
	// DeleteSubjectFlashcard(ctx context.Context, subjectId, flashcardId string) error
	// //subject flashcard category
	// GetAllSubjectFlashcardCategories(ctx context.Context, subjectId string, query *query.Conditions) ([]models.SubjectFlashcardCategory, error)
	// AddSubjectFlashcardCategory(ctx context.Context, subjectId string, data *models.SubjectFlashcardCategoryDto) (*models.SubjectFlashcardCategory, error)
	// UpdateSubjectFlashcardCategory(ctx context.Context, subjectId, categoryId string, data *models.SubjectFlashcardCategoryDto) (*models.SubjectFlashcardCategory, error)
	// DeleteSubjectFlashcardCategory(ctx context.Context, subjectId, categoryId string) error
	// //subject assignments
	// GetAllSubjectAssignments(ctx context.Context, subjectId string, query *query.Conditions) ([]models.SubjectAssignment, error)
	// GetSubjectAssignment(ctx context.Context, subjectId, assignmentId string) (*models.SubjectAssignment, error)
	// AddSubjectAssignment(ctx context.Context, subjectId string, data *models.SubjectAssignmentDto) (*models.SubjectAssignment, error)
	// UpdateSubjectAssignment(ctx context.Context, subjectId, assignmentId string, data *models.SubjectAssignmentDto) (*models.SubjectAssignment, error)
	// DeleteSubjectAssignment(ctx context.Context, subjectId, assignmentId string) error
	// //subject faqs
	// GetAllSubjectFAQs(ctx context.Context, subjectId string, query *query.Conditions) ([]models.SubjectFAQ, error)
	// GetSubjectFAQ(ctx context.Context, subjectId, faqId string) (*models.SubjectFAQ, error)
	// AddSubjectFAQ(ctx context.Context, subjectId string, data *models.SubjectFAQDto) (*models.SubjectFAQ, error)
	// UpdateSubjectFAQ(ctx context.Context, subjectId, faqId string, data *models.SubjectFAQDto) (*models.SubjectFAQ, error)
	//DeleteSubjectFAQ(ctx context.Context, subjectId, faqId string) error
}

type subjectsvcs struct {
	subjectrepo db.MainRepo[models.Subject]
	// subjectStudyNoterepo         db.MainRepo[models.SubjectStudyNote]
	// subjectStudyNoteCategoryrepo db.MainRepo[models.SubjectStudyNoteCategory]
	// subjectFlashcardrepo         db.MainRepo[models.SubjectFlashcard]
	// subjectFlashcardCategoryrepo db.MainRepo[models.SubjectFlashcardCategory]
	// subjectAssignmentrepo        db.MainRepo[models.SubjectAssignment]
	// subjectFaqrepo               db.MainRepo[models.SubjectFAQ]
}

func NewSubjectSvcs(i *do.Injector) (SubjectSvcs, error) {
	return &subjectsvcs{
		subjectrepo: db.NewRepo[models.Subject](i),
		// subjectStudyNoterepo:         db.NewRepo[models.SubjectStudyNote](i),
		// subjectStudyNoteCategoryrepo: db.NewRepo[models.SubjectStudyNoteCategory](i),
		// subjectFlashcardrepo:         db.NewRepo[models.SubjectFlashcard](i),
		// subjectFlashcardCategoryrepo: db.NewRepo[models.SubjectFlashcardCategory](i),
		// subjectAssignmentrepo:        db.NewRepo[models.SubjectAssignment](i),
		// subjectFaqrepo:               db.NewRepo[models.SubjectFAQ](i),
	}, nil
}

func (s *subjectsvcs) GetById(ctx context.Context, subjectId string) (*models.Subject, error) {
	_id, err := primitive.ObjectIDFromHex(subjectId)
	if err != nil {
		return nil, errors.New("invalid subject id")
	}

	return s.subjectrepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
}

func (s *subjectsvcs) Get(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.SubjectPagination, error) {
	if err := query.CheckValid(); err != nil {
		return nil, errors.New("invalid query")
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, errors.New("invalid convert to mongo filters")
	}
	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
		{"$match": filter},
	}

	count, err := s.subjectrepo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})
	pipeline = append(pipeline, bson.M{"$skip": skip})
	pipeline = append(pipeline, bson.M{"$limit": limit})

	var result []models.Subject
	err = s.subjectrepo.Aggregate(ctx, pipeline, func(cursor *mongo.Cursor) error {
		return cursor.All(ctx, &result)
	})
	if err != nil {
		return nil, err
	}

	var totalPages float64 = math.Ceil(float64(count) / float64(limit))
	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: count,
	}

	return &models.SubjectPagination{
		Subjects:   result,
		Pagination: pagination,
	}, nil
}

func (s *subjectsvcs) GetAll(ctx context.Context, query *query.Conditions) ([]models.Subject, error) {
	if err := query.CheckValid(); err != nil {
		return nil, errors.New("invalid query")
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, errors.New("invalid convert to mongo filters")
	}

	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
	}
	if len(filter) > 0 {
		pipeline = append(pipeline, bson.M{"$match": filter})
	}
	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})

	var result []models.Subject
	err = s.subjectrepo.Aggregate(ctx, pipeline, func(cursor *mongo.Cursor) error {
		return cursor.All(ctx, &result)
	})
	if err != nil {
		return nil, errors.New("failed to fetch subjects: " + err.Error())
	}

	return result, nil
}

func (s *subjectsvcs) Add(ctx context.Context, data *models.SubjectDto) (*models.Subject, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	subject := &models.Subject{
		Id:         primitive.NewObjectID(),
		SubjectDto: *data,
		CreatedBy:  cfg.User.Id,
		UpdatedBy:  cfg.User.Id,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := s.subjectrepo.Add(ctx, subject); err != nil {
		return nil, err
	}
	return subject, nil

}

func (s *subjectsvcs) Update(ctx context.Context, subjectId string, data *models.SubjectDto) (*models.Subject, error) {
	_id, err := primitive.ObjectIDFromHex(subjectId)
	if err != nil {
		return nil, errors.New("invalid subject id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	subject := &models.Subject{
		Id:         _id,
		SubjectDto: *data,
		UpdatedBy:  cfg.User.Id,
		UpdatedAt:  time.Now(),
	}

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": subject}

	updatedSubject, err := s.subjectrepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}
	return updatedSubject, nil

}

func (s *subjectsvcs) Delete(ctx context.Context, subjectId string) error {
	_id, err := primitive.ObjectIDFromHex(subjectId)
	if err != nil {
		return errors.New("invalid subject id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": bson.M{"trash": true, "updatedBy": cfg.User.Id, "updatedAt": time.Now()}}

	_, err = s.subjectrepo.Patch(ctx, filter, update)

	if err != nil {
		return err
	}

	return nil
}

// subject study note
// func (s *subjectsvcs) GetAllSubjectStudyNotes(ctx context.Context, subjectId string, query *query.Conditions) ([]models.SubjectStudyNote, error) {
// 	_id, err := primitive.ObjectIDFromHex(subjectId)
// 	if err != nil {
// 		return nil, errors.New("invalid subject id")
// 	}

// 	if err := query.CheckValid(); err != nil {
// 		return nil, errors.New("invalid query")
// 	}

// 	filter, err := query.ConvertToMongo()
// 	if err != nil {
// 		return nil, errors.New("invalid convert to mongo filters")
// 	}
// 	pipeline := []bson.M{
// 		{"$match": bson.M{"subjectId": _id, "trash": false}},
// 		{"$match": filter},
// 		{"$sort": bson.M{"_id": -1}},
// 	}

// 	var result []models.SubjectStudyNote
// 	err = s.subjectStudyNoterepo.Aggregate(ctx, pipeline, func(cursor *mongo.Cursor) error {
// 		return cursor.All(ctx, &result)
// 	})
// 	if err != nil {
// 		return nil, err
// 	}

// 	return result, nil
// }

// func (s *subjectsvcs) GetSubjectStudyNote(ctx context.Context, subjectId, subjectStudyNoteId string) (*models.SubjectStudyNote, error) {
// 	sId, err := primitive.ObjectIDFromHex(subjectId)
// 	if err != nil {
// 		return nil, errors.New("invalid subject id")
// 	}

// 	snId, err := primitive.ObjectIDFromHex(subjectStudyNoteId)
// 	if err != nil {
// 		return nil, errors.New("invalid subject study note id")
// 	}

// 	return s.subjectStudyNoterepo.GetByFilter(ctx, bson.M{"subjectId": sId, "_id": snId, "trash": false})
// }

// func (s *subjectsvcs) AddSubjectStudyNote(ctx context.Context, subjectId string, data *models.SubjectStudyNoteDto) (*models.SubjectStudyNote, error) {
// 	sId, err := primitive.ObjectIDFromHex(subjectId)
// 	if err != nil {
// 		return nil, errors.New("invalid subject id")
// 	}

// 	cfg, err := util.GetReqAppCfg(ctx)
// 	if err != nil {
// 		return nil, err
// 	}

// 	subjectStudyNote := &models.SubjectStudyNote{
// 		Id:                  primitive.NewObjectID(),
// 		SubjectId:           sId,
// 		SubjectStudyNoteDto: *data,
// 		CreatedBy:           cfg.User.Id,
// 		UpdatedBy:           cfg.User.Id,
// 		CreatedAt:           time.Now(),
// 		UpdatedAt:           time.Now(),
// 	}

// 	if err := s.subjectStudyNoterepo.Add(ctx, subjectStudyNote); err != nil {
// 		return nil, err
// 	}

// 	return subjectStudyNote, nil

// }

// func (s *subjectsvcs) UpdateSubjectStudyNote(ctx context.Context, subjectId, subjectStudyNoteId string, data *models.SubjectStudyNoteDto) (*models.SubjectStudyNote, error) {
// 	sId, err := primitive.ObjectIDFromHex(subjectId)
// 	if err != nil {
// 		return nil, errors.New("invalid subject id")
// 	}

// 	snId, err := primitive.ObjectIDFromHex(subjectStudyNoteId)
// 	if err != nil {
// 		return nil, errors.New("invalid subject study note id")
// 	}

// 	cfg, err := util.GetReqAppCfg(ctx)
// 	if err != nil {
// 		return nil, err
// 	}

// 	subjectStudyNote := &models.SubjectStudyNote{
// 		Id:                  snId,
// 		SubjectId:           sId,
// 		SubjectStudyNoteDto: *data,
// 		UpdatedBy:           cfg.User.Id,
// 		UpdatedAt:           time.Now(),
// 	}

// 	filter := bson.M{"subjectId": sId, "_id": snId, "trash": false}
// 	update := bson.M{"$set": subjectStudyNote}

// 	updatedSubjectStudyNote, err := s.subjectStudyNoterepo.Patch(ctx, filter, update)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return updatedSubjectStudyNote, nil

// }

// func (s *subjectsvcs) DeleteSubjectStudyNote(ctx context.Context, subjectId, subjectStudyNoteId string) error {
// 	sId, err := primitive.ObjectIDFromHex(subjectId)
// 	if err != nil {
// 		return errors.New("invalid subject id")
// 	}

// 	snId, err := primitive.ObjectIDFromHex(subjectStudyNoteId)
// 	if err != nil {
// 		return errors.New("invalid subject study note id")
// 	}

// 	cfg, err := util.GetReqAppCfg(ctx)
// 	if err != nil {
// 		return err
// 	}

// 	filter := bson.M{"subjectId": sId, "_id": snId, "trash": false}
// 	update := bson.M{"$set": bson.M{"trash": true, "updatedBy": cfg.User.Id, "updatedAt": time.Now()}}

// 	_, err = s.subjectStudyNoterepo.Patch(ctx, filter, update)
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }

// //subject study note category

// func (s *subjectsvcs) GetAllSubjectStudyNoteCategories(ctx context.Context, subjectId string, query *query.Conditions) ([]models.SubjectStudyNoteCategory, error) {
// 	_id, err := primitive.ObjectIDFromHex(subjectId)
// 	if err != nil {
// 		return nil, errors.New("invalid subject id")
// 	}

// 	if err := query.CheckValid(); err != nil {
// 		return nil, errors.New("invalid query")
// 	}

// 	filter, err := query.ConvertToMongo()
// 	if err != nil {
// 		return nil, errors.New("invalid convert to mongo filters")
// 	}

// 	pipeline := []bson.M{
// 		{"$match": bson.M{"subjectId": _id, "trash": false}},
// 		{"$match": filter},
// 		{"$sort": bson.M{"_id": -1}},
// 	}

// 	var result []models.SubjectStudyNoteCategory
// 	err = s.subjectStudyNoteCategoryrepo.Aggregate(ctx, pipeline, func(cursor *mongo.Cursor) error {
// 		return cursor.All(ctx, &result)
// 	})
// 	if err != nil {
// 		return nil, err
// 	}

// 	return result, nil
// }

// func (s *subjectsvcs) AddSubjectStudyNoteCategory(ctx context.Context, subjectId string, data *models.SubjectStudyNoteCategoryDto) (*models.SubjectStudyNoteCategory, error) {
// 	_id, err := primitive.ObjectIDFromHex(subjectId)
// 	if err != nil {
// 		return nil, errors.New("invalid subject id")
// 	}

// 	cfg, err := util.GetReqAppCfg(ctx)
// 	if err != nil {
// 		return nil, err
// 	}

// 	subjectStudyNoteCategory := &models.SubjectStudyNoteCategory{
// 		Id:                          primitive.NewObjectID(),
// 		SubjectId:                   _id,
// 		SubjectStudyNoteCategoryDto: *data,
// 		CreatedBy:                   cfg.User.Id,
// 		UpdatedBy:                   cfg.User.Id,
// 		CreatedAt:                   time.Now(),
// 		UpdatedAt:                   time.Now(),
// 	}

// 	if err := s.subjectStudyNoteCategoryrepo.Add(ctx, subjectStudyNoteCategory); err != nil {
// 		return nil, err
// 	}

// 	return subjectStudyNoteCategory, nil

// }

// func (s *subjectsvcs) UpdateSubjectStudyNoteCategory(ctx context.Context, subjectId, subjectStudyNoteCategoryId string, data *models.SubjectStudyNoteCategoryDto) (*models.SubjectStudyNoteCategory, error) {
// 	sid, err := primitive.ObjectIDFromHex(subjectId)
// 	if err != nil {
// 		return nil, errors.New("invalid subject id")
// 	}

// 	sncId, err := primitive.ObjectIDFromHex(subjectStudyNoteCategoryId)
// 	if err != nil {
// 		return nil, errors.New("invalid subject study note category id")
// 	}

// 	cfg, err := util.GetReqAppCfg(ctx)
// 	if err != nil {
// 		return nil, err
// 	}

// 	subjectStudyNoteCategory := &models.SubjectStudyNoteCategory{
// 		Id:                          sncId,
// 		SubjectId:                   sid,
// 		SubjectStudyNoteCategoryDto: *data,
// 		UpdatedBy:                   cfg.User.Id,
// 		UpdatedAt:                   time.Now(),
// 	}

// 	filter := bson.M{"subjectId": sid, "_id": sncId, "trash": false}
// 	update := bson.M{"$set": subjectStudyNoteCategory}

// 	updatedSubjectStudyNoteCategory, err := s.subjectStudyNoteCategoryrepo.Patch(ctx, filter, update)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return updatedSubjectStudyNoteCategory, nil

// }

// func (s *subjectsvcs) DeleteSubjectStudyNoteCategory(ctx context.Context, subjectId, subjectStudyNoteCategoryId string) error {
// 	sid, err := primitive.ObjectIDFromHex(subjectId)
// 	if err != nil {
// 		return errors.New("invalid subject id")
// 	}

// 	sncId, err := primitive.ObjectIDFromHex(subjectStudyNoteCategoryId)
// 	if err != nil {
// 		return errors.New("invalid subject study note category id")
// 	}

// 	cfg, err := util.GetReqAppCfg(ctx)
// 	if err != nil {
// 		return err
// 	}

// 	filter := bson.M{"subjectId": sid, "_id": sncId, "trash": false}
// 	update := bson.M{"$set": bson.M{"trash": true, "updatedBy": cfg.User.Id, "updatedAt": time.Now()}}

// 	_, err = s.subjectStudyNoteCategoryrepo.Patch(ctx, filter, update)
// 	if err != nil {
// 		return err
// 	}

// 	return nil

// }

// //subject flashcard

// func (s *subjectsvcs) GetAllSubjectFlashcards(ctx context.Context, subjectId string, query *query.Conditions) ([]models.SubjectFlashcard, error) {
// 	_id, err := primitive.ObjectIDFromHex(subjectId)
// 	if err != nil {
// 		return nil, errors.New("invalid subject id")
// 	}

// 	if err := query.CheckValid(); err != nil {
// 		return nil, errors.New("invalid query")
// 	}

// 	filter, err := query.ConvertToMongo()
// 	if err != nil {
// 		return nil, errors.New("invalid convert to mongo filters")
// 	}

// 	pipeline := []bson.M{
// 		{"$match": bson.M{"subjectId": _id, "trash": false}},
// 		{"$match": filter},
// 		{"$sort": bson.M{"_id": -1}},
// 	}

// 	var result []models.SubjectFlashcard
// 	err = s.subjectFlashcardrepo.Aggregate(ctx, pipeline, func(cursor *mongo.Cursor) error {
// 		return cursor.All(ctx, &result)
// 	})
// 	if err != nil {
// 		return nil, err
// 	}

// 	return result, nil
// }

// func (s *subjectsvcs) GetSubjectFlashcard(ctx context.Context, subjectId, flashcardId string) (*models.SubjectFlashcard, error) {
// 	sId, err := primitive.ObjectIDFromHex(subjectId)
// 	if err != nil {
// 		return nil, errors.New("invalid subject id")
// 	}

// 	fId, err := primitive.ObjectIDFromHex(flashcardId)
// 	if err != nil {
// 		return nil, errors.New("invalid flashcard id")
// 	}

// 	return s.subjectFlashcardrepo.GetByFilter(ctx, bson.M{"subjectId": sId, "_id": fId, "trash": false})
// }

// func (s *subjectsvcs) AddSubjectFlashcard(ctx context.Context, subjectId string, data *models.SubjectFlashcardDto) (*models.SubjectFlashcard, error) {
// 	sId, err := primitive.ObjectIDFromHex(subjectId)
// 	if err != nil {
// 		return nil, errors.New("invalid subject id")
// 	}

// 	cfg, err := util.GetReqAppCfg(ctx)
// 	if err != nil {
// 		return nil, err
// 	}

// 	flashcard := &models.SubjectFlashcard{
// 		Id:                  primitive.NewObjectID(),
// 		SubjectId:           sId,
// 		SubjectFlashcardDto: *data,
// 		CreatedBy:           cfg.User.Id,
// 		UpdatedBy:           cfg.User.Id,
// 		CreatedAt:           time.Now(),
// 		UpdatedAt:           time.Now(),
// 	}

// 	if err := s.subjectFlashcardrepo.Add(ctx, flashcard); err != nil {
// 		return nil, err
// 	}

// 	return flashcard, nil
// }

// func (s *subjectsvcs) UpdateSubjectFlashcard(ctx context.Context, subjectId, flashcardId string, data *models.SubjectFlashcardDto) (*models.SubjectFlashcard, error) {
// 	sId, err := primitive.ObjectIDFromHex(subjectId)
// 	if err != nil {
// 		return nil, errors.New("invalid subject id")
// 	}

// 	fId, err := primitive.ObjectIDFromHex(flashcardId)
// 	if err != nil {
// 		return nil, errors.New("invalid flashcard id")
// 	}

// 	cfg, err := util.GetReqAppCfg(ctx)
// 	if err != nil {
// 		return nil, err
// 	}

// 	flashcard := &models.SubjectFlashcard{
// 		Id:                  fId,
// 		SubjectId:           sId,
// 		SubjectFlashcardDto: *data,
// 		UpdatedBy:           cfg.User.Id,
// 		UpdatedAt:           time.Now(),
// 	}

// 	filter := bson.M{"subjectId": sId, "_id": fId, "trash": false}
// 	update := bson.M{"$set": flashcard}

// 	updatedFlashcard, err := s.subjectFlashcardrepo.Patch(ctx, filter, update)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return updatedFlashcard, nil
// }

// func (s *subjectsvcs) DeleteSubjectFlashcard(ctx context.Context, subjectId, flashcardId string) error {
// 	sId, err := primitive.ObjectIDFromHex(subjectId)
// 	if err != nil {
// 		return errors.New("invalid subject id")
// 	}

// 	fId, err := primitive.ObjectIDFromHex(flashcardId)
// 	if err != nil {
// 		return errors.New("invalid flashcard id")
// 	}

// 	cfg, err := util.GetReqAppCfg(ctx)
// 	if err != nil {
// 		return err
// 	}

// 	filter := bson.M{"subjectId": sId, "_id": fId, "trash": false}
// 	update := bson.M{"$set": bson.M{"trash": true, "updatedBy": cfg.User.Id, "updatedAt": time.Now()}}

// 	_, err = s.subjectFlashcardrepo.Patch(ctx, filter, update)
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }

// //subject flashcard category

// func (s *subjectsvcs) GetAllSubjectFlashcardCategories(ctx context.Context, subjectId string, query *query.Conditions) ([]models.SubjectFlashcardCategory, error) {
// 	_id, err := primitive.ObjectIDFromHex(subjectId)
// 	if err != nil {
// 		return nil, errors.New("invalid subject id")
// 	}

// 	if err := query.CheckValid(); err != nil {
// 		return nil, errors.New("invalid query")
// 	}

// 	filter, err := query.ConvertToMongo()
// 	if err != nil {
// 		return nil, errors.New("invalid convert to mongo filters")
// 	}

// 	pipeline := []bson.M{
// 		{"$match": bson.M{"subjectId": _id, "trash": false}},
// 		{"$match": filter},
// 		{"$sort": bson.M{"_id": -1}},
// 	}

// 	var result []models.SubjectFlashcardCategory
// 	err = s.subjectFlashcardCategoryrepo.Aggregate(ctx, pipeline, func(cursor *mongo.Cursor) error {
// 		return cursor.All(ctx, &result)
// 	})
// 	if err != nil {
// 		return nil, err
// 	}

// 	return result, nil
// }

// func (s *subjectsvcs) AddSubjectFlashcardCategory(ctx context.Context, subjectId string, data *models.SubjectFlashcardCategoryDto) (*models.SubjectFlashcardCategory, error) {
// 	_id, err := primitive.ObjectIDFromHex(subjectId)
// 	if err != nil {
// 		return nil, errors.New("invalid subject id")
// 	}

// 	cfg, err := util.GetReqAppCfg(ctx)
// 	if err != nil {
// 		return nil, err
// 	}

// 	category := &models.SubjectFlashcardCategory{
// 		Id:                          primitive.NewObjectID(),
// 		SubjectId:                   _id,
// 		SubjectFlashcardCategoryDto: *data,
// 		CreatedBy:                   cfg.User.Id,
// 		UpdatedBy:                   cfg.User.Id,
// 		CreatedAt:                   time.Now(),
// 		UpdatedAt:                   time.Now(),
// 	}

// 	if err := s.subjectFlashcardCategoryrepo.Add(ctx, category); err != nil {
// 		return nil, err
// 	}

// 	return category, nil
// }

// func (s *subjectsvcs) UpdateSubjectFlashcardCategory(ctx context.Context, subjectId, categoryId string, data *models.SubjectFlashcardCategoryDto) (*models.SubjectFlashcardCategory, error) {
// 	sId, err := primitive.ObjectIDFromHex(subjectId)
// 	if err != nil {
// 		return nil, errors.New("invalid subject id")
// 	}

// 	cId, err := primitive.ObjectIDFromHex(categoryId)
// 	if err != nil {
// 		return nil, errors.New("invalid category id")
// 	}

// 	cfg, err := util.GetReqAppCfg(ctx)
// 	if err != nil {
// 		return nil, err
// 	}

// 	category := &models.SubjectFlashcardCategory{
// 		Id:                          cId,
// 		SubjectId:                   sId,
// 		SubjectFlashcardCategoryDto: *data,
// 		UpdatedBy:                   cfg.User.Id,
// 		UpdatedAt:                   time.Now(),
// 	}

// 	filter := bson.M{"subjectId": sId, "_id": cId, "trash": false}
// 	update := bson.M{"$set": category}

// 	updatedCategory, err := s.subjectFlashcardCategoryrepo.Patch(ctx, filter, update)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return updatedCategory, nil
// }

// func (s *subjectsvcs) DeleteSubjectFlashcardCategory(ctx context.Context, subjectId, categoryId string) error {
// 	sId, err := primitive.ObjectIDFromHex(subjectId)
// 	if err != nil {
// 		return errors.New("invalid subject id")
// 	}

// 	cId, err := primitive.ObjectIDFromHex(categoryId)
// 	if err != nil {
// 		return errors.New("invalid category id")
// 	}

// 	cfg, err := util.GetReqAppCfg(ctx)
// 	if err != nil {
// 		return err
// 	}

// 	filter := bson.M{"subjectId": sId, "_id": cId, "trash": false}
// 	update := bson.M{"$set": bson.M{"trash": true, "updatedBy": cfg.User.Id, "updatedAt": time.Now()}}

// 	_, err = s.subjectFlashcardCategoryrepo.Patch(ctx, filter, update)
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }

// // Subject Assignment Methods
// func (s *subjectsvcs) GetAllSubjectAssignments(ctx context.Context, subjectId string, query *query.Conditions) ([]models.SubjectAssignment, error) {
// 	_id, err := primitive.ObjectIDFromHex(subjectId)
// 	if err != nil {
// 		return nil, errors.New("invalid subject id")
// 	}

// 	if err := query.CheckValid(); err != nil {
// 		return nil, errors.New("invalid query")
// 	}

// 	filter, err := query.ConvertToMongo()
// 	if err != nil {
// 		return nil, errors.New("invalid convert to mongo filters")
// 	}

// 	pipeline := []bson.M{
// 		{"$match": bson.M{"subjectId": _id, "trash": false}},
// 		{"$match": filter},
// 		{"$sort": bson.M{"_id": -1}},
// 	}

// 	var result []models.SubjectAssignment
// 	err = s.subjectAssignmentrepo.Aggregate(ctx, pipeline, func(cursor *mongo.Cursor) error {
// 		return cursor.All(ctx, &result)
// 	})
// 	if err != nil {
// 		return nil, err
// 	}

// 	return result, nil
// }

// func (s *subjectsvcs) GetSubjectAssignment(ctx context.Context, subjectId, assignmentId string) (*models.SubjectAssignment, error) {
// 	sId, err := primitive.ObjectIDFromHex(subjectId)
// 	if err != nil {
// 		return nil, errors.New("invalid subject id")
// 	}

// 	aId, err := primitive.ObjectIDFromHex(assignmentId)
// 	if err != nil {
// 		return nil, errors.New("invalid assignment id")
// 	}

// 	return s.subjectAssignmentrepo.GetByFilter(ctx, bson.M{"subjectId": sId, "_id": aId, "trash": false})
// }

// func (s *subjectsvcs) AddSubjectAssignment(ctx context.Context, subjectId string, data *models.SubjectAssignmentDto) (*models.SubjectAssignment, error) {
// 	sId, err := primitive.ObjectIDFromHex(subjectId)
// 	if err != nil {
// 		return nil, errors.New("invalid subject id")
// 	}

// 	cfg, err := util.GetReqAppCfg(ctx)
// 	if err != nil {
// 		return nil, err
// 	}

// 	assignment := &models.SubjectAssignment{
// 		Id:                   primitive.NewObjectID(),
// 		SubjectId:            sId,
// 		SubjectAssignmentDto: *data,
// 		CreatedBy:            cfg.User.Id,
// 		UpdatedBy:            cfg.User.Id,
// 		CreatedAt:            time.Now(),
// 		UpdatedAt:            time.Now(),
// 	}

// 	if err := s.subjectAssignmentrepo.Add(ctx, assignment); err != nil {
// 		return nil, err
// 	}

// 	return assignment, nil
// }

// func (s *subjectsvcs) UpdateSubjectAssignment(ctx context.Context, subjectId, assignmentId string, data *models.SubjectAssignmentDto) (*models.SubjectAssignment, error) {
// 	sId, err := primitive.ObjectIDFromHex(subjectId)
// 	if err != nil {
// 		return nil, errors.New("invalid subject id")
// 	}

// 	aId, err := primitive.ObjectIDFromHex(assignmentId)
// 	if err != nil {
// 		return nil, errors.New("invalid assignment id")
// 	}

// 	cfg, err := util.GetReqAppCfg(ctx)
// 	if err != nil {
// 		return nil, err
// 	}

// 	assignment := &models.SubjectAssignment{
// 		Id:                   aId,
// 		SubjectId:            sId,
// 		SubjectAssignmentDto: *data,
// 		UpdatedBy:            cfg.User.Id,
// 		UpdatedAt:            time.Now(),
// 	}

// 	filter := bson.M{"subjectId": sId, "_id": aId, "trash": false}
// 	update := bson.M{"$set": assignment}

// 	updatedAssignment, err := s.subjectAssignmentrepo.Patch(ctx, filter, update)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return updatedAssignment, nil
// }

// func (s *subjectsvcs) DeleteSubjectAssignment(ctx context.Context, subjectId, assignmentId string) error {
// 	sId, err := primitive.ObjectIDFromHex(subjectId)
// 	if err != nil {
// 		return errors.New("invalid subject id")
// 	}

// 	aId, err := primitive.ObjectIDFromHex(assignmentId)
// 	if err != nil {
// 		return errors.New("invalid assignment id")
// 	}

// 	cfg, err := util.GetReqAppCfg(ctx)
// 	if err != nil {
// 		return err
// 	}

// 	filter := bson.M{"subjectId": sId, "_id": aId, "trash": false}
// 	update := bson.M{"$set": bson.M{"trash": true, "updatedBy": cfg.User.Id, "updatedAt": time.Now()}}

// 	_, err = s.subjectAssignmentrepo.Patch(ctx, filter, update)
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }

// // subject faqs
// func (s *subjectsvcs) GetAllSubjectFAQs(ctx context.Context, subjectId string, query *query.Conditions) ([]models.SubjectFAQ, error) {
// 	_id, err := primitive.ObjectIDFromHex(subjectId)
// 	if err != nil {
// 		return nil, errors.New("invalid subject id")
// 	}

// 	if err := query.CheckValid(); err != nil {
// 		return nil, errors.New("invalid query")
// 	}

// 	filter, err := query.ConvertToMongo()
// 	if err != nil {
// 		return nil, errors.New("invalid convert to mongo filters")
// 	}

// 	pipeline := []bson.M{
// 		{"$match": bson.M{"subjectId": _id, "trash": false}},
// 		{"$match": filter},
// 		{"$sort": bson.M{"_id": -1}},
// 	}

// 	var result []models.SubjectFAQ
// 	err = s.subjectFaqrepo.Aggregate(ctx, pipeline, func(cursor *mongo.Cursor) error {
// 		return cursor.All(ctx, &result)
// 	})
// 	if err != nil {
// 		return nil, err
// 	}

// 	return result, nil
// }

// func (s *subjectsvcs) GetSubjectFAQ(ctx context.Context, subjectId, faqId string) (*models.SubjectFAQ, error) {
// 	sId, err := primitive.ObjectIDFromHex(subjectId)
// 	if err != nil {
// 		return nil, errors.New("invalid subject id")
// 	}

// 	fId, err := primitive.ObjectIDFromHex(faqId)
// 	if err != nil {
// 		return nil, errors.New("invalid faq id")
// 	}

// 	return s.subjectFaqrepo.GetByFilter(ctx, bson.M{"subjectId": sId, "_id": fId, "trash": false})
// }

// func (s *subjectsvcs) AddSubjectFAQ(ctx context.Context, subjectId string, data *models.SubjectFAQDto) (*models.SubjectFAQ, error) {
// 	sId, err := primitive.ObjectIDFromHex(subjectId)
// 	if err != nil {
// 		return nil, errors.New("invalid subject id")
// 	}

// 	cfg, err := util.GetReqAppCfg(ctx)
// 	if err != nil {
// 		return nil, err
// 	}

// 	faq := &models.SubjectFAQ{
// 		Id:            primitive.NewObjectID(),
// 		SubjectId:     sId,
// 		SubjectFAQDto: *data,
// 		CreatedBy:     cfg.User.Id,
// 		UpdatedBy:     cfg.User.Id,
// 		CreatedAt:     time.Now(),
// 		UpdatedAt:     time.Now(),
// 	}

// 	if err := s.subjectFaqrepo.Add(ctx, faq); err != nil {
// 		return nil, err
// 	}

// 	return faq, nil
// }

// func (s *subjectsvcs) UpdateSubjectFAQ(ctx context.Context, subjectId, faqId string, data *models.SubjectFAQDto) (*models.SubjectFAQ, error) {
// 	sId, err := primitive.ObjectIDFromHex(subjectId)
// 	if err != nil {
// 		return nil, errors.New("invalid subject id")
// 	}

// 	fId, err := primitive.ObjectIDFromHex(faqId)
// 	if err != nil {
// 		return nil, errors.New("invalid faq id")
// 	}

// 	cfg, err := util.GetReqAppCfg(ctx)
// 	if err != nil {
// 		return nil, err
// 	}

// 	faq := &models.SubjectFAQ{
// 		Id:            fId,
// 		SubjectId:     sId,
// 		SubjectFAQDto: *data,
// 		UpdatedBy:     cfg.User.Id,
// 		UpdatedAt:     time.Now(),
// 	}

// 	filter := bson.M{"subjectId": sId, "_id": fId, "trash": false}
// 	update := bson.M{"$set": faq}

// 	updatedFAQ, err := s.subjectFaqrepo.Patch(ctx, filter, update)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return updatedFAQ, nil
// }

// func (s *subjectsvcs) DeleteSubjectFAQ(ctx context.Context, subjectId, faqId string) error {
// 	sId, err := primitive.ObjectIDFromHex(subjectId)
// 	if err != nil {
// 		return errors.New("invalid subject id")
// 	}

// 	fId, err := primitive.ObjectIDFromHex(faqId)
// 	if err != nil {
// 		return errors.New("invalid faq id")
// 	}

// 	cfg, err := util.GetReqAppCfg(ctx)
// 	if err != nil {
// 		return err
// 	}

// 	filter := bson.M{"subjectId": sId, "_id": fId, "trash": false}
// 	update := bson.M{"$set": bson.M{"trash": true, "updatedBy": cfg.User.Id, "updatedAt": time.Now()}}

// 	_, err = s.subjectFaqrepo.Patch(ctx, filter, update)
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }
