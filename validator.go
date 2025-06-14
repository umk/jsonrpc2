package jsonrpc2

import (
	"errors"

	"github.com/go-playground/validator/v10"
)

var Val = validator.New(validator.WithRequiredStructEnabled())

func validateIfStruct(v any) error {
	if err := Val.Struct(v); err != nil {
		var valErr *validator.InvalidValidationError
		if !errors.As(err, &valErr) {
			// Validation failed - return the validation errors
			return err
		}
		// Not a struct or other programming error - ignore
	}

	// Validation passed
	return nil
}
