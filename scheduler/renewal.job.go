package scheduler

import (
	"context"
	"errors"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/services/content/enums"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/services/messaging"
	"larsa-academy-microservice/pkg/services/messaging/template"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"time"

	contactModels "larsa-academy-microservice/pkg/services/contact/models"

	messageEnums "larsa-academy-microservice/pkg/services/messaging/enums"
	messageModels "larsa-academy-microservice/pkg/services/messaging/models"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"github.com/rs/zerolog/log"
	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type StudentRenewal struct {
	StudentId primitive.ObjectID `bson:"_id"`
	Renewals  []models.Renewal   `bson:"renewals"`
}

type OrderData struct {
	models.Order `bson:",inline"`
	Student      contactModels.Student    `bson:"student"`
	Coachings    []models.Coaching        `bson:"coachings"`
	Courses      []models.Course          `bson:"courses"`
	Bundles      []models.Bundle          `bson:"bundles"`
	Materials    []models.PrintedMaterial `bson:"materials"`
}

type RenewalJob struct {
	locker              *Locker
	renewalRepo         db.MainRepo[models.Renewal]
	orderRepo           db.MainRepo[models.Order]
	studentRepo         db.MainRepo[contactModels.Student]
	studentCourseRepo   db.MainRepo[models.StudentCourse]
	studentCoachingRepo db.MainRepo[models.CoachingStudent]
	studentBundleRepo   db.MainRepo[models.StudentBundle]
	messageSvcs         messaging.MessageSvcs
	withtxn             *db.WithTxn
}

func NewRenewalJob(i *do.Injector, locker *Locker) *RenewalJob {
	return &RenewalJob{
		locker:              locker,
		renewalRepo:         db.NewRepo[models.Renewal](i),
		orderRepo:           db.NewRepo[models.Order](i),
		studentRepo:         db.NewRepo[contactModels.Student](i),
		studentCourseRepo:   db.NewRepo[models.StudentCourse](i),
		studentCoachingRepo: db.NewRepo[models.CoachingStudent](i),
		studentBundleRepo:   db.NewRepo[models.StudentBundle](i),
		messageSvcs:         do.MustInvoke[messaging.MessageSvcs](i),
		withtxn:             do.MustInvoke[*db.WithTxn](i),
	}
}

func (r *RenewalJob) Run() {
	ctx := context.Background()

	lock, err := r.locker.Lock(ctx, "renewal_job")
	if err != nil {
		log.Error().Msg("Error acquiring lock: " + err.Error())
		return
	}

	defer func() {
		if lock != nil {
			if err := r.locker.Unlock(ctx, lock.Id); err != nil {
				log.Error().Msg("Error releasing lock: " + err.Error())
			}
		}
	}()

	log.Info().Msg("Renewal job started")

	dbs := []string{
		"604674966fe156529e3e03e9", //development
		//"d30ib4k5drjs73d29sk0",
	}

	for _, db := range dbs {
		cfg := &types.AppCfg{
			Db: db,
			Hp: &common.HeaderParams{},
		}

		serviceToken, err := common.GetServiceToken("academy", cfg.Hp)
		if err != nil {
			log.Error().Msg("Error getting service token: " + err.Error())
			continue
		}
		cfg.Hp.ServiceToken = serviceToken

		ctx = context.WithValue(ctx, util.ReqAppCfg, cfg)

		if err := r.handleDbRenewals(ctx); err != nil {
			log.Error().Msg("Error handling renewals for db: " + db + " error: " + err.Error())
			continue
		}

		log.Info().Msg("Renewal job successfull for db: " + db)
	}
}

func (r *RenewalJob) handleDbRenewals(ctx context.Context) error {
	pipeline := []bson.M{
		{"$match": bson.M{
			"status":          enums.RenewalStatusActive,
			"trash":           false,
			"nextRenewalDate": bson.M{"$lte": time.Now().UTC()},
		}},
		{"$group": bson.M{"_id": "$studentId", "renewals": bson.M{"$push": "$$ROOT"}}},
	}

	var result []StudentRenewal
	err := r.renewalRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return err
	}

	if len(result) == 0 {
		log.Info().Msg("No renewals found")
		return nil
	}

	const batchSize = 100 // Adjust based on your needs

	for i := 0; i < len(result); i += batchSize {
		end := i + batchSize
		if end > len(result) {
			end = len(result)
		}

		batch := result[i:end]
		log.Info().Msgf("Processing batch %d-%d of %d", i+1, end, len(result))

		// Process this batch in a transaction
		orders, err := r.processBatch(ctx, batch)
		if err != nil {
			log.Error().Err(err).Msgf("Error processing batch %d-%d", i+1, end)
			// Decide if you want to continue or return error
			// Option 1: Return error and stop
			return err
			// Option 2: Log and continue with next batch
			// continue
		}

		if err := r.notifyStudents(ctx, orders); err != nil {
			log.Error().Err(err).Msgf("Error notifying students for batch %d-%d", i+1, end)
			return err
		}
	}

	return nil

}

