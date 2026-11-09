package content

import (
	"context"
	"errors"
	"fmt"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/services/content/enums"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/services/sales"
	salesModels "larsa-academy-microservice/pkg/services/sales/models"
	"larsa-academy-microservice/pkg/util"
	"time"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var coachingPricingLookup = bson.M{
	"$lookup": bson.M{
		"from": "academyCoachingPricings",
		"let": bson.M{
			"coachingIds": bson.M{"$ifNull": bson.A{"$coachingIds", bson.A{}}},
		},
		"pipeline": []bson.M{
			{"$match": bson.M{"$expr": bson.M{
				"$and": bson.A{
					bson.M{"$in": bson.A{"$coachingId", "$$coachingIds"}},
					bson.M{"$eq": bson.A{"$active", true}},
					bson.M{"$eq": bson.A{"$trash", false}},
				},
			}}},
		},
		"as": "coachingsPricings",
	},
}

var coursePricingLookup = bson.M{
	"$lookup": bson.M{
		"from": "academyCoursePricings",
		"let": bson.M{
			"courseIds": bson.M{"$ifNull": bson.A{"$courseIds", bson.A{}}},
		},
		"pipeline": []bson.M{
			{"$match": bson.M{"$expr": bson.M{
				"$and": bson.A{
					bson.M{"$in": bson.A{"$courseId", "$$courseIds"}},
					bson.M{"$eq": bson.A{"$config.isDefault", true}},
					bson.M{"$eq": bson.A{"$trash", false}},
				},
			}}},
		},
		"as": "coursesPricings",
	},
}

var bundlePricingLookup = bson.M{
	"$lookup": bson.M{
		"from": "academyBundlePricings",
		"let": bson.M{
			"bundleIds": bson.M{"$ifNull": bson.A{"$bundleIds", bson.A{}}},
		},
		"pipeline": []bson.M{
			{"$match": bson.M{"$expr": bson.M{
				"$and": bson.A{
					bson.M{"$in": bson.A{"$bundleId", "$$bundleIds"}},
					bson.M{"$eq": bson.A{"$active", true}},
					bson.M{"$eq": bson.A{"$trash", false}},
				},
			}}},
		},
		"as": "bundlesPricings",
	},
}

var printedMaterialLookup = bson.M{
	"$lookup": bson.M{
		"from": "academyPrintedMaterials",
		"let": bson.M{
			"materialIds": bson.M{"$ifNull": bson.A{"$materialIds", bson.A{}}},
		},
		"pipeline": []bson.M{
			{"$match": bson.M{"$expr": bson.M{
				"$and": bson.A{
					bson.M{"$in": bson.A{"$_id", "$$materialIds"}},
					bson.M{"$eq": bson.A{"$trash", false}},
				},
			}}},
		},
		"as": "materialsPricings",
	},
}

type CartSvcs interface {
	MyCart(ctx context.Context) (*models.Cart, error)
	AddToCart(ctx context.Context, data *models.CartItemDto) (*models.Cart, error)
	UpdateCartItem(ctx context.Context, cartItemId string, data *models.CartItemDto) (*models.Cart, error)
	DeleteCartItem(ctx context.Context, cartId string) (*models.Cart, error)
	ClearCart(ctx context.Context) error
	Checkout(ctx context.Context) (*models.Order, error)
	ApplyCoupon(ctx context.Context, code string) (*models.Cart, error)
	RemoveCoupon(ctx context.Context) (*models.Cart, error)
}

type cartsvcs struct {
	cartRepo db.MainRepo[models.Cart]
	//
	coachingRepo        db.MainRepo[models.Coaching]
	courseRepo          db.MainRepo[models.Course]
	studentSessionRepo  db.MainRepo[models.StudentSession]
	studentCourseRepo   db.MainRepo[models.StudentCourse]
	studentCoachingRepo db.MainRepo[models.CoachingStudent]
	studentBundleRepo   db.MainRepo[models.StudentBundle]
	renewalRepo         db.MainRepo[models.Renewal]
	bundleRepo                 db.MainRepo[models.Bundle]
	subjectRepo                db.MainRepo[models.Subject]
	orderRepo                  db.MainRepo[models.Order]
	printedMaterialRepo        db.MainRepo[models.PrintedMaterial]
	printedMaterialStudentRepo db.MainRepo[models.PrintedMaterialStudent]
	//pricing
	invoiceSvcs sales.InvoiceSvcs
	pricingSvcs sales.PricingSvcs
	//transation
	withtxn *db.WithTxn
}

func NewCartSvcs(i *do.Injector) (CartSvcs, error) {
	invoiceSvcs, err := do.Invoke[sales.InvoiceSvcs](i)
	if err != nil {
		return nil, err
	}
	pricingSvcs, err := do.Invoke[sales.PricingSvcs](i)
	if err != nil {
		return nil, err
	}
	return &cartsvcs{
		cartRepo:            db.NewRepo[models.Cart](i),
		coachingRepo:        db.NewRepo[models.Coaching](i),
		courseRepo:          db.NewRepo[models.Course](i),
		studentSessionRepo:  db.NewRepo[models.StudentSession](i),
		studentCourseRepo:   db.NewRepo[models.StudentCourse](i),
		studentCoachingRepo: db.NewRepo[models.CoachingStudent](i),
		studentBundleRepo:   db.NewRepo[models.StudentBundle](i),
		renewalRepo:         db.NewRepo[models.Renewal](i),
		bundleRepo:                 db.NewRepo[models.Bundle](i),
		subjectRepo:                db.NewRepo[models.Subject](i),
		orderRepo:                  db.NewRepo[models.Order](i),
		printedMaterialRepo:        db.NewRepo[models.PrintedMaterial](i),
		printedMaterialStudentRepo: db.NewRepo[models.PrintedMaterialStudent](i),
		//pricing
		invoiceSvcs: invoiceSvcs,
		pricingSvcs: pricingSvcs,
		//
		withtxn: do.MustInvoke[*db.WithTxn](i),
	}, nil
}

func (c *cartsvcs) metaData(ctx context.Context, data *models.CartItemDto) (map[string]any, error) {
	var meta map[string]any
	switch data.Type {
	case enums.CartTypeCoaching:
		{
			cs, err := util.Converter[*models.CoachingSubscribe](data.Data)
			if err != nil {
				return nil, err
			}

			if cs == nil {
				return nil, errors.New("missing coaching subscribe data")
			}

			if cs.NumOfSessions%2 != 0 {
				return nil, errors.New("number of sessions must be even")
			}

			coaching, err := c.coachingRepo.GetByFilter(ctx, bson.M{"_id": data.RefId, "trash": false})
			if err != nil {
				return nil, errors.New("coaching program not found")
			}
			if cs.NumOfSessions < coaching.MinSessionNum || cs.NumOfSessions > coaching.MaxSessionNum {
				return nil, errors.New("error number of sessions")
			}

			if _, err := c.pricingSvcs.GetPrice(ctx, data.Type, data.RefId, data.Data); err != nil {
				return nil, fmt.Errorf("failed to get a pricing: %w", err)
			}

			subject, err := c.subjectRepo.GetByFilter(ctx, bson.M{"_id": coaching.Subject, "trash": false})
			var subjects []any
			if err == nil && subject != nil {
				subjects = append(subjects, subject.Name)
			}

			meta = map[string]any{
				"numOfSessions":   cs.NumOfSessions,
				"title":           coaching.Title,
				"description":     coaching.Description,
				"subjects":        subjects,
				"thumbnail":       coaching.Thumbnail,
				"sessionDuration": coaching.SessionDuration,
			}

		}
	case enums.CartTypeCourse:
		{
			_, err := util.Converter[*models.CourseSubscribe](data.Data)
			if err != nil {
				return nil, err
			}

			course, err := c.courseRepo.GetByFilter(ctx, bson.M{"_id": data.RefId, "trash": false})
			if err != nil {
				return nil, errors.New("course not found")
			}

			_, err = c.pricingSvcs.GetPrice(ctx, data.Type, data.RefId, data.Data)
			if err != nil {
				return nil, fmt.Errorf("failed to get a pricing: %w", err)
			}

			var resolvedSubjects []any
			subjectIdToIndex := make(map[primitive.ObjectID]int)
			for _, id := range course.BasicInfo.LinkedSubjects {
				subjectIdToIndex[id] = len(resolvedSubjects)
				resolvedSubjects = append(resolvedSubjects, nil)
			}

			if len(subjectIdToIndex) > 0 {
				ids := make([]primitive.ObjectID, 0, len(subjectIdToIndex))
				for id := range subjectIdToIndex {
					ids = append(ids, id)
				}
				var subjects []models.Subject
				err := c.subjectRepo.Aggregate(ctx, []bson.M{{"$match": bson.M{"_id": bson.M{"$in": ids}, "trash": false}}}, func(cursor *mongo.Cursor) error {
					return cursor.All(ctx, &subjects)
				})
				if err == nil {
					for _, s := range subjects {
						resolvedSubjects[subjectIdToIndex[s.Id]] = s.Name
					}
				}

				// Remove nil entries (subjects not found)
				var filteredSubjects []any
				for _, s := range resolvedSubjects {
					if s != nil {
						filteredSubjects = append(filteredSubjects, s)
					}
				}
				resolvedSubjects = filteredSubjects
			}

			meta = map[string]any{
				"title":       course.BasicInfo.Name,
				"description": course.BasicInfo.Description,
				"subjects":    resolvedSubjects,
				"thumbnail":   course.BasicInfo.Thumbnail,
			}

		}
	case enums.CartTypeBundle:
		{
			_, err := util.Converter[*models.BundleSubscribe](data.Data)
			if err != nil {
				return nil, err
			}

			bundle, err := c.bundleRepo.GetByFilter(ctx, bson.M{"_id": data.RefId, "trash": false})
			if err != nil {
				return nil, errors.New("error get bundle")
			}

			_, err = c.pricingSvcs.GetPrice(ctx, data.Type, data.RefId, data.Data)
			if err != nil {
				return nil, fmt.Errorf("failed to get a pricing: %w", err)
			}

			resolvedSubjects := make([]any, len(bundle.Subject))
			subjectIdToIndex := make(map[primitive.ObjectID]int)
			for i, s := range bundle.Subject {
				if id, err := primitive.ObjectIDFromHex(s); err == nil {
					subjectIdToIndex[id] = i
				} else {
					resolvedSubjects[i] = s
				}
			}

			if len(subjectIdToIndex) > 0 {
				ids := make([]primitive.ObjectID, 0, len(subjectIdToIndex))
				for id := range subjectIdToIndex {
					ids = append(ids, id)
				}
				var subjects []models.Subject
				err := c.subjectRepo.Aggregate(ctx, []bson.M{{"$match": bson.M{"_id": bson.M{"$in": ids}, "trash": false}}}, func(cursor *mongo.Cursor) error {
					return cursor.All(ctx, &subjects)
				})
				if err == nil {
					for _, s := range subjects {
						resolvedSubjects[subjectIdToIndex[s.Id]] = s.Name
					}
				}
				// Remove nil entries (subjects not found)
				var filteredSubjects []any
				for _, s := range resolvedSubjects {
					if s != nil {
						filteredSubjects = append(filteredSubjects, s)
					}
				}
				resolvedSubjects = filteredSubjects
			}

			meta = map[string]any{
				"title":       bundle.Name,
				"description": bundle.Description,
				"subjects":    resolvedSubjects,
				"thumbnail":   bundle.Thumbnail,
			}

		}
	case enums.CartTypeMaterial:
		{
			_, err := util.Converter[*models.MaterialSubscribe](data.Data)
			if err != nil {
				return nil, err
			}

			material, err := c.printedMaterialRepo.GetByFilter(ctx, bson.M{"_id": data.RefId, "trash": false})
			if err != nil {
				return nil, errors.New("printed material not found")
			}

			if _, err := c.pricingSvcs.GetPrice(ctx, data.Type, data.RefId, data.Data); err != nil {
				return nil, fmt.Errorf("failed to get pricing for material: %w", err)
			}

			meta = map[string]any{
				"title":       material.Title,
				"description": material.Description,
				"thumbnail":   material.Image,
			}
		}
	default:
		return nil, errors.New("invalid cart type")
	}

	return meta, nil
}

func (c *cartsvcs) checkExistingEnrollment(ctx context.Context, studentId primitive.ObjectID, itemType enums.CartType, refId primitive.ObjectID) error {
	switch itemType {
	case enums.CartTypeCourse:
		// Check if it's a renewable course
		isRenewable, err := c.renewalRepo.Count(ctx, bson.M{
			"studentId":   studentId,
			"refId":       refId,
			"contentType": enums.RenewalContentTypeCourse,
			"trash":       false,
		})
		if err != nil {
			return err
		}

		// If not renewable, check for direct enrollment
		if isRenewable == 0 {
			count, err := c.studentCourseRepo.Count(ctx, bson.M{
				"studentId": studentId,
				"courseId":  refId,
				"trash":     false,
			})
			if err == nil && count > 0 {
				return errors.New("you are already enrolled in this course")
			}
		}

		// Check if enrolled via bundle
		var bundles []models.StudentBundle
		err = c.studentBundleRepo.Aggregate(ctx, []bson.M{
			{"$match": bson.M{"studentId": studentId, "trash": false}},
			{"$lookup": bson.M{
				"from":         "academyBundles",
				"localField":   "bundleId",
				"foreignField": "_id",
				"as":           "bundleInfo",
			}},
			{"$unwind": "$bundleInfo"},
			{"$match": bson.M{"bundleInfo.content.refId": refId, "bundleInfo.content.type": "course"}},
		}, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &bundles)
		})
		if err == nil && len(bundles) > 0 {
			return fmt.Errorf("you already have access to this course through bundle: %s", bundles[0].Id.Hex())
		}

	case enums.CartTypeCoaching:
		// Coaching always allowed (for top-ups)
		return nil

	case enums.CartTypeBundle:
		count, err := c.studentBundleRepo.Count(ctx, bson.M{
			"studentId": studentId,
			"bundleId":  refId,
			"trash":     false,
		})
		if err == nil && count > 0 {
			return errors.New("you are already enrolled in this bundle")
		}

	case enums.CartTypeMaterial:
		count, err := c.printedMaterialStudentRepo.Count(ctx, bson.M{
			"studentId":  studentId,
			"materialId": refId,
			"trash":      false,
		})
		if err == nil && count > 0 {
			return errors.New("you already own this printed material")
		}
	}
	return nil
}

