// Package validation wraps go-playground/validator so handlers can
// validate decoded request bodies and get back the application's
// consistent field-error format instead of raw validator errors.
package validation

import (
	"errors"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/stan-ley-tech/medqueue/internal/apperr"
)

var validate = newValidator()

func newValidator() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	return v
}

// Struct validates s against its `validate` struct tags and returns a
// single *apperr.Error with one field message per failing rule.
func Struct(s any) error {
	err := validate.Struct(s)
	if err == nil {
		return nil
	}

	var invalidErr *validator.InvalidValidationError
	if errors.As(err, &invalidErr) {
		return apperr.Internal(err)
	}

	var validationErrs validator.ValidationErrors
	if !errors.As(err, &validationErrs) {
		return apperr.Internal(err)
	}

	fields := map[string]string{}
	for _, fe := range validationErrs {
		fields[jsonFieldName(fe)] = message(fe)
	}
	return apperr.Validation("request failed validation", fields)
}

func jsonFieldName(fe validator.FieldError) string {
	return strings.ToLower(fe.Field()[:1]) + fe.Field()[1:]
}

func message(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email address"
	case "min":
		return "must be at least " + fe.Param() + " characters"
	case "max":
		return "must be at most " + fe.Param() + " characters"
	case "gt":
		return "must be greater than " + fe.Param()
	case "gte":
		return "must be greater than or equal to " + fe.Param()
	case "oneof":
		return "must be one of: " + fe.Param()
	case "uuid":
		return "must be a valid UUID"
	case "e164":
		return "must be a valid phone number in E.164 format"
	default:
		return "is invalid"
	}
}