func (r *RenewalJob) processBatch(ctx context.Context, data []StudentRenewal) ([]models.Order, error) {
	orders, err := r.withtxn.Exec(ctx, func(ctx mongo.SessionContext) (any, error) {

		renewalUpdateOps := []mongo.WriteModel{}
		orderInsertOps := []mongo.WriteModel{}
		courseUpdateOps := []mongo.WriteModel{}
		coachingUpdateOps := []mongo.WriteModel{}
		bundleUpdateOps := []mongo.WriteModel{}
		orders := []models.Order{}

		for _, studentRenewals := range data {

			items := []models.CartItem{}

			for _, renewal := range studentRenewals.Renewals {
				switch renewal.ContentType {
				case enums.RenewalContentTypeCourse:
					items = append(items, models.CartItem{
						CartItemDto: models.CartItemDto{
							Type:  enums.CartTypeCourse,
							RefId: renewal.RefId,
							Data:  renewal.Data,
						},
					})
					courseUpdateOps = append(courseUpdateOps, mongo.NewUpdateOneModel().
						SetFilter(bson.M{"studentId": renewal.StudentId, "courseId": renewal.RefId, "trash": false}).
						SetUpdate(bson.M{"$set": bson.M{"status": enums.EnrollmentStatusSuspended}}))
				case enums.RenewalContentTypeCoaching:
					items = append(items, models.CartItem{
						CartItemDto: models.CartItemDto{
							Type:  enums.CartTypeCoaching,
							RefId: renewal.RefId,
							Data:  renewal.Data,
						},
					})
					coachingUpdateOps = append(coachingUpdateOps, mongo.NewUpdateOneModel().
						SetFilter(bson.M{"studentId": renewal.StudentId, "coachingId": renewal.RefId, "trash": false}).
						SetUpdate(bson.M{"$set": bson.M{"status": enums.EnrollmentStatusSuspended}}))
				case enums.RenewalContentTypeBundle:
					items = append(items, models.CartItem{
						CartItemDto: models.CartItemDto{
							Type:  enums.CartTypeBundle,
							RefId: renewal.RefId,
							Data:  renewal.Data,
						},
					})
					bundleUpdateOps = append(bundleUpdateOps, mongo.NewUpdateOneModel().
						SetFilter(bson.M{"studentId": renewal.StudentId, "bundleId": renewal.RefId, "trash": false}).
						SetUpdate(bson.M{"$set": bson.M{"status": enums.EnrollmentStatusSuspended}}))
				case enums.RenewalContentTypeMaterial:
					items = append(items, models.CartItem{
						CartItemDto: models.CartItemDto{
							Type:  enums.CartTypeMaterial,
							RefId: renewal.RefId,
							Data:  renewal.Data,
						},
					})
				}

				if renewal.RenewalType == enums.RenewalTypeMonthly {
					renewal.NextRenewalDate = renewal.NextRenewalDate.AddDate(0, 1, 0)
					renewalUpdateOps = append(renewalUpdateOps, mongo.NewUpdateOneModel().SetFilter(bson.M{"_id": renewal.Id}).SetUpdate(bson.M{"$set": renewal}))
				} else if renewal.RenewalType == enums.RenewalTypeYearly {
					renewal.NextRenewalDate = renewal.NextRenewalDate.AddDate(1, 0, 0)
					renewalUpdateOps = append(renewalUpdateOps, mongo.NewUpdateOneModel().SetFilter(bson.M{"_id": renewal.Id}).SetUpdate(bson.M{"$set": renewal}))
				}
			}

			order := models.Order{
				Id:          primitive.NewObjectID(),
				StudentId:   studentRenewals.StudentId,
				Items:       items,
				CoachingIds: []primitive.ObjectID{},
				CourseIds:   []primitive.ObjectID{},
				BundleIds:   []primitive.ObjectID{},
				MaterialIds: []primitive.ObjectID{},
				TotalPrice:  0,
				Status:      enums.OrderStatusPending,
				IsRenewal:   true,
				CreatedAt:   time.Now(),
			}

			order.SortIds()

			orderInsertOps = append(orderInsertOps, mongo.NewInsertOneModel().SetDocument(order))
			orders = append(orders, order)

		}

		if len(orderInsertOps) > 0 {
			if _, err := r.orderRepo.BulkWrite(ctx, orderInsertOps); err != nil {
				return nil, err
			}
		}
		if len(renewalUpdateOps) > 0 {
			if _, err := r.renewalRepo.BulkWrite(ctx, renewalUpdateOps); err != nil {
				return nil, err
			}
		}

		if len(courseUpdateOps) > 0 {
			if _, err := r.studentCourseRepo.BulkWrite(ctx, courseUpdateOps); err != nil {
				return nil, err
			}
		}

		if len(coachingUpdateOps) > 0 {
			if _, err := r.studentCoachingRepo.BulkWrite(ctx, coachingUpdateOps); err != nil {
				return nil, err
			}
		}

		if len(bundleUpdateOps) > 0 {
			if _, err := r.studentBundleRepo.BulkWrite(ctx, bundleUpdateOps); err != nil {
				return nil, err
			}
		}

		return orders, nil

	})
	if err != nil {
		return nil, errors.New("error getting students")
	}

	return orders.([]models.Order), nil
}

