package contact

import (
	"context"
	"errors"
	"fmt"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/query"

	// "larsa-academy-microservice/pkg/services/contact/constant"
	"larsa-academy-microservice/pkg/services/contact/enums"
	"larsa-academy-microservice/pkg/services/contact/models"
	content_enums "larsa-academy-microservice/pkg/services/content/enums"
	content_models "larsa-academy-microservice/pkg/services/content/models"
	points_models "larsa-academy-microservice/pkg/services/points/models"
	"larsa-academy-microservice/pkg/services/sorting"
	support_enums "larsa-academy-microservice/pkg/services/support/enums"
	support_models "larsa-academy-microservice/pkg/services/support/models"
	support_repo "larsa-academy-microservice/pkg/services/support/repo"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"math"
	"time"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type ParentSvcs interface {
	GetOne(ctx context.Context, id string) (*models.Parent, error)
	Get(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.ParentsPagination, error)
	Add(ctx context.Context, data *models.ParentDto) (*models.Parent, error)
	Update(ctx context.Context, id string, data *models.ParentDto) (*models.Parent, error)
	Delete(ctx context.Context, id string) error
	GetByStudentPhone(ctx context.Context, phone models.PhoneNumber) ([]models.Parent, error)
	GetSummary(ctx context.Context, id string) (*models.ParentSummary, error)
	GetChildren(ctx context.Context, id string, skip, limit int64, query *query.Conditions) (*models.ChildrenResponse, error)
	GetAll(ctx context.Context, query *query.Conditions) ([]models.ParentWithStats, error)
	IsUserParentOfStudent(ctx context.Context, userId, studentId primitive.ObjectID) (bool, error)
	GetChildrenIds(ctx context.Context, parentId primitive.ObjectID) ([]primitive.ObjectID, error)
	GetParentUserIdsForStudent(ctx context.Context, studentId primitive.ObjectID) ([]primitive.ObjectID, error)
	UpdateMe(ctx context.Context, id string, data *models.ParentProfileUpdateDto) (*models.Parent, error)
}

type parentSvcs struct {
	repo                      db.MainRepo[models.Parent]
	studentrepo               db.MainRepo[models.Student]
	parentStudentRelationRepo db.MainRepo[models.ParentStudentRelation]
	complaintRepo             support_repo.ComplaintRepo
	studentCourseRepo         db.MainRepo[content_models.StudentCourse]
	studentCoachingRepo       db.MainRepo[content_models.CoachingStudent]
	studentPointRepo          db.MainRepo[points_models.StudentPoint]
	studentSessionRepo        db.MainRepo[content_models.StudentSession]
	// studentSvcs               StudentSvcs
	withtxn  *db.WithTxn
	credSvcs CredSvcs
	// parentStudentRelationSvcs ParentStudentRelationSvcs

	sortingsvcs sorting.SortingSvcs
}

func NewParentSvcs(i *do.Injector) (ParentSvcs, error) {
	return &parentSvcs{
		repo:                      db.NewRepo[models.Parent](i),
		studentrepo:               db.NewRepo[models.Student](i),
		parentStudentRelationRepo: db.NewRepo[models.ParentStudentRelation](i),
		complaintRepo:             do.MustInvoke[support_repo.ComplaintRepo](i),
		studentCourseRepo:         db.NewRepo[content_models.StudentCourse](i),
		studentCoachingRepo:       db.NewRepo[content_models.CoachingStudent](i),
		studentPointRepo:          db.NewRepo[points_models.StudentPoint](i),
		studentSessionRepo:        db.NewRepo[content_models.StudentSession](i),
		// studentSvcs:               do.MustInvoke[StudentSvcs](i),
		withtxn:  do.MustInvoke[*db.WithTxn](i),
		credSvcs: do.MustInvoke[CredSvcs](i),
		// parentStudentRelationSvcs: do.MustInvoke[ParentStudentRelationSvcs](i),

		sortingsvcs: do.MustInvoke[sorting.SortingSvcs](i),
	}, nil
}

func (s *parentSvcs) GetOne(ctx context.Context, id string) (*models.Parent, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	parent, err := s.repo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, err
	}
	if parent == nil {
		return nil, errors.New("parent not found")
	}
	// relation, err := s.parentStudentRelationSvcs.GetByParent(ctx, id)
	// if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
	// 	return nil, err
	// }
	// parent.ParentStudentRelations = relation

	return parent, nil
}
func (s *parentSvcs) Get(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.ParentsPagination, error) {
	if err := query.CheckValid(); err != nil {
		return nil, errors.New("query not valid")
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, errors.New("error converting query to mongo")
	}

	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
		{"$match": filter},
	}

	count, err := s.repo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	pipeline = append(pipeline,
		bson.M{"$sort": bson.M{"_id": -1}},
		bson.M{"$skip": skip},
		bson.M{"$limit": limit},
	)

	// Add lookup to count children
	pipeline = append(pipeline, bson.M{
		"$lookup": bson.M{
			"from": "academyParentsStudentRelations",
			"let":  bson.M{"parentId": "$_id"},
			"pipeline": []bson.M{
				{"$match": bson.M{
					"$expr": bson.M{
						"$and": []bson.M{
							{"$eq": []interface{}{"$trash", false}},
							{"$or": []bson.M{
								{
									"$and": []bson.M{
										{"$eq": []interface{}{"$firstId", "$$parentId"}},
										{"$eq": []interface{}{"$firstType", enums.PersonTypeParent}},
										{"$eq": []interface{}{"$secondType", enums.PersonTypeStudent}},
									},
								},
								{
									"$and": []bson.M{
										{"$eq": []interface{}{"$secondId", "$$parentId"}},
										{"$eq": []interface{}{"$secondType", enums.PersonTypeParent}},
										{"$eq": []interface{}{"$firstType", enums.PersonTypeStudent}},
									},
								},
							}},
						},
					},
				}},
				{"$count": "count"},
			},
			"as": "childrenCount",
		},
	})

	// Add project to include numberOfChildren field
	pipeline = append(pipeline, bson.M{
		"$addFields": bson.M{
			"numberOfChildren": bson.M{
				"$ifNull": []interface{}{
					bson.M{"$arrayElemAt": []interface{}{"$childrenCount.count", 0}},
					0,
				},
			},
		},
	})

	// Remove the childrenCount array from output
	pipeline = append(pipeline, bson.M{
		"$project": bson.M{
			"childrenCount": 0,
		},
	})

	var result []models.ParentWithStats
	errAg := s.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if errAg != nil {
		return nil, errAg
	}

	var totalPages float64 = math.Ceil(float64(count) / float64(limit))
	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: count,
	}

	return &models.ParentsPagination{
		Parents:    result,
		Pagination: pagination,
	}, nil
}
func (s *parentSvcs) Add(ctx context.Context, data *models.ParentDto) (*models.Parent, error) {
	var deleteUserOnErr func() error

	result, err := s.withtxn.Exec(ctx, func(ctx mongo.SessionContext) (any, error) {
		cfg, err := util.GetReqAppCfg(ctx)
		if err != nil {
			return nil, err
		}

		parent := &models.Parent{
			ParentDto: *data,
			CreatedAt: time.Now(),
			CreatedBy: cfg.User.Id,
			UpdatedAt: time.Now(),
			UpdatedBy: cfg.User.Id,
		}

		result, err := s.credSvcs.Add(ctx, parent, "parent")
		if err != nil {
			return nil, err
		}
		deleteUserOnErr = result.DeleteOnErr

		parent.Id = result.UserData.Id

		seq, err := s.sortingsvcs.GetAndUpdateSourceSeq(ctx, "parents")
		if err != nil {
			return nil, err
		}

		parent.Seq = fmt.Sprintf("PRT-%d", seq)

		if err := s.repo.Add(ctx, parent); err != nil {
			return nil, err
		}

		return parent, nil
	})

	if err != nil {
		if deleteUserOnErr != nil {
			deleteUserOnErr()
		}
		return nil, err
	}

	return result.(*models.Parent), nil
}
func (s *parentSvcs) Update(ctx context.Context, id string, data *models.ParentDto) (*models.Parent, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s.withtxn.Exec(ctx, func(ctx mongo.SessionContext) (any, error) {
		oldParent, err := s.GetOne(ctx, id)
		if err != nil {
			return nil, err
		}
		parent := &models.Parent{
			ParentDto: *data,
			CreatedAt: oldParent.CreatedAt,
			CreatedBy: oldParent.CreatedBy,
			UpdatedAt: time.Now(),
			UpdatedBy: cfg.User.Id,
		}

		parent.Id = _id

		// Sync phone if provided in update
		if data.ParentContactInfo.MobilePhoneNumber.Content != "" && data.ParentContactInfo.MobilePhoneNumber.Prefix != "" {
			parent.ParentContactInfo.MobilePhoneNumber = data.ParentContactInfo.MobilePhoneNumber
		}

		filter := bson.M{"_id": _id}
		update := bson.M{"$set": parent}

		updatedParent, err := s.repo.Patch(ctx, filter, update)
		if err != nil {
			return nil, err
		}

		if err := s.credSvcs.Update(ctx, id, parent); err != nil {
			return nil, err
		}

		return updatedParent, nil
	})

	if err != nil {
		return nil, err
	}

	return result.(*models.Parent), nil
}

