package contact

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"larsa-academy-microservice/pkg/gateway"
	gatewaymodels "larsa-academy-microservice/pkg/gateway/models"
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/services/contact/models"
	"larsa-academy-microservice/pkg/util"
	"log/slog"
	"time"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"github.com/go-playground/validator/v10"
	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrUserNotFound = errors.New("user not found")
var ErrDupliateEmail = errors.New("duplicated email")
var ErrUnKnowen = errors.New("unknowen error")
var ErrUnauthorized = errors.New("unauthorized")
var ErrForbidden = errors.New("forbidden")

type CredSvcs interface {
	Add(ctx context.Context, data models.CredData, roleName string) (*models.AddUserResult, error)
	Register(ctx context.Context, firstName, lastName, email, password, passwordConfirmation, phone string, phoneOTP *string, picture *string) (*models.RegisterUserResult, error)
	Update(ctx context.Context, id string, data models.CredData) error
	UpdateRaw(ctx context.Context, id string, data map[string]any) error
	Delete(ctx context.Context, id string) error
}

func (c *credSvcs) UpdateRaw(ctx context.Context, id string, data map[string]any) error {
	serviceToken, _ := common.GetServiceToken("academyservice", &common.HeaderParams{})
	resp, err := c.gateway.Request(ctx, "users", "users/"+id, "PATCH", serviceToken, data)
	if err != nil {
		return errors.New("error updating user")
	}

	if resp.StatusCode != 200 {
		// Try to parse error message from response body
		var bodyBytes []byte
		if resp.Body != nil {
			bodyBytes, _ = io.ReadAll(resp.Body)
		}

		slog.Warn("Users service update failed",
			"statusCode", resp.StatusCode,
			"body", string(bodyBytes),
			"userId", id)

		if len(bodyBytes) > 0 {
			var errorResponse struct {
				Msg string `json:"msg"`
			}
			if json.Unmarshal(bodyBytes, &errorResponse) == nil && errorResponse.Msg != "" {
				return errors.New(errorResponse.Msg)
			}
		}

		// Handle specific status codes
		switch resp.StatusCode {
		case 409:
			return ErrDupliateEmail
		case 401:
			return ErrUnauthorized
		case 403:
			return ErrForbidden
		default:
			return errors.New("error updating user")
		}
	}

	return nil
}

type credSvcs struct {
	gateway   gateway.Gateway
	usersgw   *gateway.UsersGw
	rolesgw   *gateway.RolesGw
	validator *validator.Validate
}

func NewCredSvcs(i *do.Injector) (CredSvcs, error) {
	return &credSvcs{
		gateway:   do.MustInvoke[gateway.Gateway](i),
		usersgw:   do.MustInvoke[*gateway.UsersGw](i),
		rolesgw:   do.MustInvoke[*gateway.RolesGw](i),
		validator: do.MustInvoke[*validator.Validate](i),
	}, nil
}

func (c *credSvcs) Add(ctx context.Context, data models.CredData, roleName string) (*models.AddUserResult, error) {
	userData := data.GetUserData()

	rules := map[string]interface{}{
		"firstName":            "required",
		"lastName":             "required",
		"email":                "required,email",
		"password":             "required,min=6",
		"passwordConfirmation": "required,min=6",
		"picture":              "omitempty",
	}

	errs := c.validator.ValidateMap(userData, rules)
	if len(errs) > 0 {
		return nil, errors.New("invalid user data")
	}

	// Get role by name using rolesgw
	var roles []primitive.ObjectID
	if roleName != "" {
		role, err := c.rolesgw.GetRoleByName(ctx, roleName)
		if err != nil {
			slog.Error("Failed to get role by name", "error", err, "roleName", roleName, "email", userData["email"])
			return nil, fmt.Errorf("failed to get role by name '%s': %w", roleName, err)
		}
		roles = []primitive.ObjectID{role.Id}
	}

	// Convert userData map to PostUserData struct
	postUserData := &gatewaymodels.PostUserData{
		FirstName:    userData["firstName"].(string),
		LastName:     userData["lastName"].(string),
		Email:        userData["email"].(string),
		Password:     userData["password"].(string),
		Roles:        roles,
		Capabilities: []primitive.ObjectID{}, // Default empty capabilities
	}

	// Add phone if provided
	if phone, ok := userData["phone"].(string); ok && phone != "" {
		postUserData.Phone = phone
	}

	// Add picture if provided
	if picture, ok := userData["picture"].(string); ok && picture != "" {
		postUserData.Picture = picture
	}

	// Use AddUser from usersgw (admin endpoint allows phone without OTP)
	userId, err := c.usersgw.AddUser(ctx, postUserData)
	if err != nil {
		return nil, err
	}

	result := &models.AddUserResult{
		UserData: models.UserData{
			Id: userId,
		},
		DeleteOnErr: func() error {
			resp, err := c.gateway.Request(ctx, "users", "users/"+userId.Hex(), "DELETE", "", map[string]any{})
			if err != nil {
				slog.Error("Failed to cleanup user after error", "error", err, "userId", userId.Hex())
				return errors.New("error deleting user")
			}
			if resp != nil && resp.StatusCode != 200 {
				slog.Error("Failed to cleanup user after error", "statusCode", resp.StatusCode, "userId", userId.Hex())
				return errors.New("error deleting user")
			}
			return nil
		},
	}

	return result, nil
}