func (r *RenewalJob) notifyStudents(ctx context.Context, orders []models.Order) error {

	orderIds := []primitive.ObjectID{}
	for _, order := range orders {
		orderIds = append(orderIds, order.Id)
	}

	pipeline := []bson.M{
		{"$match": bson.M{"_id": bson.M{"$in": orderIds}, "trash": false}},
		{
			"$lookup": bson.M{
				"from": "academyStudents",
				"let":  bson.M{"studentId": "$studentId"},
				"pipeline": []bson.M{
					{"$match": bson.M{"$expr": bson.M{"$eq": []interface{}{"$_id", "$$studentId"}}}},
				},
				"as": "student",
			},
		},
		{"$unwind": bson.M{
			"path":                       "$student",
			"preserveNullAndEmptyArrays": true,
		}},
		{
			"$lookup": bson.M{
				"from": "academyCoachings",
				"let": bson.M{
					"coachingIds": "$coachingIds",
				},
				"pipeline": []bson.M{
					{"$match": bson.M{"$expr": bson.M{
						"$and": bson.A{
							bson.M{"$in": bson.A{"$_id", "$$coachingIds"}},
							bson.M{"$eq": bson.A{"$trash", false}},
						},
					}}},
				},
				"as": "coachings",
			},
		},
		{
			"$lookup": bson.M{
				"from": "academyCourses",
				"let": bson.M{
					"courseIds": "$courseIds",
				},
				"pipeline": []bson.M{
					{"$match": bson.M{"$expr": bson.M{
						"$and": bson.A{
							bson.M{"$in": bson.A{"$_id", "$$courseIds"}},
							bson.M{"$eq": bson.A{"$trash", false}},
						},
					}}},
				},
				"as": "courses",
			},
		},
		{
			"$lookup": bson.M{
				"from": "academyBundles",
				"let": bson.M{
					"bundleIds": "$bundleIds",
				},
				"pipeline": []bson.M{
					{"$match": bson.M{"$expr": bson.M{
						"$and": bson.A{
							bson.M{"$in": bson.A{"$_id", "$$bundleIds"}},
							bson.M{"$eq": bson.A{"$trash", false}},
						},
					}}},
				},
				"as": "bundles",
			},
		},
		{
			"$lookup": bson.M{
				"from": "academyPrintedMaterials",
				"let": bson.M{
					"materialIds": "$materialIds",
				},
				"pipeline": []bson.M{
					{"$match": bson.M{"$expr": bson.M{
						"$and": bson.A{
							bson.M{"$in": bson.A{"$_id", "$$materialIds"}},
							bson.M{"$eq": bson.A{"$trash", false}},
						},
					}}},
				},
			},
			"as": "materials",
		},
	}

	var orderList []OrderData
	if err := r.orderRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &orderList)
	}); err != nil {
		return errors.New("error getting orders")
	}

	if err := r.sendRenewalEmail(ctx, orderList); err != nil {
		return err
	}
	if err := r.sendRenewalNotification(ctx, orderList); err != nil {
		return err
	}

	return nil
}