func (s *parentSvcs) UpdateMe(ctx context.Context, id string, data *models.ParentProfileUpdateDto) (*models.Parent, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s.withtxn.Exec(ctx, func(ctx mongo.SessionContext) (any, error) {
		oldParent, err := s.GetOne(ctx, id)
		if err != nil {
			return nil, err
		}

		// Update user service FIRST
		userUpdateData := map[string]any{
			"firstName": data.FirstName.GetContentByLang("ar"),
			"lastName":  data.LastName.GetContentByLang("ar"),
			"email":     oldParent.AccountParentSetting.Email, // Preserve email
		}

		if oldParent.ParentContactInfo.MobilePhoneNumber.Content != "" && oldParent.ParentContactInfo.MobilePhoneNumber.Prefix != "" {
			userUpdateData["phone"] = oldParent.ParentContactInfo.MobilePhoneNumber.Prefix + oldParent.ParentContactInfo.MobilePhoneNumber.Content
		}

		if data.Picture.Path != "" {
			userUpdateData["picture"] = data.Picture.Path
		}

		if err := s.credSvcs.UpdateRaw(ctx, id, userUpdateData); err != nil {
			return nil, err
		}

		// Update local parent
		parent := *oldParent
		parent.ParentGeneralInfo.FirstName = data.FirstName
		parent.ParentGeneralInfo.LastName = data.LastName
		parent.ParentGeneralInfo.Gender = data.Gender
		parent.ParentGeneralInfo.DateOfBirth = data.DateOfBirth
		parent.ParentGeneralInfo.Nationality = data.Nationality
		parent.ParentGeneralInfo.Picture = data.Picture
		parent.ParentAddressDetails = data.Address
		parent.UpdatedAt = time.Now()
		parent.UpdatedBy = cfg.User.Id

		filter := bson.M{"_id": _id}
		update := bson.M{"$set": parent}

		updatedParent, err := s.repo.Patch(ctx, filter, update)
		if err != nil {
			return nil, err
		}

		return updatedParent, nil
	})

	if err != nil {
		return nil, err
	}

	return result.(*models.Parent), nil
}
func (s *parentSvcs) Delete(ctx context.Context, id string) error {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	_, err = s.withtxn.Exec(ctx, func(ctx mongo.SessionContext) (any, error) {
		// Update the parent document to set trash=true and update timestamps
		filter := bson.M{"_id": _id, "trash": false}
		update := bson.M{
			"$set": bson.M{
				"trash":     true,
				"updatedAt": time.Now(),
				"updatedBy": cfg.User.Id,
			},
		}

		// Use the repo to update the document
		updatedParent, err := s.repo.Patch(ctx, filter, update)
		if err != nil {
			return nil, err
		}

		if updatedParent == nil {
			return nil, errors.New("parent not found or already deleted")
		}
		if err := s.credSvcs.Delete(ctx, id); err != nil {
			return nil, err
		}

		return updatedParent, nil
	})

	return err
}
func (s *parentSvcs) GetByStudentPhone(ctx context.Context, phone models.PhoneNumber) ([]models.Parent, error) {
	filter := bson.M{
		"parentContactInfo.mobilePhoneNumber.prefix":  phone.Prefix,
		"parentContactInfo.mobilePhoneNumber.content": phone.Content,
		"trash": false,
	}
	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
		{"$match": filter},
	}
	var result []models.Parent
	errAg := s.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if errAg != nil {
		return nil, errAg
	}
	return result, nil
}

