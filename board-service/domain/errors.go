package domain

import "errors"

var ErrDeskNotFound = errors.New("desk not found")
var ErrPinNotFound = errors.New("pin not found")
var ErrValidation = errors.New("validation error")
