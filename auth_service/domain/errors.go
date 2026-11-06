package domain

import "errors"

var ErrUserNotFound = errors.New("пользователь не найден")
var ErrUserTagExists = errors.New("пользователь с таким тегом уже существует")
var ErrInvalidCredentials = errors.New("неверный логин или пароль")
var ErrValidation = errors.New("ошибка проверки данных")

const (
	ValidationRequired          = "обязательное поле"
	ValidationTooShort          = "значение слишком короткое"
	ValidationTooLong           = "значение слишком длинное"
	ValidationUnderage          = "возраст меньше допустимого"
	ValidationTooOld            = "возраст превышает допустимый"
	ValidationFutureDate        = "дата не может быть в будущем"
	ValidationMustContainLetter = "пароль должен содержать хотя бы одну букву"
)

type ValidationErrors struct {
	Fields map[string]string
}

func (e *ValidationErrors) Error() string {
	return "ошибка проверки данных"
}