func (s *parentSvcs) GetSummary(ctx context.Context, id string) (*models.ParentSummary, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	parent, err := s.GetOne(ctx, id)
	if err != nil {
		return nil, err
	}

	// Get complaints filed by this parent
	complaintsPipeline := []bson.M{
		{"$match": bson.M{
			"createdBy":         _id,
			"complaintUserRole": support_enums.RoleParent,
			"trash":             false,
		}},
		{"$sort": bson.M{"_id": -1}},
	}
	var complaints []support_models.Complaint
	err = s.complaintRepo.Aggregate(ctx, complaintsPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &complaints)
	})
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}

	// Convert complaints to interface{} slice
	complaintsInterface := make([]interface{}, len(complaints))
	for i := range complaints {
		complaintsInterface[i] = complaints[i]
	}

	// Get children (parent-student relations)
	childrenPipeline := []bson.M{
		{"$match": bson.M{
			"$or": []bson.M{
				{
					"firstId":    _id,
					"firstType":  enums.PersonTypeParent,
					"secondType": enums.PersonTypeStudent,
				},
				{
					"secondId":   _id,
					"secondType": enums.PersonTypeParent,
					"firstType":  enums.PersonTypeStudent,
				},
			},
			"trash": false,
		}},
		{"$sort": bson.M{"_id": -1}},
	}
	var children []models.ParentStudentRelation
	err = s.parentStudentRelationRepo.Aggregate(ctx, childrenPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &children)
	})
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}

	return &models.ParentSummary{
		Parent:     *parent,
		Complaints: complaintsInterface,
		Children:   children,
	}, nil
}