func (c *cartsvcs) AddToCart(ctx context.Context, data *models.CartItemDto) (*models.Cart, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Validate enrollment
	if err := c.checkExistingEnrollment(ctx, cfg.User.Id, data.Type, data.RefId); err != nil {
		return nil, err
	}

	cart, err := c.cartRepo.GetByFilter(ctx, bson.M{"studentId": cfg.User.Id})
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			meta, err := c.metaData(ctx, data)
			if err != nil {
				return nil, err
			}

			cartItem := models.CartItem{
				Id:          primitive.NewObjectID(),
				CartItemDto: *data,
				Meta:        meta,
				CreatedAt:   time.Now(),
			}

			cart = &models.Cart{
				Id:        primitive.NewObjectID(),
				StudentId: cfg.User.Id,
				Items: []models.CartItem{
					cartItem,
				},
				CoachingIds: []primitive.ObjectID{},
				CourseIds:   []primitive.ObjectID{},
				BundleIds:   []primitive.ObjectID{},
				MaterialIds: []primitive.ObjectID{},
			}

			cart.SortIds()

			if err := c.cartRepo.Add(ctx, cart); err != nil {
				return nil, fmt.Errorf("failed to add cart: %w", err)
			}
			return c.MyCart(ctx)
		} else {
			return nil, fmt.Errorf("failed to get cart: %w", err)
		}

	} else {
		items := cart.Items
		for _, item := range items {
			if item.Type == data.Type && item.RefId == data.RefId {
				return nil, errors.New("item already in cart")
			}
		}

		meta, err := c.metaData(ctx, data)
		if err != nil {
			return nil, err
		}

		item := models.CartItem{
			Id:          primitive.NewObjectID(),
			CartItemDto: *data,
			Meta:        meta,
			CreatedAt:   time.Now(),
		}
		items = append(items, item)
		cart.Items = items
		cart.SortIds()

		_, err = c.cartRepo.Patch(ctx, bson.M{"_id": cart.Id}, bson.M{"$set": cart})
		if err != nil {
			return nil, fmt.Errorf("failed to update cart: %w", err)
		}
		return c.MyCart(ctx)

	}

}

