package contact

import (
	"context"
	"errors"
	"math"
	"time"

	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/contact/constant"
	"larsa-academy-microservice/pkg/services/contact/models"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// StudentNoteSvcs interface
type StudentNoteSvcs interface {
	Create(ctx context.Context, studentId primitive.ObjectID, data *models.StudentNoteCreateDto, createdBy primitive.ObjectID) (*models.StudentNote, error)
	GetById(ctx context.Context, id primitive.ObjectID) (*models.StudentNote, error)
	GetByStudentId(ctx context.Context, studentId primitive.ObjectID, skip, limit int64, conditions *query.Conditions) (*models.StudentNotePagination, error)
	Update(ctx context.Context, id primitive.ObjectID, data *models.StudentNoteUpdateDto, updatedBy primitive.ObjectID) (*models.StudentNote, error)
	UpdatePosition(ctx context.Context, studentId, noteId primitive.ObjectID, newPosition int, updatedBy primitive.ObjectID) error
	Delete(ctx context.Context, id primitive.ObjectID, deletedBy primitive.ObjectID) error
}

// studentNoteSvcs implementation
type studentNoteSvcs struct {
	repo db.MainRepo[models.StudentNote]
}

func NewStudentNoteSvcs(i *do.Injector) (StudentNoteSvcs, error) {
	return &studentNoteSvcs{
		repo: db.NewRepo[models.StudentNote](i),
	}, nil
}

func (s *studentNoteSvcs) Create(ctx context.Context, studentId primitive.ObjectID, data *models.StudentNoteCreateDto, createdBy primitive.ObjectID) (*models.StudentNote, error) {
	if err := s.checkAccess(ctx, studentId); err != nil {
		return nil, err
	}

	// Get current max sort value for this student
	maxSort, err := s.getMaxSort(ctx, studentId)
	if err != nil {
		return nil, err
	}

	note := &models.StudentNote{
		Id:        primitive.NewObjectID(),
		StudentId: studentId,
		Title:     data.Title,
		Tags:      data.Tags,
		Content:   data.Content,
		Sort:      maxSort + 1, // Place at the end
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		CreatedBy: createdBy,
		UpdatedBy: createdBy,
		Trash:     false,
	}

	if err := s.repo.Add(ctx, note); err != nil {
		return nil, err
	}

	return note, nil
}

func (s *studentNoteSvcs) GetById(ctx context.Context, id primitive.ObjectID) (*models.StudentNote, error) {
	note, err := s.repo.GetByFilter(ctx, bson.M{"_id": id, "trash": false})
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, constant.ERR_NOTE_NOT_FOUND
		}
		return nil, err
	}
	if err := s.checkAccess(ctx, note.StudentId); err != nil {
		return nil, err
	}
	return note, nil
}

func (s *studentNoteSvcs) GetByStudentId(ctx context.Context, studentId primitive.ObjectID, skip, limit int64, conditions *query.Conditions) (*models.StudentNotePagination, error) {
	if err := s.checkAccess(ctx, studentId); err != nil {
		return nil, err
	}

	filter := bson.M{
		"studentId": studentId,
		"trash":     false,
	}

	// Add additional conditions if provided
	if conditions != nil {
		if err := conditions.CheckValid(); err != nil {
			return nil, err
		}

		conditionFilter, err := conditions.ConvertToMongo()
		if err != nil {
			return nil, err
		}

		// Merge filters
		for key, value := range conditionFilter {
			filter[key] = value
		}
	}

	// Get total count
	totalCount, err := s.repo.Count(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Build aggregation pipeline for pagination
	pipeline := []bson.M{
		{"$match": filter},
		{"$sort": bson.M{"sort": 1}},
		{"$skip": skip},
		{"$limit": limit},
	}

	var notes []models.StudentNote
	err = s.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &notes)
	})
	if err != nil {
		return nil, err
	}

	// Calculate pagination
	totalPages := math.Ceil(float64(totalCount) / float64(limit))
	pagination := &models.StudentNotePagination{
		Notes: notes,
		Pagination: types.Pagination{
			TotalPages: totalPages,
			PerPage:    limit,
			TotalCount: totalCount,
		},
	}

	return pagination, nil
}

