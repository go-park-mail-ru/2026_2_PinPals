package domain

import "errors"

var ErrUserNotFound = errors.New("user not found")
var ErrUserTagExists = errors.New("user tag already exists")
var ErrValidation = errors.New("validation error")
var ErrSubscriptionNotFound = errors.New("subscription not found")
