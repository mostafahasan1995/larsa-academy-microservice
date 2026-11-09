package helpers

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

func GenericValidation(v *validator.Validate, dto interface{}) error {
	if err := v.Struct(dto); err != nil {
		errors := make(map[string]string)

		for _, err := range err.(validator.ValidationErrors) {
			fieldName := err.Field()

			fieldValue := err.Value()
			errorTag := err.Tag()
			nameSpace := err.Namespace()

			errorMessage := fmt.Sprintf("Validation failed on field '%s' with value '%v', failed tag '%s'.", fieldName, fieldValue, errorTag)

			errors[nameSpace] = errorMessage
		}
		return ValidationErrors(errors)
	}
	return nil
}
