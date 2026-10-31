package domain

import "errors"

var ErrPinNotFound = errors.New("pin not found")
var ErrTagNotFound = errors.New("tag not found")
var ErrValidation = errors.New("validation error")