func (s *studentNoteSvcs) Update(ctx context.Context, id primitive.ObjectID, data *models.StudentNoteUpdateDto, updatedBy primitive.ObjectID) (*models.StudentNote, error) {
	note, err := s.repo.GetByFilter(ctx, bson.M{"_id": id, "trash": false})
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, constant.ERR_NOTE_NOT_FOUND
		}
		return nil, err
	}
	if err := s.checkAccess(ctx, note.StudentId); err != nil {
		return nil, err
	}

	filter := bson.M{"_id": id, "trash": false}
	setFields := bson.M{
		"updatedAt": time.Now(),
		"updatedBy": updatedBy,
	}

	if data.Title != nil {
		setFields["title"] = *data.Title
	}
	if data.Tags != nil {
		setFields["tags"] = data.Tags
	}
	if data.Content != nil {
		setFields["content"] = *data.Content
	}

	update := bson.M{"$set": setFields}
	return s.repo.Patch(ctx, filter, update)
}

func (s *studentNoteSvcs) UpdatePosition(ctx context.Context, studentId, noteId primitive.ObjectID, newPosition int, updatedBy primitive.ObjectID) error {
	if err := s.checkAccess(ctx, studentId); err != nil {
		return err
	}

	// Get all active notes for this student
	notes, err := s.getActiveByStudentId(ctx, studentId)
	if err != nil {
		return err
	}

	// Find the target note
	var targetNote *models.StudentNote
	targetIndex := -1
	for i, note := range notes {
		if note.Id == noteId {
			targetNote = note
			targetIndex = i
			break
		}
	}

	if targetNote == nil {
		return constant.ERR_NOTE_NOT_FOUND
	}

	// Set updatedBy for the target note
	targetNote.UpdatedBy = updatedBy

	// Remove target note from current position
	notes = append(notes[:targetIndex], notes[targetIndex+1:]...)

	// Ensure newPosition is within bounds
	if newPosition < 0 {
		newPosition = 0
	}
	if newPosition > len(notes) {
		newPosition = len(notes)
	}

	// Insert at new position
	notes = append(notes[:newPosition], append([]*models.StudentNote{targetNote}, notes[newPosition:]...)...)

	// Update sort values for all notes
	return s.updateSortValues(ctx, notes)
}

func (s *studentNoteSvcs) Delete(ctx context.Context, id primitive.ObjectID, deletedBy primitive.ObjectID) error {
	note, err := s.repo.GetByFilter(ctx, bson.M{"_id": id, "trash": false})
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return constant.ERR_NOTE_NOT_FOUND
		}
		return err
	}
	if err := s.checkAccess(ctx, note.StudentId); err != nil {
		return err
	}

	filter := bson.M{"_id": id, "trash": false}
	update := bson.M{
		"$set": bson.M{
			"trash":     true,
			"updatedAt": time.Now(),
			"updatedBy": deletedBy,
		},
	}

	_, err = s.repo.Patch(ctx, filter, update)
	return err
}

// Helper methods
func (s *studentNoteSvcs) getActiveByStudentId(ctx context.Context, studentId primitive.ObjectID) ([]*models.StudentNote, error) {
	pipeline := []bson.M{
		{"$match": bson.M{"studentId": studentId, "trash": false}},
		{"$sort": bson.M{"sort": 1}},
	}

	var notes []*models.StudentNote
	err := s.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &notes)
	})
	return notes, err
}

func (s *studentNoteSvcs) getMaxSort(ctx context.Context, studentId primitive.ObjectID) (int, error) {
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"studentId": studentId,
				"trash":     false,
			},
		},
		{
			"$group": bson.M{
				"_id":     nil,
				"maxSort": bson.M{"$max": "$sort"},
			},
		},
	}

	var result struct {
		MaxSort int `bson:"maxSort"`
	}

	err := s.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		if cur.Next(ctx) {
			return cur.Decode(&result)
		}
		return nil
	})

	if err != nil {
		return 0, err
	}

	return result.MaxSort, nil
}

func (s *studentNoteSvcs) updateSortValues(ctx context.Context, notes []*models.StudentNote) error {
	if len(notes) == 0 {
		return nil
	}

	var operations []mongo.WriteModel
	now := time.Now()

	for i, note := range notes {
		filter := bson.M{"_id": note.Id}
		update := bson.M{
			"$set": bson.M{
				"sort":      i,
				"updatedAt": now,
				"updatedBy": note.UpdatedBy,
			},
		}

		operation := mongo.NewUpdateOneModel().
			SetFilter(filter).
			SetUpdate(update)

		operations = append(operations, operation)
	}

	_, err := s.repo.BulkWrite(ctx, operations)
	return err
}

func (s *studentNoteSvcs) checkAccess(ctx context.Context, studentId primitive.ObjectID) error {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	// 1. Is the student themselves?
	if cfg.User != nil && cfg.User.Id == studentId {
		return nil
	}

	// 2. Has ViewStudentNotes capability?
	allowed, err := util.HasCapability(ctx, CapabilityViewStudentNotes)
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}

	return constant.ERR_ACCESS_DENIED
}
