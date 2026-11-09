package helpers

import (
	"fmt"
	"net/http"
)

type APIError struct {
	StatusCode int `json:"statusCode"`
	Message    any `json:"message"`
}

func (e APIError) Error() string {
	return fmt.Sprintf("api error: %d", e.StatusCode)
}

func NewApiError(statusCode int, err error) APIError {
	return APIError{
		StatusCode: statusCode,
		Message:    err.Error(),
	}
}

func InvalidRequestData(errors map[string]string) APIError {
	return APIError{
		StatusCode: http.StatusUnprocessableEntity,
		Message:    errors,
	}
}

func InvalidJSON() APIError {
	return NewApiError(
		http.StatusBadRequest,
		fmt.Errorf("invalid json request data"),
	)
}

func BadRequest(message string) APIError {
	return NewApiError(
		http.StatusBadRequest,
		fmt.Errorf("%s", message),
	)
}

func InternalServerError(message string) APIError {
	return NewApiError(
		http.StatusInternalServerError,
		fmt.Errorf("%s", message),
	)
}

func InvalidJsonCustomMessage(message string) APIError {
	return NewApiError(
		http.StatusBadRequest,
		fmt.Errorf("%s", message),
	)
}

func DataBaseInsertError() APIError {
	return NewApiError(
		http.StatusInternalServerError,
		fmt.Errorf("database insert failed"),
	)
}

func DataBaseUpdateError() APIError {
	return NewApiError(
		http.StatusInternalServerError,
		fmt.Errorf("database update failed"),
	)
}

func DataBaseDeleteError() APIError {
	return NewApiError(
		http.StatusInternalServerError,
		fmt.Errorf("database delete failed"),
	)
}

func NotFoundError(message string) APIError {
	return NewApiError(
		http.StatusNotFound,
		fmt.Errorf("%s", message),
	)
}

func Unauthorized(message string) APIError {
	return NewApiError(
		http.StatusUnauthorized,
		fmt.Errorf("%s", message),
	)
}

func Forbidden(message string) APIError {
	return NewApiError(
		http.StatusForbidden,
		fmt.Errorf("%s", message),
	)
}

func InvalidObjectId() APIError {
	return NewApiError(
		http.StatusBadRequest,
		fmt.Errorf("invalid Object Id format"),
	)
}

func InvalidObjectIdCustomMessage(message string) APIError {
	return NewApiError(
		http.StatusBadRequest,
		fmt.Errorf("%s", message),
	)
}

func InvalidPagination() APIError {
	return NewApiError(http.StatusBadRequest, fmt.Errorf("invalid Object Id format"))
}

func ValidationErrors(errors map[string]string) APIError {
	return APIError{
		StatusCode: http.StatusBadRequest,
		Message:    errors,
	}
}

func BulkValidationErrors(errors map[string][]string) APIError {
	return APIError{
		StatusCode: http.StatusBadRequest,
		Message:    errors,
	}
}
