package content

import (
	"context"
	"errors"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/query"
	contact "larsa-academy-microservice/pkg/services/contact"
	"larsa-academy-microservice/pkg/services/content/constant"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/services/messaging"
	"larsa-academy-microservice/pkg/services/zoom"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"math"
	"time"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type PlacementTestSvcs interface {
	GetById(ctx context.Context, id string) (*models.PlacementTest, error)
	GetByPathwayId(ctx context.Context, pathId string) (*models.PlacementTest, error)
	Get(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.PlacementTestPagination, error)
	Add(ctx context.Context, data *models.PlacementTestDto) (*models.PlacementTest, error)
	Update(ctx context.Context, id string, data *models.PlacementTestDto) (*models.PlacementTest, error)
	Delete(ctx context.Context, id string) error

	//student
	SubmitPlacementTestAnswers(ctx context.Context, data *models.StudentPlacementTestAnswers) (*models.StudentPlacementTest, error)
	OnlinePlacementTestSchedule(ctx context.Context, data *models.OnlinePlacementTestScheduleDto) (*models.OnlinePlacementTestSchedule, error)
	GetStudentPlacementTests(ctx context.Context, placementTestId string) (*models.StudentPlacementTestList, error)
	//teacher
	SubmitTeacherEvaluation(ctx context.Context, StudentPlacementTestId string, data *models.TeacherEvaluation) (any, error)
	//calendar event management
	UpdatePlacementTestSchedule(ctx context.Context, scheduleId string, data *models.UpdatePlacementTestScheduleDto) (*models.OnlinePlacementTestSchedule, error)
	DeletePlacementTestSchedule(ctx context.Context, scheduleId string) error
	GetPlacementTestSchedules(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.OnlinePlacementTestSchedulePagination, error)
	GetAllStudentPlacementTests(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.StudentPlacementTestPagination, error)
	GetStudentPlacementTestById(ctx context.Context, id string) (*models.StudentPlacementTest, error)
	DeleteStudentPlacementTest(ctx context.Context, id string) error
}

type placementtestsvcs struct {
	coachingSvcs                    CoachingSvcs
	courseSvcs                      CourseSvcs
	repo                            db.MainRepo[models.PlacementTest]
	studentPlacementTestRepo        db.MainRepo[models.StudentPlacementTest]
	onlinePlacementTestScheduleRepo db.MainRepo[models.OnlinePlacementTestSchedule]
	supervisorSvc                   contact.SupervisorSvc
	teacherAssignmentSvc            TeacherAssignmentSvcs
	zoomSvc                         zoom.ZoomService
	messageSvcs                     messaging.MessageSvcs
}

func NewPlacementTestSvcs(i *do.Injector) (PlacementTestSvcs, error) {
	return &placementtestsvcs{
		coachingSvcs:                    do.MustInvoke[CoachingSvcs](i),
		courseSvcs:                      do.MustInvoke[CourseSvcs](i),
		repo:                            db.NewRepo[models.PlacementTest](i),
		studentPlacementTestRepo:        db.NewRepo[models.StudentPlacementTest](i),
		onlinePlacementTestScheduleRepo: db.NewRepo[models.OnlinePlacementTestSchedule](i),
		supervisorSvc:                   do.MustInvoke[contact.SupervisorSvc](i),
		teacherAssignmentSvc:            do.MustInvoke[TeacherAssignmentSvcs](i),
		zoomSvc:                         do.MustInvoke[zoom.ZoomService](i),
		messageSvcs:                     do.MustInvoke[messaging.MessageSvcs](i),
	}, nil
}

// enrichPlacementTest populates the enriched fields from the referenced course or coaching
func (p *placementtestsvcs) enrichPlacementTest(ctx context.Context, placementTest *models.PlacementTest) error {
	if placementTest.RefId.IsZero() {
		return nil
	}

	switch placementTest.RefType {
	case "course":
		course, err := p.courseSvcs.GetById(ctx, placementTest.RefId.Hex())
		if err != nil {
			return err
		}
		placementTest.Country = course.BasicInfo.Country
		placementTest.ProgramType = course.BasicInfo.Type // Use Type as ProgramType
		placementTest.SchoolType = course.BasicInfo.SchoolType
		placementTest.SchoolId = course.BasicInfo.SchoolId
		// placementTest.SchoolObj = course.SchoolObj
		placementTest.Stage = course.BasicInfo.GradeLevel      // Map GradeLevel to Stage
		placementTest.Class = []string{course.BasicInfo.Level} // Map Level to Class

	case "coaching":
		coaching, err := p.coachingSvcs.GetById(ctx, placementTest.RefId.Hex(), false)
		if err != nil {
			return err
		}
		placementTest.Country = coaching.Country
		placementTest.ProgramType = coaching.ProgramType
		placementTest.SchoolType = coaching.SchoolType
		placementTest.SchoolId = coaching.SchoolId
		placementTest.SchoolObj = coaching.SchoolObj
		placementTest.Stage = coaching.Level // Map Level to Stage
		placementTest.Class = coaching.Class
	}
	return nil
}

func (p *placementtestsvcs) GetById(ctx context.Context, id string) (*models.PlacementTest, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid placement test id")
	}

	placementTest, err := p.repo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, err
	}

	// Enrich with data from ref
	if err := p.enrichPlacementTest(ctx, placementTest); err != nil {
		// Log error but don't fail the request
		// You might want to handle this differently based on your requirements
	}

	return placementTest, nil
}

