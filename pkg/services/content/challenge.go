package content

import (
	"context"
	"errors"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/gateway"
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

type ChallengeSvcs interface {
	GetById(ctx context.Context, id string) (*models.Challenge, error)
	Get(ctx context.Context, skip, limit int64, query *query.Conditions, requesterStudentId *primitive.ObjectID) (*models.ChallengePagination, error)
	Add(ctx context.Context, data *models.ChallengeDto) (*models.Challenge, error)
	Update(ctx context.Context, challengeId string, data *models.ChallengeDto) (*models.Challenge, error)
	Delete(ctx context.Context, challengeId string) error
}

type challengesvcs struct {
	challengeRepo    db.MainRepo[models.Challenge]
	courseRepo       db.MainRepo[models.Course]
	studentCourseRepo db.MainRepo[models.StudentCourse]
	submissionSvcs   ChallengeSubmissionSvcs
	usersGw          *gateway.UsersGw
}

func NewChallengeSvcs(i *do.Injector) (ChallengeSvcs, error) {
	return &challengesvcs{
		challengeRepo:    db.NewRepo[models.Challenge](i),
		courseRepo:       db.NewRepo[models.Course](i),
		studentCourseRepo: db.NewRepo[models.StudentCourse](i),
		submissionSvcs:   do.MustInvoke[ChallengeSubmissionSvcs](i),
		usersGw:          do.MustInvoke[*gateway.UsersGw](i),
	}, nil
}

func (c *challengesvcs) GetById(ctx context.Context, id string) (*models.Challenge, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid challenge id")
	}

	challenge, err := c.challengeRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, err
	}

	// Populate course object
	if !challenge.Course.IsZero() {
		course, err := c.courseRepo.GetByFilter(ctx, bson.M{"_id": challenge.Course, "trash": false})
		if err == nil {
			challenge.CourseObj = course
		}
	}

	// Populate total points from settings
	challenge.TotalPoints = challenge.Settings.NumOfRewardPoints

	// Populate statistics
	stats, err := c.submissionSvcs.GetChallengeSubmissionsWithStats(ctx, challenge.Id.Hex(), 0, 100)
	if err == nil && stats != nil {
		challenge.TotalSubmissions = stats.TotalSubmissions
		challenge.UniqueStudents = stats.UniqueStudents
		challenge.AverageScore = stats.AverageScore
	}

	// Enrich CreatedBy with name using users gateway (picture may not be available via gateway)
	if !challenge.CreatedBy.IsZero() && c.usersGw != nil {
		if u, err := c.usersGw.GetUserById(ctx, challenge.CreatedBy.Hex()); err == nil && u != nil {
			name := ""
			if u.FirstName != "" || u.LastName != "" {
				name = u.FirstName + " " + u.LastName
			}
			challenge.CreatedByName = name
			challenge.CreatedByPicture = u.Picture
		}
	}

	return challenge, nil
}

