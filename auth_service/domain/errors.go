package domain

import "errors"

var ErrUserNotFound = errors.New("user not found")
var ErrUserTagExists = errors.New("user tag already exists")
var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrValidation = errors.New("validation error")

const (
	ValidationRequired          = "required"
	ValidationTooShort          = "too_short"
	ValidationTooLong           = "too_long"
	ValidationUnderage          = "underage"
	ValidationTooOld            = "too_old"
	ValidationFutureDate        = "future_date"
	ValidationMustContainLetter = "must_contain_letter"
)

type ValidationErrors struct {
	Fields map[string]string
}

func (e *ValidationErrors) Error() string {
	return "validation error"
}
