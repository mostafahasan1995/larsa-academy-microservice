package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"larsa-academy-microservice/pkg/util"
	"log/slog"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrNotFound = errors.New("item not found")

type RoleList struct {
	Roles []Role `json:"roles"`
}

type Role struct {
	Id   primitive.ObjectID `json:"_id"`
	Name string             `json:"name"`
}

type RolesGw struct{}

func NewRolesGw(i *do.Injector) (*RolesGw, error) {
	return &RolesGw{}, nil
}

func (rgw *RolesGw) GetRoleByName(ctx context.Context, name string) (*Role, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("roles?query={\"name\":{\"$eq\":\"%s\"}}", name)
	reqOp := &common.RequestParams{
		Service: "users",
		Path:    path,
		Method:  "GET",
		Data:    map[string]interface{}{},
		Header:  cfg.Hp,
	}

	res, err := common.CallService(reqOp)
	if err != nil {
		slog.Error("Failed to call users service for role lookup", "error", err, "roleName", name)
		return nil, err
	}

	if res.StatusCode != 200 {
		slog.Error("Unexpected status code from users service for role lookup", "statusCode", res.StatusCode, "roleName", name)
		return nil, fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	var result RoleList
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		slog.Error("Failed to decode role response from users service", "error", err, "roleName", name)
		return nil, err
	}

	if len(result.Roles) == 0 {
		return nil, ErrNotFound
	}

	return &result.Roles[0], nil
}

func (rgw *RolesGw) GetRolesByIds(ctx context.Context, roleIds []primitive.ObjectID) ([]Role, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Convert ObjectIDs to strings for the query
	var idStrings []string
	for _, id := range roleIds {
		idStrings = append(idStrings, id.Hex())
	}

	// Create query with $in operator to match multiple IDs
	query := fmt.Sprintf("roles?query={\"_id\":{\"$in\":[%s]}}", formatIdsForQuery(idStrings))

	reqOp := &common.RequestParams{
		Service: "users",
		Path:    query,
		Method:  "GET",
		Data:    map[string]interface{}{},
		Header:  cfg.Hp,
	}

	res, err := common.CallService(reqOp)
	if err != nil {
		return nil, err
	}

	var result RoleList
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Roles) == 0 {
		return nil, ErrNotFound
	}

	return result.Roles, nil
}

func formatIdsForQuery(ids []string) string {
	if len(ids) == 0 {
		return ""
	}

	result := ""
	for i, id := range ids {
		if i > 0 {
			result += ","
		}
		result += fmt.Sprintf("\"%s\"", id)
	}
	return result
}