func (r *RenewalJob) sendRenewalNotification(ctx context.Context, orders []OrderData) error {
	for _, order := range orders {

		notification := &messageModels.Notification{
			ServiceName:     "academy",
			Title:           "Renewal Notification",
			Content:         "Your renewal is due on " + time.Now().Format("2006-01-02"),
			Recipient:       order.Student.Id.Hex(),
			Data:            map[string]string{},
			Url:             "",
			Badge:           0,
			Sound:           "",
			Image:           "",
			Icon:            "",
			AndroidColor:    "",
			AndroidSound:    "",
			AndroidPriority: "",
			IOSSound:        "",
		}

		r.messageSvcs.SendNotification(ctx, notification)
	}

	return nil
}

func (r *RenewalJob) sendRenewalEmail(ctx context.Context, orders []OrderData) error {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	plat, err := common.GetOptionValue("PLATFORM_NAME", cfg.Hp)
	if err != nil {
		return errors.New("error getting platform name")
	}
	platformName, ok := plat.(string)
	if !ok {
		platformName = "[Platform Name]"
	}

	val, err := common.GetOptionValue("BUSINESS_NAME", cfg.Hp)
	if err != nil {
		return errors.New("error getting business name")
	}
	businessName, ok := val.(string)
	if !ok {
		businessName = "[Business Name]"
	}

	for _, order := range orders {
		renewalDate := time.Now()
		renewalTplData := &template.RenewalTplData{
			MemberName:        order.Student.GeneralInfo.FirstName["en"] + " " + order.Student.GeneralInfo.LastName["en"],
			RenewalDate:       renewalDate,
			RenewalDateString: renewalDate.Format("2006-01-02"),
			CompanyName:       businessName,
			PlatformName:      platformName,
			Link:              "",
		}

		items := []string{}
		for _, item := range order.Items {
			switch item.Type {
			case enums.CartTypeCoaching:
				coaching, ok := util.SliceFind(order.Coachings, func(coaching models.Coaching) bool {
					return coaching.Id == item.RefId
				})
				if ok {
					items = append(items, coaching.Title["en"])
				}
			case enums.CartTypeCourse:
				course, ok := util.SliceFind(order.Courses, func(course models.Course) bool {
					return course.Id == item.RefId
				})
				if ok {
					items = append(items, course.BasicInfo.Name["en"])
				}
			case enums.CartTypeBundle:
				bundle, ok := util.SliceFind(order.Bundles, func(bundle models.Bundle) bool {
					return bundle.Id == item.RefId
				})
				if ok {
					items = append(items, bundle.Name["en"])
				}
			case enums.CartTypeMaterial:
				material, ok := util.SliceFind(order.Materials, func(material models.PrintedMaterial) bool {
					return material.Id == item.RefId
				})
				if ok {
					items = append(items, material.Title["en"])
				}
			}
		}

		renewalTplData.Items = items

		message, subject, err := r.messageSvcs.GetTemplateMessage(ctx, messageEnums.RENEWAL_EMAIL, renewalTplData)
		if err != nil {
			return err
		}

		log.Info().Msgf("Sending renewal email to %s %s", subject, message)

		email := &messageModels.Message{
			Type:        messageEnums.RENEWAL_EMAIL,
			Email:       order.Student.AccountStudentSetting.Email,
			Subject:     subject,
			Message:     message,
			MessageHtml: message,
			SenderId:    primitive.NilObjectID,
			Status:      "sent",
			Target:      "email",
			CreatedBy:   primitive.NilObjectID,
			CreatedAt:   time.Now(),
		}

		if err := r.messageSvcs.SendEmail(ctx, email); err != nil {
			return err
		}

	}

	return nil
}
