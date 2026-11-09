package support

import (
	"testing"

	"larsa-academy-microservice/pkg/query"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Complaint filter columns must match BSON field names in the Complaint model:
// status, assignTo, complaintType, relatedStudent, complaintUserRole,
// summary, description, createdAt, updatedAt, createdBy, updatedBy
var complaintFilterWhitelist = map[string]bool{
	"status":           true,
	"assignTo":         true,
	"complaintType":    true,
	"relatedStudent":   true,
	"complaintUserRole": true,
	"summary":          true,
	"description":      true,
	"createdAt":        true,
	"updatedAt":       true,
	"createdBy":        true,
	"updatedBy":        true,
}

func TestComplaintFilters_StatusFilter(t *testing.T) {
	conditions := &query.Conditions{
		Columns: []query.Column{
			{Name: "status", Exp: query.Eq, Value: "open"},
		},
	}
	require.NoError(t, conditions.CheckValid())

	filter, err := conditions.ConvertToMongo(query.WithWhitelistNames(complaintFilterWhitelist))
	require.NoError(t, err)

	assert.Equal(t, bson.M{"status": "open"}, filter)
}

func TestComplaintFilters_AssignToObjectId(t *testing.T) {
	assignToId := primitive.NewObjectID()
	conditions := &query.Conditions{
		Columns: []query.Column{
			{Name: "assignTo", Exp: query.Eq, Value: assignToId.Hex()},
		},
	}
	require.NoError(t, conditions.CheckValid())

	filter, err := conditions.ConvertToMongo(query.WithWhitelistNames(complaintFilterWhitelist))
	require.NoError(t, err)

	assert.Equal(t, assignToId, filter["assignTo"])
}

func TestComplaintFilters_ComplaintTypeObjectId(t *testing.T) {
	complaintTypeId := primitive.NewObjectID()
	conditions := &query.Conditions{
		Columns: []query.Column{
			{Name: "complaintType", Exp: query.Eq, Value: complaintTypeId.Hex()},
		},
	}
	require.NoError(t, conditions.CheckValid())

	filter, err := conditions.ConvertToMongo(query.WithWhitelistNames(complaintFilterWhitelist))
	require.NoError(t, err)

	assert.Equal(t, complaintTypeId, filter["complaintType"])
}

func TestComplaintFilters_RelatedStudentObjectId(t *testing.T) {
	studentId := primitive.NewObjectID()
	conditions := &query.Conditions{
		Columns: []query.Column{
			{Name: "relatedStudent", Exp: query.Eq, Value: studentId.Hex()},
		},
	}
	require.NoError(t, conditions.CheckValid())

	filter, err := conditions.ConvertToMongo(query.WithWhitelistNames(complaintFilterWhitelist))
	require.NoError(t, err)

	assert.Equal(t, studentId, filter["relatedStudent"])
}

func TestComplaintFilters_MultipleFiltersAnd(t *testing.T) {
	assignToId := primitive.NewObjectID()
	conditions := &query.Conditions{
		Columns: []query.Column{
			{Name: "status", Exp: query.Eq, Value: "open", Logic: query.AND},
			{Name: "assignTo", Exp: query.Eq, Value: assignToId.Hex()},
		},
	}
	require.NoError(t, conditions.CheckValid())

	filter, err := conditions.ConvertToMongo(query.WithWhitelistNames(complaintFilterWhitelist))
	require.NoError(t, err)

	andFilters, ok := filter["$and"].([]bson.M)
	require.True(t, ok)
	require.Len(t, andFilters, 2)
	assert.Equal(t, "open", andFilters[0]["status"])
	assert.Equal(t, assignToId, andFilters[1]["assignTo"])
}

func TestComplaintFilters_EmptyColumns_ReturnsEmptyFilter(t *testing.T) {
	conditions := &query.Conditions{
		Columns: []query.Column{},
	}
	require.NoError(t, conditions.CheckValid())

	filter, err := conditions.ConvertToMongo()
	require.NoError(t, err)

	assert.Equal(t, bson.M{}, filter)
}

func TestComplaintFilters_InvalidColumnName_RejectedByWhitelist(t *testing.T) {
	conditions := &query.Conditions{
		Columns: []query.Column{
			{Name: "invalidField", Exp: query.Eq, Value: "x"},
		},
	}
	require.NoError(t, conditions.CheckValid())

	_, err := conditions.ConvertToMongo(query.WithWhitelistNames(complaintFilterWhitelist))
	require.Error(t, err)
}
