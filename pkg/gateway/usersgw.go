package gateway

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/goccy/go-json"

	// "errors"

	"fmt"
	"larsa-academy-microservice/pkg/gateway/models"
	"larsa-academy-microservice/pkg/util"

	// "net/url"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrUserNotFound = errors.New("user not found")
var ErrDupliateEmail = errors.New("duplicated email")
var ErrUnKnowen = errors.New("unknowen error")
var ErrUnauthorized = errors.New("unauthorized")
var ErrForbidden = errors.New("forbidden")

type UsersGw struct{}

func NewUsersGw(i *do.Injector) (*UsersGw, error) {
	return &UsersGw{}, nil
}

// use service token when there is no access token
func (ugw *UsersGw) AddUser(ctx context.Context, user *models.PostUserData) (addedUserId primitive.ObjectID, errAdd error) {

	zeroId := primitive.NilObjectID

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return zeroId, err
	}

	userData := map[string]interface{}{
		"firstName":    user.FirstName,
		"lastName":     user.LastName,
		"email":        user.Email,
		"password":     user.Password,
		"roles":        user.Roles,
		"capabilities": user.Capabilities,
	}

	// Add phone if provided (admin endpoint allows phone without OTP)
	if user.Phone != "" {
		userData["phone"] = user.Phone
	}

	newUserReqOp := &common.RequestParams{
		Service: "users",
		Path:    "users/",
		Method:  "POST",
		Data:    userData,
		Header:  cfg.Hp,
	}

	res, err := common.CallService(newUserReqOp)

	if err != nil {
		return zeroId, errors.New("error adding user")
	} else if res.StatusCode != 200 {
		// Buffer response body to allow reading
		var bodyBytes []byte
		if res.Body != nil {
			bodyBytes, _ = io.ReadAll(res.Body)
			// Restore body for potential re-reading
			res.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		// Try to parse error message from response body
		errorMsg := ""
		if len(bodyBytes) > 0 {
			var errorResponse struct {
				Msg string `json:"msg"`
			}
			if json.Unmarshal(bodyBytes, &errorResponse) == nil && errorResponse.Msg != "" {
				errorMsg = errorResponse.Msg
			}
		}
		fmt.Println("SOMETHIGN WENT WRONG:", res.StatusCode, string(bodyBytes))

		// If no error message in response, use status code-based messages
		if errorMsg == "" {
			switch res.StatusCode {
			case 409:
				return zeroId, ErrDupliateEmail
			case 401:
				return zeroId, ErrUnauthorized
			case 403:
				return zeroId, ErrForbidden
			default:
				return zeroId, errors.New("error adding user")
			}
		}

		// Return error with parsed message
		return zeroId, errors.New(errorMsg)
	}

	type TempUser struct {
		Id primitive.ObjectID `json:"_id"`
	}

	type AddedUser struct {
		User TempUser `json:"user"`
	}

	var data AddedUser
	if errDec := json.NewDecoder(res.Body).Decode(&data); errDec != nil {
		return zeroId, errDec
	}

	return data.User.Id, nil
}

func (ugw *UsersGw) UpdateUser(ctx context.Context, userId string, user *models.PostUserData) (UpdatedUserId primitive.ObjectID, errUpdate error) {

	zeroId := primitive.NilObjectID

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return zeroId, err
	}

	userData := map[string]interface{}{
		"firstName":    user.FirstName,
		"lastName":     user.LastName,
		"email":        user.Email,
		"password":     user.Password,
		"roles":        user.Roles,
		"capabilities": user.Capabilities,
	}

	// Add phone if provided
	if user.Phone != "" {
		userData["phone"] = user.Phone
	}

	updateUserReqOp := &common.RequestParams{
		Service: "users",
		Path:    fmt.Sprintf("users/%s", userId),
		Method:  "PATCH",
		Data:    userData,
		Header:  cfg.Hp,
	}

	res, err := common.CallService(updateUserReqOp)

	if err != nil {
		return zeroId, errors.New("error updating user")
	} else if res.StatusCode != 200 {
		// Buffer response body to allow reading
		var bodyBytes []byte
		if res.Body != nil {
			bodyBytes, _ = io.ReadAll(res.Body)
			// Restore body for potential re-reading
			res.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		// Try to parse error message from response body
		errorMsg := ""
		if len(bodyBytes) > 0 {
			var errorResponse struct {
				Msg string `json:"msg"`
			}
			if json.Unmarshal(bodyBytes, &errorResponse) == nil && errorResponse.Msg != "" {
				errorMsg = errorResponse.Msg
			}
		}

		// If no error message in response, use status code-based messages
		if errorMsg == "" {
			switch res.StatusCode {
			case 409:
				return zeroId, ErrDupliateEmail
			case 401:
				return zeroId, ErrUnauthorized
			case 403:
				return zeroId, ErrForbidden
			default:
				return zeroId, errors.New("error updating user")
			}
		}

		// Return error with parsed message
		return zeroId, errors.New(errorMsg)
	}

	type TempUser struct {
		Id primitive.ObjectID `json:"_id"`
	}

	type UpdatedUser struct {
		User TempUser `json:"user"`
	}

	var data UpdatedUser
	if errDec := json.NewDecoder(res.Body).Decode(&data); errDec != nil {
		return zeroId, errDec
	}

	return data.User.Id, nil
}

func (ugw *UsersGw) GetUserById(ctx context.Context, id string) (*common.User, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	serviceToken, err := common.GetServiceToken("tourismservice", &common.HeaderParams{})
	hp := common.HeaderParams{
		Client:       cfg.Db,
		ServiceToken: serviceToken,
	}
	if err != nil {
		return nil, err
	}
	reqOp := &common.RequestParams{
		Service: "users",
		Path:    fmt.Sprintf("users/all?query={\"_id\":\"%s\"}", id),
		Method:  "GET",
		Data:    map[string]interface{}{},
		Header:  &hp,
	}

	var result struct {
		Users []*common.User
	}
	res, err := common.CallService(reqOp)
	if err != nil {
		return nil, err
	}
	err = json.NewDecoder(res.Body).Decode(&result)
	if len(result.Users) > 0 {
		return result.Users[0], err
	}
	return nil, err
}
