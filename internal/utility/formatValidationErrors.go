package utility

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

// ValidationError holds a formatted error response
type ValidationError struct {
	Pointer string `json:"pointer"`
	Detail  string `json:"detail"`
}

// formatValidationErrors converts validator errors to readable messages
func FormatValidationErrors(err error) []ValidationError {
	var errors []ValidationError

	for _, e := range err.(validator.ValidationErrors) {
		var message string

		// Create human-readable messages based on the tag
		switch e.Tag() {
		case "required":
			message = fmt.Sprintf("%s is required", e.Field())
		case "email":
			message = fmt.Sprintf("%s must be a valid email", e.Field())
		case "min":
			message = fmt.Sprintf("%s must be at least %s characters", e.Field(), e.Param())
		case "max":
			message = fmt.Sprintf("%s must be at most %s characters", e.Field(), e.Param())
		case "oneof":
			message = fmt.Sprintf("%s must be one of: %s", e.Field(), e.Param())
		default:
			message = fmt.Sprintf("%s is invalid", e.Field())
		}

		errors = append(errors, ValidationError{
			Pointer: e.Field(),
			Detail:  message,
		})
	}

	return errors
}