func (s *parentSvcs) GetChildren(ctx context.Context, id string, skip, limit int64, query *query.Conditions) (*models.ChildrenResponse, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	// Build base filter with parent ID $or condition
	baseFilter := bson.M{
		"$or": []bson.M{
			{
				"firstId":    _id,
				"firstType":  enums.PersonTypeParent,
				"secondType": enums.PersonTypeStudent,
			},
			{
				"secondId":   _id,
				"secondType": enums.PersonTypeParent,
				"firstType":  enums.PersonTypeStudent,
			},
		},
		"trash": false,
	}

	// Merge flexible filter if provided
	if query != nil && len(query.Columns) > 0 {
		if err := query.CheckValid(); err != nil {
			return nil, errors.New("query not valid")
		}
		queryFilter, err := query.ConvertToMongo()
		if err != nil {
			return nil, errors.New("error converting query to mongo")
		}
		// Merge query filter with base filter
		// If query filter has $or, we need to combine it with base filter's $or using $and
		if _, hasOr := queryFilter["$or"]; hasOr {
			// Combine both filters using $and
			baseFilter = bson.M{
				"$and": []bson.M{
					baseFilter,
					queryFilter,
				},
			}
		} else {
			// Simple merge for non-$or filters
			for k, v := range queryFilter {
				baseFilter[k] = v
			}
		}
	}

	// Build pipeline for counting
	countPipeline := []bson.M{
		{"$match": baseFilter},
	}
	count, err := s.parentStudentRelationRepo.Count(ctx, countPipeline)
	if err != nil {
		return nil, err
	}

	// Build pipeline for fetching relations
	pipeline := []bson.M{
		{"$match": baseFilter},
		{"$sort": bson.M{"_id": -1}},
		{"$skip": skip},
		{"$limit": limit},
	}

	var relations []models.ParentStudentRelation
	err = s.parentStudentRelationRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &relations)
	})
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}

	// Get parent data for enrichment
	parent, err := s.repo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil || parent == nil {
		return nil, errors.New("parent not found")
	}
	parentId := _id.Hex()

	// Enhance each relation with student stats and enriched parent/student data
	children := make([]models.ChildResponse, 0, len(relations))
	for _, relation := range relations {
		// Determine student ID and parent ID from relation
		var studentId, relationParentId primitive.ObjectID
		if relation.FirstType == enums.PersonTypeStudent {
			studentId = relation.FirstId
			relationParentId = relation.SecondId
		} else {
			studentId = relation.SecondId
			relationParentId = relation.FirstId
		}

		child := models.ChildResponse{
			Id:                        relation.Id.Hex(),
			EnrolledCoursesCount:      0,
			EnrolledCoachingsCount:    0,
			Grade:                     "",
			AttendancePercentage:      0.0,
			AcademicProgress:         0.0,
			CoachingCommitmentProgress: 0.0,
			TotalPoints:               0,
			LastActive:                time.Time{},
			Status:                    "",
			CreatedAt:                 relation.CreatedAt,
			UpdatedAt:                 relation.UpdatedAt,
		}

		// Get student data for enrichment
		student, err := s.studentrepo.GetByFilter(ctx, bson.M{"_id": studentId, "trash": false})
		if err == nil && student != nil {
			child.Student = models.StudentInfo{
				Id:        studentId.Hex(),
				FirstName: student.GeneralInfo.FirstName,
				LastName:  student.GeneralInfo.LastName,
			}
			child.Status = string(student.GeneralInfo.Status)
			child.Grade = student.AcademicInfo.Stage
		} else {
			child.Student = models.StudentInfo{
				Id: studentId.Hex(),
			}
		}

		// Get parent data from the relation for enrichment
		if relationParentId != _id {
			relParent, err := s.repo.GetByFilter(ctx, bson.M{"_id": relationParentId, "trash": false})
			if err == nil && relParent != nil {
				child.Parent = models.ParentInfo{
					Id:        relationParentId.Hex(),
					FirstName: relParent.ParentGeneralInfo.FirstName,
					LastName:  relParent.ParentGeneralInfo.LastName,
				}
			} else {
				child.Parent = models.ParentInfo{
					Id: relationParentId.Hex(),
				}
			}
		} else {
			child.Parent = models.ParentInfo{
				Id:        parentId,
				FirstName: parent.ParentGeneralInfo.FirstName,
				LastName:  parent.ParentGeneralInfo.LastName,
			}
		}

		child.RelationType = relation.RelationType

		// Get student courses for enrolled count, progress, and last activity
		studentCoursesPipeline := []bson.M{
			{"$match": bson.M{
				"studentId": studentId,
				"trash":     false,
			}},
		}
		var studentCourses []content_models.StudentCourse
		err = s.studentCourseRepo.Aggregate(ctx, studentCoursesPipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &studentCourses)
		})
		if err == nil {
			child.EnrolledCoursesCount = len(studentCourses)

			// Calculate average academic progress
			if len(studentCourses) > 0 {
				totalProgress := 0
				latestActivity := time.Time{}
				for _, sc := range studentCourses {
					totalProgress += sc.Progress
					if sc.LastActivity.After(latestActivity) {
						latestActivity = sc.LastActivity
					}
				}
				child.AcademicProgress = float64(totalProgress) / float64(len(studentCourses))
				child.LastActive = latestActivity
			}
		}

		// Get student points
		studentPoint, err := s.studentPointRepo.GetByFilter(ctx, bson.M{"studentId": studentId, "trash": false})
		if err == nil && studentPoint != nil {
			child.TotalPoints = studentPoint.TotalPoints
		}

		// Calculate attendance percentage from student sessions
		// Concluded sessions = completed + missed + cancelled
		// Attendance = (completed / total concluded) * 100
		sessionsPipeline := []bson.M{
			{"$match": bson.M{
				"studentId": studentId,
				"trash":     false,
				"status": bson.M{
					"$in": []content_enums.StudentSessionStatus{
						content_enums.StudentSessionStatusCompleted,
						content_enums.StudentSessionStatusMissed,
						content_enums.StudentSessionStatusCancelled,
					},
				},
			}},
			{"$group": bson.M{
				"_id":   "$status",
				"count": bson.M{"$sum": 1},
			}},
		}
		var sessionCounts []struct {
			Id    content_enums.StudentSessionStatus `bson:"_id"`
			Count int                                `bson:"count"`
		}
		err = s.studentSessionRepo.Aggregate(ctx, sessionsPipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &sessionCounts)
		})
		if err == nil {
			totalConcluded := 0
			completedCount := 0
			for _, sc := range sessionCounts {
				totalConcluded += sc.Count
				if sc.Id == content_enums.StudentSessionStatusCompleted {
					completedCount = sc.Count
				}
			}
			if totalConcluded > 0 {
				child.AttendancePercentage = (float64(completedCount) / float64(totalConcluded)) * 100.0
			}
		}

		// Get coaching enrollments count and average commitment progress for parent home (courses + coaching)
		coachingEnrollPipeline := []bson.M{
			{"$match": bson.M{
				"studentId": studentId,
				"trash":     false,
			}},
			{"$group": bson.M{
				"_id":     nil,
				"count":   bson.M{"$sum": 1},
				"avgComm": bson.M{"$avg": "$commitmentProgress"},
			}},
		}
		var coachingAgg []struct {
			Count   int     `bson:"count"`
			AvgComm float64 `bson:"avgComm"`
		}
		err = s.studentCoachingRepo.Aggregate(ctx, coachingEnrollPipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &coachingAgg)
		})
		if err == nil && len(coachingAgg) > 0 {
			child.EnrolledCoachingsCount = coachingAgg[0].Count
			child.CoachingCommitmentProgress = coachingAgg[0].AvgComm
		}

		children = append(children, child)
	}

	var totalPages float64 = math.Ceil(float64(count) / float64(limit))
	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: count,
	}

	return &models.ChildrenResponse{
		Children:   children,
		Pagination: pagination,
	}, nil
}

