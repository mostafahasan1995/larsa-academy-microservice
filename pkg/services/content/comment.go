package content

import (
	"context"
	"errors"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/content/enums"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"math"
	"time"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/sync/errgroup"
)

var populateUserField = []bson.M{
	{
		"$lookup": bson.M{
			"from":         "users",
			"localField":   "userId",
			"foreignField": "_id",
			"as":           "userData",
		},
	},
	{
		"$unwind": bson.M{
			"path":                       "$userData",
			"preserveNullAndEmptyArrays": true,
		},
	},
	{
		"$set": bson.M{
			"firstName": "$userData.firstName",
			"lastName":  "$userData.lastName",
			"email":     "$userData.email",
			"picture":   "$userData.picture",
		},
	},
}

type CommentSvcs interface {
	GetCommentsByRef(ctx context.Context, typ enums.CommentType, refId string, skip, limit int64, query *query.Conditions) (*models.CommentPagination, error)
	AddComment(ctx context.Context, data *models.CommentDto) (*models.Comment, error)
	UpdateComments(ctx context.Context, commentId string, data *models.CommentDto) (*models.Comment, error)
	DeleteComment(ctx context.Context, commentId string) error
}

type commentsvcs struct {
	commentRepo db.MainRepo[models.Comment]
}

func NewCommentSvcs(i *do.Injector) (CommentSvcs, error) {
	return &commentsvcs{
		commentRepo: db.NewRepo[models.Comment](i),
	}, nil
}

func (c *commentsvcs) GetCommentsByRef(ctx context.Context, typ enums.CommentType, refId string, skip, limit int64, query *query.Conditions) (*models.CommentPagination, error) {
	_id, err := primitive.ObjectIDFromHex(refId)
	if err != nil {
		return nil, errors.New("invalid ref id")
	}

	if err := query.CheckValid(); err != nil {
		return nil, err
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, err
	}

	pipeline := []bson.M{
		{"$match": bson.M{
			"refId":   _id,
			"type":    typ,
			"replyTo": primitive.NilObjectID,
			"trash":   false,
			"$and":    []bson.M{filter},
		}},
	}

	count, err := c.commentRepo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})
	pipeline = append(pipeline, bson.M{"$skip": skip})
	pipeline = append(pipeline, bson.M{"$limit": limit})
	pipeline = append(pipeline, populateUserField...)

	var result []models.CommentRes
	if err := c.commentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	var f func(comment models.CommentRes) (*models.CommentRes, error)

	f = func(comment models.CommentRes) (*models.CommentRes, error) {
		pipeline := []bson.M{
			{"$match": bson.M{"replyTo": comment.Id, "trash": false}},
		}

		pipeline = append(pipeline, populateUserField...)

		var replies []models.CommentRes
		if err := c.commentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &replies)
		}); err != nil {
			return nil, err
		}

		var commentReplies []models.CommentRes
		for _, reply := range replies {
			com, err := f(reply)
			if err != nil {
				return nil, err
			}
			commentReplies = append(commentReplies, *com)
		}

		comment.Replies = commentReplies

		return &comment, nil

	}

	resultWithReplies := make([]models.CommentRes, len(result))
	var g errgroup.Group

	for index, comment := range result {

		g.Go(func() error {

			com, err := f(comment)
			if err != nil {
				return err
			}

			resultWithReplies[index] = *com
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	totalPages := math.Ceil(float64(count) / float64(limit))
	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: count,
	}

	return &models.CommentPagination{
		Comments:   resultWithReplies,
		Pagination: pagination,
	}, nil

}

func (c *commentsvcs) AddComment(ctx context.Context, data *models.CommentDto) (*models.Comment, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	comment := &models.Comment{
		Id:         primitive.NewObjectID(),
		UserId:     cfg.User.Id,
		CommentDto: *data,
		CreatedAt:  time.Now(),
		CreatedBy:  cfg.User.Id,
		UpdatedAt:  time.Now(),
		UpdatedBy:  cfg.User.Id,
	}

	if err := c.commentRepo.Add(ctx, comment); err != nil {
		return nil, err
	}

	return comment, nil
}

func (c *commentsvcs) UpdateComments(ctx context.Context, commentId string, data *models.CommentDto) (*models.Comment, error) {
	_id, err := primitive.ObjectIDFromHex(commentId)
	if err != nil {
		return nil, errors.New("invalid comment id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": bson.M{
		"content":   data.Content,
		"isPublish": data.IsPublish,
		"updatedAt": time.Now(),
		"updatedBy": cfg.User.Id,
	}}

	updatedComment, err := c.commentRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, errors.New("failed to update comment")
	}

	return updatedComment, nil
}

func (c *commentsvcs) DeleteComment(ctx context.Context, commentId string) error {
	_id, err := primitive.ObjectIDFromHex(commentId)
	if err != nil {
		return errors.New("invalid comment id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": bson.M{"trash": true, "updatedAt": time.Now(), "updatedBy": cfg.User.Id}}

	if _, err := c.commentRepo.Patch(ctx, filter, update); err != nil {
		return errors.New("failed to delete comment")
	}

	return nil
}
