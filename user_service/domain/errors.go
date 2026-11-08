package domain

import "errors"

var ErrUserNotFound = errors.New("user not found")
var ErrUserTagExists = errors.New("user tag already exists")
var ErrValidation = errors.New("validation error")