func (s *parentSvcs) GetAll(ctx context.Context, query *query.Conditions) ([]models.ParentWithStats, error) {
	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
	}

	// Apply query filter if provided
	if query != nil && len(query.Columns) > 0 {
		if err := query.CheckValid(); err != nil {
			return nil, errors.New("query not valid")
		}
		filter, err := query.ConvertToMongo()
		if err != nil {
			return nil, errors.New("error converting query to mongo")
		}
		pipeline = append(pipeline, bson.M{"$match": filter})
	}

	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})

	// Add lookup to count children
	pipeline = append(pipeline, bson.M{
		"$lookup": bson.M{
			"from": "academyParentsStudentRelations",
			"let":  bson.M{"parentId": "$_id"},
			"pipeline": []bson.M{
				{"$match": bson.M{
					"$expr": bson.M{
						"$and": []bson.M{
							{"$eq": []interface{}{"$trash", false}},
							{"$or": []bson.M{
								{
									"$and": []bson.M{
										{"$eq": []interface{}{"$firstId", "$$parentId"}},
										{"$eq": []interface{}{"$firstType", enums.PersonTypeParent}},
										{"$eq": []interface{}{"$secondType", enums.PersonTypeStudent}},
									},
								},
								{
									"$and": []bson.M{
										{"$eq": []interface{}{"$secondId", "$$parentId"}},
										{"$eq": []interface{}{"$secondType", enums.PersonTypeParent}},
										{"$eq": []interface{}{"$firstType", enums.PersonTypeStudent}},
									},
								},
							}},
						},
					},
				}},
				{"$count": "count"},
			},
			"as": "childrenCount",
		},
	})

	// Add project to include numberOfChildren field
	pipeline = append(pipeline, bson.M{
		"$addFields": bson.M{
			"numberOfChildren": bson.M{
				"$ifNull": []interface{}{
					bson.M{"$arrayElemAt": []interface{}{"$childrenCount.count", 0}},
					0,
				},
			},
		},
	})

	// Remove the childrenCount array from output
	pipeline = append(pipeline, bson.M{
		"$project": bson.M{
			"childrenCount": 0,
		},
	})

	var result []models.ParentWithStats
	err := s.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// IsUserParentOfStudent checks if the given userId is a parent of the given studentId