func (c *cartsvcs) UpdateCartItem(ctx context.Context, cartItemId string, data *models.CartItemDto) (*models.Cart, error) {
	_id, err := primitive.ObjectIDFromHex(cartItemId)
	if err != nil {
		return nil, errors.New("invalid cart id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	cart, err := c.cartRepo.GetByFilter(ctx, bson.M{"studentId": cfg.User.Id})
	if err != nil {
		return nil, fmt.Errorf("failed to get cart: %w", err)
	}

	var itemToUpdate *models.CartItem
	for _, item := range cart.Items {
		if item.Id == _id {
			itemToUpdate = &item
			break
		}
	}

	if itemToUpdate == nil {
		return nil, errors.New("cart item not found")
	}

	// Validate enrollment if RefId or Type changed
	if itemToUpdate.RefId != data.RefId || itemToUpdate.Type != data.Type {
		if err := c.checkExistingEnrollment(ctx, cfg.User.Id, data.Type, data.RefId); err != nil {
			return nil, err
		}
	}

	meta, err := c.metaData(ctx, data)
	if err != nil {
		return nil, err
	}
	itemToUpdate.Meta = meta
	itemToUpdate.CartItemDto = *data
	itemToUpdate.CreatedAt = time.Now()

	updatedItems := []models.CartItem{}
	for _, item := range cart.Items {
		if item.Id == _id {
			updatedItems = append(updatedItems, *itemToUpdate)
		} else {
			updatedItems = append(updatedItems, item)
		}
	}

	cart.Items = updatedItems
	cart.SortIds()

	filter := bson.M{"_id": cart.Id}
	update := bson.M{"$set": cart}
	_, err = c.cartRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, fmt.Errorf("failed to update cart: %w", err)
	}
	return c.MyCart(ctx)
}

func (c *cartsvcs) DeleteCartItem(ctx context.Context, cartItemId string) (*models.Cart, error) {
	_id, err := primitive.ObjectIDFromHex(cartItemId)
	if err != nil {
		return nil, errors.New("invalid cart item id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	cart, err := c.cartRepo.GetByFilter(ctx, bson.M{"studentId": cfg.User.Id})
	if err != nil {
		return nil, fmt.Errorf("failed to get cart: %w", err)
	}

	updatedItems := []models.CartItem{}
	for _, item := range cart.Items {
		if item.Id != _id {
			updatedItems = append(updatedItems, item)
		}
	}

	cart.Items = updatedItems
	cart.SortIds()

	filter := bson.M{"_id": cart.Id}
	update := bson.M{"$set": cart}
	_, err = c.cartRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, fmt.Errorf("failed to update cart: %w", err)
	}
	return c.MyCart(ctx)
}

func (c *cartsvcs) enrichCartItemsMeta(ctx context.Context, items []models.CartItem) {
	for i, item := range items {
		if (item.Type == enums.CartTypeBundle || item.Type == enums.CartTypeCourse || item.Type == enums.CartTypeCoaching) && item.Meta != nil {
			if subjectsRaw, ok := item.Meta["subjects"]; ok {
				var subjects []any
				switch v := subjectsRaw.(type) {
				case []string:
					for _, s := range v {
						subjects = append(subjects, s)
					}
				case []any:
					subjects = v
				case primitive.A:
					for _, s := range v {
						subjects = append(subjects, s)
					}
				}

				if len(subjects) > 0 {
					resolvedSubjects := make([]any, len(subjects))
					subjectIdToIndex := make(map[primitive.ObjectID]int)
					hasIds := false
					for j, s := range subjects {
						if str, ok := s.(string); ok {
							if id, err := primitive.ObjectIDFromHex(str); err == nil {
								subjectIdToIndex[id] = j
								hasIds = true
								continue
							}
						}
						resolvedSubjects[j] = s
					}

					if hasIds {
						ids := make([]primitive.ObjectID, 0, len(subjectIdToIndex))
						for id := range subjectIdToIndex {
							ids = append(ids, id)
						}
						var subjectsObjs []models.Subject
						err := c.subjectRepo.Aggregate(ctx, []bson.M{{"$match": bson.M{"_id": bson.M{"$in": ids}, "trash": false}}}, func(cursor *mongo.Cursor) error {
							return cursor.All(ctx, &subjectsObjs)
						})
						if err == nil {
							for _, s := range subjectsObjs {
								resolvedSubjects[subjectIdToIndex[s.Id]] = s.Name
							}
						}
					}

					// Remove nil entries (subjects that were IDs but not found)
					var filteredSubjects []any
					for _, s := range resolvedSubjects {
						if s != nil {
							filteredSubjects = append(filteredSubjects, s)
						}
					}
					items[i].Meta["subjects"] = filteredSubjects
				}
			}
		}
	}
}

func (c *cartsvcs) MyCart(ctx context.Context) (*models.Cart, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	filter := bson.M{"studentId": cfg.User.Id}

	pipeline := []bson.M{
		{"$match": filter},
		coachingPricingLookup,
		coursePricingLookup,
		bundlePricingLookup,
		printedMaterialLookup,
	}

	type aux struct {
		models.Cart         `bson:",inline"`
		models.PricingPlans `bson:",inline"`
	}
	var result []aux
	if err := c.cartRepo.Aggregate(ctx, pipeline, func(cursor *mongo.Cursor) error {
		return cursor.All(ctx, &result)
	}); err != nil {
		return nil, fmt.Errorf("failed to get cart: %w", err)
	}

	if len(result) == 0 {
		// Return empty cart instead of error
		return &models.Cart{
			Id:          primitive.NilObjectID,
			StudentId:   cfg.User.Id,
			Items:       []models.CartItem{},
			CoachingIds: []primitive.ObjectID{},
			CourseIds:   []primitive.ObjectID{},
			BundleIds:   []primitive.ObjectID{},
			MaterialIds: []primitive.ObjectID{},
			TotalPrice:  0,
			Currency:    "USD",
		}, nil
	}

	cartData := result[0]

	c.enrichCartItemsMeta(ctx, cartData.Cart.Items)

	priceResults, err := c.pricingSvcs.GetPricesBatch(ctx, cartData.Cart.Items)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate prices: %w", err)
	}

	var subTotal float64
	for i := range cartData.Cart.Items {
		for _, pr := range priceResults {
			if cartData.Cart.Items[i].RefId == pr.RefId {
				if pr.Error == "" {
					cartData.Cart.Items[i].Price = &pr.Price
					subTotal += pr.Price
					cartData.Cart.Items[i].Currency = "USD"
				}
				break
			}
		}
	}

	// Persist prices in cart items (optional but helpful for checkout source of truth)
	_, _ = c.cartRepo.Patch(ctx, bson.M{"_id": cartData.Cart.Id}, bson.M{"$set": bson.M{"items": cartData.Cart.Items}})

	// Apply coupon if present
	if cartData.Cart.CouponCode != "" {
		calcRes, err := c.invoiceSvcs.CalculateInvoiceTotal(ctx, &salesModels.InvoiceTotalCalculationRequest{
			SubTotal:   subTotal,
			CouponCode: cartData.Cart.CouponCode,
		})
		if err == nil && calcRes != nil {
			cartData.Cart.TotalPrice = calcRes.TotalAmount
			cartData.Cart.Discount = calcRes.Discount
			cartData.Cart.CouponCode = calcRes.CouponCode // Ensure we use normalized code
		} else {
			// If coupon is invalid, clear it in the DB and returned object
			_, _ = c.cartRepo.Patch(ctx, bson.M{"_id": cartData.Cart.Id}, bson.M{"$unset": bson.M{"couponCode": ""}})
			cartData.Cart.CouponCode = ""
			cartData.Cart.TotalPrice = subTotal
			cartData.Cart.Discount = 0
		}
	} else {
		cartData.Cart.TotalPrice = subTotal
		cartData.Cart.Discount = 0
	}

	cartData.Cart.Currency = "USD"

	return &cartData.Cart, nil
}

func (c *cartsvcs) ApplyCoupon(ctx context.Context, code string) (*models.Cart, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	cart, err := c.MyCart(ctx)
	if err != nil {
		return nil, err
	}

	if len(cart.Items) == 0 {
		return nil, errors.New("cart is empty")
	}

	// Validate coupon
	_, err = c.invoiceSvcs.CalculateInvoiceTotal(ctx, &salesModels.InvoiceTotalCalculationRequest{
		SubTotal:   cart.TotalPrice + cart.Discount, // use subtotal
		CouponCode: code,
	})
	if err != nil {
		return nil, fmt.Errorf("invalid coupon: %w", err)
	}

	_, err = c.cartRepo.Patch(ctx, bson.M{"studentId": cfg.User.Id}, bson.M{"$set": bson.M{"couponCode": code}})
	if err != nil {
		return nil, err
	}

	return c.MyCart(ctx)
}

func (c *cartsvcs) RemoveCoupon(ctx context.Context) (*models.Cart, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	cart, err := c.cartRepo.GetByFilter(ctx, bson.M{"studentId": cfg.User.Id})
	if err != nil {
		return nil, err
	}

	_, err = c.cartRepo.Patch(ctx, bson.M{"_id": cart.Id}, bson.M{"$unset": bson.M{"couponCode": ""}})
	if err != nil {
		return nil, err
	}

	return c.MyCart(ctx)
}

func (c *cartsvcs) ClearCart(ctx context.Context) error {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	if err := c.cartRepo.DeleteMain(ctx, bson.M{"studentId": cfg.User.Id}); err != nil {
		return fmt.Errorf("failed to clear cart: %w", err)
	}

	return nil
}

func (c *cartsvcs) Checkout(ctx context.Context) (*models.Order, error) {
	result, err := c.withtxn.Exec(ctx, func(ctx mongo.SessionContext) (any, error) {
		cfg, err := util.GetReqAppCfg(ctx)

		if err != nil {
			return nil, err
		}

		cart, err := c.MyCart(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get cart: %w", err)
		}

		if len(cart.Items) == 0 {
			return nil, errors.New("cart is empty")
		}

		order := &models.Order{
			Id:         primitive.NewObjectID(),
			StudentId:  cfg.User.Id,
			Items:      cart.Items,
			TotalPrice: cart.TotalPrice,
			CouponCode: cart.CouponCode,
			Discount:   cart.Discount,
			Status:     enums.OrderStatusPending,
			CreatedAt:  time.Now(),
			CreatedBy:  cfg.User.Id,
			UpdatedAt:  time.Now(),
			UpdatedBy:  cfg.User.Id,
		}

		order.SortIds()

		// Initial creation of invoice from order
		inv, err := c.invoiceSvcs.CreateFromOrder(ctx, order)
		if err != nil {
			return nil, fmt.Errorf("failed to create invoice: %w", err)
		}

		// Update order with invoice ID and sync total price from invoice (Source of Truth)
		order.InvoiceId = inv.ID
		order.TotalPrice = inv.TotalAmount

		if err := c.orderRepo.Add(ctx, order); err != nil {
			return nil, fmt.Errorf("failed to add order: %w", err)
		}

		if err := c.cartRepo.DeleteMain(ctx, bson.M{"studentId": cfg.User.Id}); err != nil {
			return nil, fmt.Errorf("failed to delete cart: %w", err)
		}

		return order, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to checkout: %w", err)
	}

	return result.(*models.Order), nil
}