func (c *credSvcs) Register(ctx context.Context, firstName, lastName, email, password, passwordConfirmation, phone string, phoneOTP *string, picture *string) (*models.RegisterUserResult, error) {
	// Add timeout for external service calls
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Get service token for user registration
	serviceToken, err := common.GetServiceToken("academyservice", &common.HeaderParams{})
	if err != nil {
		slog.Error("Failed to get service token for user registration", "error", err, "email", email)
		return nil, helpers.InternalServerError("failed to get service token")
	}

	// Prepare user data for registration
	userData := map[string]any{
		"firstName":            firstName,
		"lastName":             lastName,
		"email":                email,
		"password":             password,
		"passwordConfirmation": passwordConfirmation,
		"phone":                phone,
	}

	// Only include phoneOTP if provided
	if phoneOTP != nil && *phoneOTP != "" {
		userData["phoneOTP"] = *phoneOTP
	}

	// Only include picture if provided
	if picture != nil && *picture != "" {
		userData["picture"] = *picture
	}

	// Register user in users service
	resp, err := c.gateway.Request(ctx, "users", "users/register", "POST", "", userData)
	if err != nil {
		slog.Error("Error calling users service for registration", "error", err, "email", email)
		return nil, helpers.InternalServerError("error registering user")
	}

	// Buffer response body to allow multiple reads
	var bodyBytes []byte
	if resp.Body != nil {
		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("Error reading response body", "error", err, "email", email)
			return nil, helpers.InternalServerError("error reading response")
		}
		// Restore body for potential re-reading
		resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	if resp.StatusCode != 200 {
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
			switch resp.StatusCode {
			case 400:
				errorMsg = "invalid request data"
			case 401:
				errorMsg = "unauthorized"
			case 403:
				errorMsg = "forbidden"
			case 409:
				errorMsg = "email or phone already registered"
			case 422:
				errorMsg = "validation error"
			default:
				errorMsg = "error registering user"
			}
		}

		slog.Warn("User registration failed in users service", "statusCode", resp.StatusCode, "error", errorMsg, "email", email)
		return nil, helpers.NewApiError(resp.StatusCode, fmt.Errorf("%s", errorMsg))
	}

	// Parse user response (users service returns user object directly)
	type UserResponse struct {
		Id primitive.ObjectID `json:"_id"`
	}

	var userResp UserResponse
	if errDec := json.Unmarshal(bodyBytes, &userResp); errDec != nil {
		return nil, helpers.InternalServerError("error parsing user response")
	}

	// Copy headers from users service response
	headers := make(map[string][]string)
	for key, values := range resp.Header {
		headers[key] = values
	}

	result := &models.RegisterUserResult{
		UserData: models.UserData{
			Id: userResp.Id,
		},
		UserResponse: bodyBytes,
		Headers:      headers,
		DeleteOnErr: func() error {
			delResp, err := c.gateway.Request(ctx, "users", "users/"+userResp.Id.Hex(), "DELETE", serviceToken, map[string]any{})
			if err != nil || delResp.StatusCode != 200 {
				slog.Error("Failed to cleanup user after registration failure", "error", err, "userId", userResp.Id.Hex(), "email", email)
				return errors.New("error deleting user")
			}
			return nil
		},
	}

	return result, nil
}

func (c *credSvcs) Update(ctx context.Context, id string, data models.CredData) error {
	user := data.GetUserData()

	rules := map[string]interface{}{
		"firstName": "required",
		"lastName":  "required",
		"email":     "required,email",
		"password":  "omitempty,min=6",
		"picture":   "omitempty",
	}

	errs := c.validator.ValidateMap(user, rules)
	if len(errs) > 0 {
		return errors.New("invalid user data")
	}

	return c.UpdateRaw(ctx, id, user)
}

func (c *credSvcs) Delete(ctx context.Context, id string) error {

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}
	newUserReqOp := &common.RequestParams{
		Service: "users",
		Path:    fmt.Sprintf("users/%s", id),
		Method:  "DELETE",
		Data:    map[string]interface{}{},
		Header:  cfg.Hp,
	}

	res, err := common.CallService(newUserReqOp)

	if err != nil {
		return errors.New("something went wrong")
	} else if res.StatusCode != 200 {

		switch res.StatusCode {
		case 409:
			return ErrDupliateEmail
		case 401:
			return ErrUnauthorized
		case 403:
			return ErrForbidden
		default:
			return err
		}

	}

	return nil

}