func (s *parentSvcs) IsUserParentOfStudent(ctx context.Context, userId, studentId primitive.ObjectID) (bool, error) {
	filter := bson.M{
		"$or": []bson.M{
			{
				"firstId":    userId,
				"firstType":  enums.PersonTypeParent,
				"secondId":   studentId,
				"secondType": enums.PersonTypeStudent,
			},
			{
				"firstId":    studentId,
				"firstType":  enums.PersonTypeStudent,
				"secondId":   userId,
				"secondType": enums.PersonTypeParent,
			},
		},
		"trash": false,
	}

	count, err := s.parentStudentRelationRepo.Count(ctx, []bson.M{{"$match": filter}})
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// GetChildrenIds returns all student IDs associated with the given parent ID
func (s *parentSvcs) GetChildrenIds(ctx context.Context, parentId primitive.ObjectID) ([]primitive.ObjectID, error) {
	filter := bson.M{
		"$or": []bson.M{
			{
				"firstId":    parentId,
				"firstType":  enums.PersonTypeParent,
				"secondType": enums.PersonTypeStudent,
			},
			{
				"secondId":   parentId,
				"secondType": enums.PersonTypeParent,
				"firstType":  enums.PersonTypeStudent,
			},
		},
		"trash": false,
	}

	pipeline := []bson.M{
		{"$match": filter},
	}

	var relations []models.ParentStudentRelation
	err := s.parentStudentRelationRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &relations)
	})
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}

	childrenIds := []primitive.ObjectID{}
	for _, rel := range relations {
		if rel.FirstType == enums.PersonTypeStudent {
			childrenIds = append(childrenIds, rel.FirstId)
		} else if rel.SecondType == enums.PersonTypeStudent {
			childrenIds = append(childrenIds, rel.SecondId)
		}
	}

	return childrenIds, nil
}

