package sales

import (
	"context"
	"fmt"
	"larsa-academy-microservice/pkg/db"
	contentEnums "larsa-academy-microservice/pkg/services/content/enums"
	contentModels "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/services/exchange"
	"larsa-academy-microservice/pkg/services/sales/models"
	"larsa-academy-microservice/pkg/util"
	"log/slog"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type PricingSvcs interface {
	GetPrice(ctx context.Context, pType contentEnums.CartType, refId primitive.ObjectID, data any) (*models.PriceResult, error)
	GetPricesBatch(ctx context.Context, items []contentModels.CartItem) ([]models.PriceResult, error)
	GetPriceEnrichmentStages(ctx context.Context, pType contentEnums.CartType, idField string) ([]bson.M, error)
}

type pricingsvcs struct {
	coachingPricingRepo db.MainRepo[contentModels.CoachingPricing]
	coursePricingRepo   db.MainRepo[contentModels.CoursePricing]
	bundlePricingRepo   db.MainRepo[contentModels.BundlePricing]
	materialRepo        db.MainRepo[contentModels.PrintedMaterial]
	exchangeSvcs        exchange.ExchangeService
}

func NewPricingSvcs(i *do.Injector) (PricingSvcs, error) {
	exchangeSvcs, err := do.Invoke[exchange.ExchangeService](i)
	if err != nil {
		return nil, err
	}

	return &pricingsvcs{
		coachingPricingRepo: db.NewRepo[contentModels.CoachingPricing](i),
		coursePricingRepo:   db.NewRepo[contentModels.CoursePricing](i),
		bundlePricingRepo:   db.NewRepo[contentModels.BundlePricing](i),
		materialRepo:        db.NewRepo[contentModels.PrintedMaterial](i),
		exchangeSvcs:        exchangeSvcs,
	}, nil
}

func (s *pricingsvcs) GetPrice(ctx context.Context, pType contentEnums.CartType, refId primitive.ObjectID, data any) (*models.PriceResult, error) {
	result := &models.PriceResult{RefId: refId}

	switch pType {
	case contentEnums.CartTypeCoaching:
		cs, err := util.Converter[*contentModels.CoachingSubscribe](data)
		if err != nil || cs == nil {
			return nil, fmt.Errorf("failed to get coaching subscribe: %w", err)
		}
		pricing, err := s.coachingPricingRepo.GetByFilter(ctx, bson.M{"coachingId": refId, "active": true, "trash": false})
		if err != nil || pricing == nil {
			return nil, fmt.Errorf("coaching pricing not found")
		}
		result.Price = pricing.PricePerSession * float64(cs.NumOfSessions)
		result.Currency = pricing.Currency

	case contentEnums.CartTypeCourse:
		pricing, err := s.coursePricingRepo.GetByFilter(ctx, bson.M{"courseId": refId, "config.isDefault": true, "trash": false})
		if err != nil || pricing == nil {
			return nil, fmt.Errorf("course pricing not found")
		}
		result.Price = pricing.Price
		result.Currency = pricing.Currency

	case contentEnums.CartTypeBundle:
		pricing, err := s.bundlePricingRepo.GetByFilter(ctx, bson.M{"bundleId": refId, "active": true, "trash": false})
		if err != nil || pricing == nil {
			return nil, fmt.Errorf("bundle pricing not found")
		}
		result.Price = pricing.Price
		result.Currency = pricing.Currency

	case contentEnums.CartTypeMaterial:
		material, err := s.materialRepo.GetByFilter(ctx, bson.M{"_id": refId, "trash": false})
		if err != nil || material == nil {
			return nil, fmt.Errorf("material not found")
		}
		result.Price = material.Price
		result.Currency = material.Currency

	default:
		return nil, fmt.Errorf("invalid product type")
	}

	if result.Currency == "" || result.Currency == "USD" {
		result.Currency = "USD"
		return result, nil
	}

	convertedPrice, err := s.exchangeSvcs.Convert(ctx, result.Price, result.Currency, "USD")
	if err != nil {
		return nil, fmt.Errorf("failed to convert price to USD: %w", err)
	}
	result.Price = convertedPrice
	result.Currency = "USD"

	return result, nil
}

func (s *pricingsvcs) GetPriceEnrichmentStages(ctx context.Context, pType contentEnums.CartType, idField string) ([]bson.M, error) {
	rates, err := s.exchangeSvcs.GetRates(ctx)
	if err != nil {
		// Fallback: Use empty rates if service fails, price will stay in original currency or 0
		rates = map[string]float64{"USD": 1.0}
	}

	// Prepare rate switch for MongoDB
	rateBranches := []bson.M{}
	for curr, rate := range rates {
		rateBranches = append(rateBranches, bson.M{
			"case": bson.M{"$eq": []any{"$currency", curr}},
			"then": rate,
		})
	}

	// Default to 1.0 if currency not found (avoids division by zero)
	conversionRateExpr := bson.M{
		"$switch": bson.M{
			"branches": rateBranches,
			"default":  1.0,
		},
	}

	var fromCollection string
	var matchExpr bson.M
	var foreignIdField string

	switch pType {
	case contentEnums.CartTypeCoaching:
		fromCollection = "academyCoachingPricings"
		foreignIdField = "coachingId"
		matchExpr = bson.M{
			"$expr": bson.M{"$and": []bson.M{
				{"$eq": []any{"$" + foreignIdField, "$$entityId"}},
				{"$eq": []any{"$active", true}},
				{"$eq": []any{"$trash", false}},
			}},
		}
	case contentEnums.CartTypeCourse:
		fromCollection = "academyCoursePricings"
		foreignIdField = "courseId"
		matchExpr = bson.M{
			"$expr": bson.M{"$and": []bson.M{
				{"$eq": []any{"$" + foreignIdField, "$$entityId"}},
				{"$eq": []any{"$config.isDefault", true}},
				{"$eq": []any{"$trash", false}},
			}},
		}
	case contentEnums.CartTypeBundle:
		fromCollection = "academyBundlePricings"
		foreignIdField = "bundleId"
		matchExpr = bson.M{
			"$expr": bson.M{"$and": []bson.M{
				{"$eq": []any{"$" + foreignIdField, "$$entityId"}},
				{"$eq": []any{"$active", true}},
				{"$eq": []any{"$trash", false}},
			}},
		}
	case contentEnums.CartTypeMaterial:
		fromCollection = "academyPrintedMaterials"
		foreignIdField = "_id"
		matchExpr = bson.M{
			"$expr": bson.M{"$and": []bson.M{
				{"$eq": []any{"$" + foreignIdField, "$$entityId"}},
				{"$eq": []any{"$trash", false}},
			}},
		}
	default:
		return nil, fmt.Errorf("invalid product type")
	}

	stages := []bson.M{
		{
			"$lookup": bson.M{
				"from": fromCollection,
				"let":  bson.M{"entityId": idField},
				"pipeline": []bson.M{
					{"$match": matchExpr},
					{"$limit": 1},
					{
						"$addFields": bson.M{
							"price": bson.M{
								"$divide": []any{"$price", conversionRateExpr},
							},
							"currency": "USD",
						},
					},
					// Special case for Coaching: we don't have qty here, so we return pricePerSession as price
					// if collection is CoachingPricings, map pricePerSession to price
				},
				"as": "pricing",
			},
		},
	}

	if pType == contentEnums.CartTypeCoaching {
		stages[0]["$lookup"].(bson.M)["pipeline"].([]bson.M)[2]["$addFields"].(bson.M)["price"] = bson.M{
			"$divide": []any{"$pricePerSession", conversionRateExpr},
		}
	}

	stages = append(stages, bson.M{
		"$addFields": bson.M{
			"pricing": bson.M{"$arrayElemAt": []interface{}{"$pricing", 0}},
		},
	})

	return stages, nil
}

func (s *pricingsvcs) GetPricesBatch(ctx context.Context, items []contentModels.CartItem) ([]models.PriceResult, error) {
	if len(items) == 0 {
		return []models.PriceResult{}, nil
	}

	// Group items by type to reduce DB calls
	typeGroup := make(map[contentEnums.CartType][]primitive.ObjectID)
	for _, item := range items {
		typeGroup[item.Type] = append(typeGroup[item.Type], item.RefId)
	}

	resultsMap := make(map[string]models.PriceResult)

	for pType, ids := range typeGroup {
		switch pType {
		case contentEnums.CartTypeCoaching:
			var pricings []contentModels.CoachingPricing
			pipeline := []bson.M{{"$match": bson.M{"coachingId": bson.M{"$in": ids}, "active": true, "trash": false}}}
			if err := s.coachingPricingRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
				return cur.All(ctx, &pricings)
			}); err == nil {
				for _, p := range pricings {
					resultsMap[p.CoachingId.Hex()] = models.PriceResult{RefId: p.CoachingId, Price: p.PricePerSession, Currency: p.Currency}
				}
			}
		case contentEnums.CartTypeCourse:
			var pricings []contentModels.CoursePricing
			pipeline := []bson.M{{"$match": bson.M{"courseId": bson.M{"$in": ids}, "config.isDefault": true, "trash": false}}}
			if err := s.coursePricingRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
				if err := cur.All(ctx, &pricings); err != nil {
					return err
				}
				for _, p := range pricings {
					resultsMap[p.CourseId.Hex()] = models.PriceResult{RefId: p.CourseId, Price: p.Price, Currency: p.Currency}
				}
				return nil
			}); err != nil {
				slog.Error("Pricing DEBUG: Aggregate error:", "Err", err)
			}
		case contentEnums.CartTypeBundle:
			var pricings []contentModels.BundlePricing
			pipeline := []bson.M{{"$match": bson.M{"bundleId": bson.M{"$in": ids}, "active": true, "trash": false}}}
			if err := s.bundlePricingRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
				return cur.All(ctx, &pricings)
			}); err == nil {
				for _, p := range pricings {
					resultsMap[p.BundleId.Hex()] = models.PriceResult{RefId: p.BundleId, Price: p.Price, Currency: p.Currency}
				}
			}
		case contentEnums.CartTypeMaterial:
			var materials []contentModels.PrintedMaterial
			pipeline := []bson.M{{"$match": bson.M{"_id": bson.M{"$in": ids}, "trash": false}}}
			if err := s.materialRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
				return cur.All(ctx, &materials)
			}); err == nil {
				for _, p := range materials {
					resultsMap[p.Id.Hex()] = models.PriceResult{RefId: p.Id, Price: p.Price, Currency: p.Currency}
				}
			}
		}
	}

	finalResults := make([]models.PriceResult, 0, len(items))
	for _, item := range items {
		res, ok := resultsMap[item.RefId.Hex()]
		if !ok {
			finalResults = append(finalResults, models.PriceResult{RefId: item.RefId, Error: "pricing not found"})
			continue
		}

		// Calculate coaching total if needed (requires item data)
		if item.Type == contentEnums.CartTypeCoaching {
			cs, err := util.Converter[*contentModels.CoachingSubscribe](item.Data)
			if err == nil && cs != nil {
				res.Price = res.Price * float64(cs.NumOfSessions)
			}
		}

		// Convert to USD
		if res.Currency != "USD" && res.Currency != "" {
			converted, err := s.exchangeSvcs.Convert(ctx, res.Price, res.Currency, "USD")
			if err == nil {
				res.Price = converted
				res.Currency = "USD"
			} else {
				res.Error = fmt.Sprintf("conversion failed: %v", err)
			}
		} else {
			res.Currency = "USD"
		}

		finalResults = append(finalResults, res)
	}

	return finalResults, nil
}