func (c *challengesvcs) Get(ctx context.Context, skip, limit int64, query *query.Conditions, requesterStudentId *primitive.ObjectID) (*models.ChallengePagination, error) {
	if err := query.CheckValid(); err != nil {
		return nil, err
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, err
	}

	// Build aggregation pipeline with lookups and statistics
	pipeline := []bson.M{
		// Match non-deleted challenges first
		{"$match": bson.M{"trash": false}},

		// Lookup course information
		{"$lookup": bson.M{
			"from":         "academyCourses",
			"localField":   "course",
			"foreignField": "_id",
			"as":           "courseObj",
		}},
		{"$unwind": bson.M{
			"path":                       "$courseObj",
			"preserveNullAndEmptyArrays": true,
		}},

		// Lookup submissions to calculate statistics
		{"$lookup": bson.M{
			"from":         "academyChallengeSubmissions",
			"localField":   "_id",
			"foreignField": "challengeId",
			"as":           "submissions",
		}},

		// Add calculated fields for statistics
		{"$addFields": bson.M{
			"totalPoints": "$settings.rewardPoints",
			"totalSubmissions": bson.M{
				"$size": bson.M{
					"$filter": bson.M{
						"input": "$submissions",
						"cond":  bson.M{"$eq": []interface{}{"$$this.trash", false}},
					},
				},
			},
			"uniqueStudents": bson.M{
				"$size": bson.M{
					"$setUnion": bson.M{
						"$map": bson.M{
							"input": bson.M{
								"$filter": bson.M{
									"input": "$submissions",
									"cond":  bson.M{"$eq": []interface{}{"$$this.trash", false}},
								},
							},
							"as": "sub",
							"in": "$$sub.studentId",
						},
					},
				},
			},
			"averageScore": bson.M{
				"$cond": bson.M{
					"if": bson.M{"$gt": []interface{}{bson.M{"$size": "$submissions"}, 0}},
					"then": bson.M{
						"$avg": bson.M{
							"$map": bson.M{
								"input": bson.M{
									"$filter": bson.M{
										"input": "$submissions",
										"cond":  bson.M{"$eq": []interface{}{"$$this.trash", false}},
									},
								},
								"as": "sub",
								"in": "$$sub.avgScore",
							},
						},
					},
					"else": 0,
				},
			},
		}},

		// Lookup creator info from users collection
		{"$lookup": bson.M{
			"from":         "users",
			"localField":   "createdBy",
			"foreignField": "_id",
			"as":           "creator",
		}},
		{"$unwind": bson.M{"path": "$creator", "preserveNullAndEmptyArrays": true}},
		{"$set": bson.M{"createdByName": "$creator.firstName", "createdByPicture": "$creator.picture"}},

		// Remove submissions array (no longer needed)
		{"$project": bson.M{
			"submissions": 0,
		}},
	}

	// When requester is a student, restrict visibility based on scope and enrolled courses
	if requesterStudentId != nil && !requesterStudentId.IsZero() {
		var enrollments []models.StudentCourse
		if err := c.studentCourseRepo.Aggregate(ctx, []bson.M{
			{"$match": bson.M{
				"studentId": *requesterStudentId,
				"trash":     false,
			}},
		}, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &enrollments)
		}); err == nil {
			enrolledCourseIds := make([]primitive.ObjectID, 0, len(enrollments))
			for _, e := range enrollments {
				if !e.CourseId.IsZero() {
					enrolledCourseIds = append(enrolledCourseIds, e.CourseId)
				}
			}

			scopeFilter := bson.M{
				"$or": []bson.M{
					{"scope": bson.M{"$exists": false}},
					{"scope": "global"},
				},
			}

			if len(enrolledCourseIds) > 0 {
				scopeFilter["$or"] = append(scopeFilter["$or"].([]bson.M), bson.M{
					"$and": []bson.M{
						{"scope": "course"},
						{"course": bson.M{"$in": enrolledCourseIds}},
					},
				})
			}

			pipeline = append(pipeline, bson.M{"$match": scopeFilter})
		}
	}

	// Add submissionObj lookup when requester is a student (for listing their submissions per challenge)
	if requesterStudentId != nil && !requesterStudentId.IsZero() {
		pipeline = append(pipeline, bson.M{
			"$lookup": bson.M{
				"from": "academyChallengeSubmissions",
				"let":  bson.M{"challengeId": "$_id", "studentId": *requesterStudentId},
				"pipeline": []bson.M{
					{
						"$match": bson.M{
							"$expr": bson.M{
								"$and": []bson.M{
									{"$eq": []interface{}{"$challengeId", "$$challengeId"}},
									{"$eq": []interface{}{"$studentId", "$$studentId"}},
									{"$eq": []interface{}{"$trash", false}},
								},
							},
						},
					},
					{"$sort": bson.M{"createdAt": -1}},
				},
				"as": "submissionObj",
			},
		})
	}

	pipeline = append(pipeline,
		bson.M{"$match": filter},
		bson.M{
			"$facet": bson.M{
				"metadata": []bson.M{
					{"$count": "total"},
				},
				"data": []bson.M{
					{"$sort": bson.M{"_id": -1}},
					{"$skip": skip},
					{"$limit": limit},
				},
			},
		})

	type facetResult struct {
		Metadata []struct {
			Total int64 `bson:"total"`
		} `bson:"metadata"`
		Data []models.Challenge `bson:"data"`
	}

	var results []facetResult
	if err := c.challengeRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &results)
	}); err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return &models.ChallengePagination{
			Challenges: []models.Challenge{},
			Pagination: types.Pagination{
				TotalPages: 0,
				PerPage:    limit,
				TotalCount: 0,
			},
		}, nil
	}

	result := results[0]
	count := int64(0)
	if len(result.Metadata) > 0 {
		count = result.Metadata[0].Total
	}

	challenges := result.Data

	totalPages := math.Ceil(float64(count) / float64(limit))
	return &models.ChallengePagination{
		Challenges: challenges,
		Pagination: types.Pagination{
			TotalPages: totalPages,
			PerPage:    limit,
			TotalCount: count,
		},
	}, nil
}

func (c *challengesvcs) Add(ctx context.Context, data *models.ChallengeDto) (*models.Challenge, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	challenge := &models.Challenge{
		Id:           primitive.NewObjectID(),
		ChallengeDto: *data,
		CreatedAt:    time.Now(),
		CreatedBy:    cfg.User.Id,
		UpdatedAt:    time.Now(),
		UpdatedBy:    cfg.User.Id,
	}
	ensureQuestionIds(&challenge.Quiz)

	if err := c.challengeRepo.Add(ctx, challenge); err != nil {
		return nil, err
	}

	return challenge, nil
}

func (c *challengesvcs) Update(ctx context.Context, challengeId string, data *models.ChallengeDto) (*models.Challenge, error) {
	_id, err := primitive.ObjectIDFromHex(challengeId)
	if err != nil {
		return nil, errors.New("invalid challenge id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	challenge := &models.Challenge{
		Id:           _id,
		ChallengeDto: *data,
		UpdatedAt:    time.Now(),
		UpdatedBy:    cfg.User.Id,
	}
	ensureQuestionIds(&challenge.Quiz)

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": challenge}

	updatedChallenge, err := c.challengeRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	return updatedChallenge, nil
}

func (c *challengesvcs) Delete(ctx context.Context, challengeId string) error {
	_id, err := primitive.ObjectIDFromHex(challengeId)
	if err != nil {
		return errors.New("invalid challenge id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": bson.M{"trash": true, "updatedAt": time.Now(), "updatedBy": cfg.User.Id}}

	_, err = c.challengeRepo.Patch(ctx, filter, update)
	if err != nil {
		return err
	}

	return nil
}

// ensureQuestionIds assigns ObjectIDs to questions that have zero IDs
func ensureQuestionIds(quiz *models.ChallengeQuiz) {
	for i := range quiz.Questions {
		if quiz.Questions[i].Id.IsZero() {
			quiz.Questions[i].Id = primitive.NewObjectID()
		}
	}
}