// GetParentUserIdsForStudent returns all parent user IDs associated with the given student ID.
func (s *parentSvcs) GetParentUserIdsForStudent(ctx context.Context, studentId primitive.ObjectID) ([]primitive.ObjectID, error) {
	filter := bson.M{
		"$or": []bson.M{
			{
				"firstId":    studentId,
				"firstType":  enums.PersonTypeStudent,
				"secondType": enums.PersonTypeParent,
			},
			{
				"secondId":   studentId,
				"secondType": enums.PersonTypeStudent,
				"firstType":  enums.PersonTypeParent,
			},
		},
		"trash": false,
	}

	pipeline := []bson.M{
		{"$match": filter},
	}

	var relations []models.ParentStudentRelation
	err := s.parentStudentRelationRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &relations)
	})
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}

	parentIds := make([]primitive.ObjectID, 0, len(relations))
	seen := make(map[primitive.ObjectID]bool)
	for _, rel := range relations {
		var parentId primitive.ObjectID
		if rel.FirstType == enums.PersonTypeParent {
			parentId = rel.FirstId
		} else if rel.SecondType == enums.PersonTypeParent {
			parentId = rel.SecondId
		}
		if !parentId.IsZero() && !seen[parentId] {
			seen[parentId] = true
			parentIds = append(parentIds, parentId)
		}
	}

	return parentIds, nil
}