func (p *placementtestsvcs) GetByPathwayId(ctx context.Context, pathId string) (*models.PlacementTest, error) {
	_id, err := primitive.ObjectIDFromHex(pathId)
	if err != nil {
		return nil, errors.New("invalid pathway id")
	}

	//todo: check status - active
	placementTest, err := p.repo.GetByFilter(ctx, bson.M{"pathway": _id, "trash": false})
	if err != nil {
		return nil, err
	}

	// Enrich with data from ref
	if err := p.enrichPlacementTest(ctx, placementTest); err != nil {
		// Log error but don't fail the request
	}

	return placementTest, nil
}

func (p *placementtestsvcs) Get(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.PlacementTestPagination, error) {
	if err := query.CheckValid(); err != nil {
		return nil, errors.New("invalid query")
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, err
	}

	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
		{"$match": filter},
	}

	count, err := p.repo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})
	pipeline = append(pipeline, bson.M{"$skip": skip})
	pipeline = append(pipeline, bson.M{"$limit": limit})

	var result []models.PlacementTest
	if err := p.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	// Enrich each placement test
	for i := range result {
		if err := p.enrichPlacementTest(ctx, &result[i]); err != nil {
			// Log error but continue
		}
	}

	return &models.PlacementTestPagination{
		PlacementTests: result,
		Pagination: types.Pagination{
			TotalPages: math.Ceil(float64(count) / float64(limit)),
			PerPage:    limit,
			TotalCount: count,
		},
	}, nil
}

func (p *placementtestsvcs) Add(ctx context.Context, data *models.PlacementTestDto) (*models.PlacementTest, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	placementTest := &models.PlacementTest{
		Id:               primitive.NewObjectID(),
		PlacementTestDto: *data,
		CreatedAt:        time.Now(),
		CreatedBy:        cfg.User.Id,
		UpdatedAt:        time.Now(),
		UpdatedBy:        cfg.User.Id,
	}

	if err := p.repo.Add(ctx, placementTest); err != nil {
		return nil, err
	}

	// Enrich with data from ref before returning
	if err := p.enrichPlacementTest(ctx, placementTest); err != nil {
		// Log error but don't fail the request
	}

	return placementTest, nil
}

func (p *placementtestsvcs) Update(ctx context.Context, id string, data *models.PlacementTestDto) (*models.PlacementTest, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid placement test id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	placementTest := &models.PlacementTest{
		Id:               _id,
		PlacementTestDto: *data,
		UpdatedAt:        time.Now(),
		UpdatedBy:        cfg.User.Id,
	}

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": placementTest}

	updatedPlacementTest, err := p.repo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	// Enrich with data from ref before returning
	if err := p.enrichPlacementTest(ctx, updatedPlacementTest); err != nil {
		// Log error but don't fail the request
	}

	return updatedPlacementTest, nil

}

func (p *placementtestsvcs) Delete(ctx context.Context, id string) error {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid placement test id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": bson.M{"trash": true, "updatedAt": time.Now(), "updatedBy": cfg.User.Id}}

	_, err = p.repo.Patch(ctx, filter, update)
	if err != nil {
		return err
	}

	return nil
}

// GetStudentPlacementTests retrieves all student placement test submissions/schedules for a given placement test.
// Students can only access their own placement tests. If the student has no submissions,
// the placement test is still returned with an empty submissions list.
func (p *placementtestsvcs) GetStudentPlacementTests(ctx context.Context, placementTestId string) (*models.StudentPlacementTestList, error) {
	placementTestObjectId, err := primitive.ObjectIDFromHex(placementTestId)
	if err != nil {
		return nil, constant.ErrInvalidPlacementTestId
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	placementTest, err := p.repo.GetByFilter(ctx, bson.M{"_id": placementTestObjectId, "trash": false})
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, constant.ErrPlacementTestNotFound
		}
		return nil, err
	}

	// Enrich placement test with data from ref
	if err := p.enrichPlacementTest(ctx, placementTest); err != nil {
		// Log error but continue
	}

	// Get all student placement tests for this student and placement test
	filter := bson.M{
		"placementTestId": placementTestObjectId,
		"studentId":       cfg.User.Id,
		"trash":           false,
	}

	var studentPlacementTests []models.StudentPlacementTest
	pipeline := []bson.M{
		{"$match": filter},
		{"$sort": bson.M{"_id": -1}},             // Most recent first
		{"$project": bson.M{"placementTest": 0}}, // drop duplicated embedded placement test to reduce payload
	}

	err = p.studentPlacementTestRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &studentPlacementTests)
	})
	if err != nil {
		return nil, err
	}

	return &models.StudentPlacementTestList{
		PlacementTest: *placementTest,
		Submissions:   studentPlacementTests,
	}, nil
}
