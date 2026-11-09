package contact

import (
	"context"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/util"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// UserBasicInfo contains the minimal info needed for display
type UserBasicInfo struct {
	Id    primitive.ObjectID
	Name  transl.Localizable[string]
	Email string
	Type  string // "teacher", "supervisor", "user", "parent", "student"
}

// mergeLocalizableNames combines firstName and lastName Localizable maps into a full name
// Preserves all language keys from both maps
func mergeLocalizableNames(firstName, lastName interface{}) transl.Localizable[string] {
	result := make(transl.Localizable[string])
	
	// Helper to extract Localizable map from BSON value
	extractMap := func(val interface{}) map[string]string {
		if m, ok := val.(bson.M); ok {
			result := make(map[string]string)
			for k, v := range m {
				if str, ok := v.(string); ok {
					result[k] = str
				}
			}
			return result
		}
		// Fallback for legacy string data
		if str, ok := val.(string); ok && str != "" {
			return map[string]string{"en": str}
		}
		return nil
	}
	
	fnMap := extractMap(firstName)
	lnMap := extractMap(lastName)
	
	// Collect all language keys from both maps
	allLangs := make(map[string]bool)
	for k := range fnMap {
		allLangs[k] = true
	}
	for k := range lnMap {
		allLangs[k] = true
	}
	
	// Merge for each language
	for lang := range allLangs {
		fn := fnMap[lang]
		ln := lnMap[lang]
		if fn == "" && ln == "" {
			continue
		}
		// Combine with space, trim if needed
		fullName := fn
		if fn != "" && ln != "" {
			fullName = fn + " " + ln
		} else if ln != "" {
			fullName = ln
		}
		if fullName != "" {
			result[lang] = fullName
		}
	}
	
	// If no languages found, return empty map (will be handled by Localizable marshalling)
	if len(result) == 0 {
		result["en"] = ""
	}
	
	return result
}

type UserLookupSvc interface {
	// GetUsersBasicInfo efficiently fetches names/emails for a list of IDs from multiple collections
	GetUsersBasicInfo(ctx context.Context, userIds []primitive.ObjectID) (map[primitive.ObjectID]UserBasicInfo, error)
}

type userLookupSvc struct {
	dbClient *mongo.Client
}

func NewUserLookupSvc(i *do.Injector) (UserLookupSvc, error) {
	return &userLookupSvc{
		dbClient: do.MustInvoke[*mongo.Client](i),
	}, nil
}

func (s *userLookupSvc) GetUsersBasicInfo(ctx context.Context, userIds []primitive.ObjectID) (map[primitive.ObjectID]UserBasicInfo, error) {
	result := make(map[primitive.ObjectID]UserBasicInfo)
	missing := make(map[primitive.ObjectID]bool)

	for _, id := range userIds {
		missing[id] = true
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}
	db := s.dbClient.Database(cfg.Db)

	// Helper to get missing IDs as slice
	getMissingIds := func() []primitive.ObjectID {
		ids := make([]primitive.ObjectID, 0, len(missing))
		for id := range missing {
			ids = append(ids, id)
		}
		return ids
	}

	// 1. Try Teachers Collection
	if ids := getMissingIds(); len(ids) > 0 {
		cursor, err := db.Collection("academyTeachers").Find(ctx, bson.M{
			"_id": bson.M{"$in": ids},
		})
		if err == nil {
			var teachers []bson.M
			if err = cursor.All(ctx, &teachers); err == nil {
				for _, t := range teachers {
					if id, ok := t["_id"].(primitive.ObjectID); ok {
						// Merge firstName and lastName preserving Localizable structure
						name := mergeLocalizableNames(t["firstName"], t["lastName"])

						email := ""
						if acc, ok := t["accountSettings"].(bson.M); ok {
							email, _ = acc["email"].(string)
						}

						result[id] = UserBasicInfo{
							Id:    id,
							Name:  name,
							Email: email,
							Type:  "teacher",
						}
						delete(missing, id)
					}
				}
			}
			cursor.Close(ctx)
		}
	}

	// 2. Try Supervisors Collection
	if ids := getMissingIds(); len(ids) > 0 {
		cursor, err := db.Collection("academySupervisors").Find(ctx, bson.M{
			"_id": bson.M{"$in": ids},
		})
		if err == nil {
			var supervisors []bson.M
			if err = cursor.All(ctx, &supervisors); err == nil {
				for _, sup := range supervisors {
					if id, ok := sup["_id"].(primitive.ObjectID); ok {
						// Merge firstName and lastName preserving Localizable structure
						name := mergeLocalizableNames(sup["firstName"], sup["lastName"])

						email := ""
						if acc, ok := sup["accountSettings"].(bson.M); ok {
							email, _ = acc["email"].(string)
						}

						result[id] = UserBasicInfo{
							Id:    id,
							Name:  name,
							Email: email,
							Type:  "supervisor",
						}
						delete(missing, id)
					}
				}
			}
			cursor.Close(ctx)
		}
	}

	// 3. Try Parents Collection
	if ids := getMissingIds(); len(ids) > 0 {
		cursor, err := db.Collection("academyParents").Find(ctx, bson.M{
			"_id": bson.M{"$in": ids},
		})
		if err == nil {
			var parents []bson.M
			if err = cursor.All(ctx, &parents); err == nil {
				for _, p := range parents {
					if id, ok := p["_id"].(primitive.ObjectID); ok {
						var firstName, lastName interface{}
						if genInfo, ok := p["parentGeneralInfo"].(bson.M); ok {
							firstName = genInfo["firstName"]
							lastName = genInfo["lastName"]
						}
						// Merge firstName and lastName preserving Localizable structure
						name := mergeLocalizableNames(firstName, lastName)

						email := ""
						if acc, ok := p["accountParentSetting"].(bson.M); ok {
							email, _ = acc["email"].(string)
						}

						result[id] = UserBasicInfo{
							Id:    id,
							Name:  name,
							Email: email,
							Type:  "parent",
						}
						delete(missing, id)
					}
				}
			}
			cursor.Close(ctx)
		}
	}

	// 4. Try Students Collection
	if ids := getMissingIds(); len(ids) > 0 {
		cursor, err := db.Collection("academyStudents").Find(ctx, bson.M{
			"_id": bson.M{"$in": ids},
		})
		if err == nil {
			var students []bson.M
			if err = cursor.All(ctx, &students); err == nil {
				for _, s := range students {
					if id, ok := s["_id"].(primitive.ObjectID); ok {
						var firstName, lastName interface{}
						if genInfo, ok := s["generalInfo"].(bson.M); ok {
							firstName = genInfo["firstName"]
							lastName = genInfo["lastName"]
						}
						// Merge firstName and lastName preserving Localizable structure
						name := mergeLocalizableNames(firstName, lastName)

						email := ""
						if acc, ok := s["accountStudentSetting"].(bson.M); ok {
							email, _ = acc["email"].(string)
						}

						result[id] = UserBasicInfo{
							Id:    id,
							Name:  name,
							Email: email,
							Type:  "student",
						}
						delete(missing, id)
					}
				}
			}
			cursor.Close(ctx)
		}
	}

	// 5. Try Users Collection (Fallback)
	if ids := getMissingIds(); len(ids) > 0 {
		cursor, err := db.Collection("users").Find(ctx, bson.M{
			"_id": bson.M{"$in": ids},
		})
		if err == nil {
			var users []bson.M
			if err = cursor.All(ctx, &users); err == nil {
				for _, u := range users {
					if id, ok := u["_id"].(primitive.ObjectID); ok {
						// Merge firstName and lastName preserving Localizable structure
						name := mergeLocalizableNames(u["firstName"], u["lastName"])
						email, _ := u["email"].(string)

						result[id] = UserBasicInfo{
							Id:    id,
							Name:  name,
							Email: email,
							Type:  "user",
						}
						delete(missing, id)
					}
				}
			}
			cursor.Close(ctx)
		}
	}

	return result, nil
}

