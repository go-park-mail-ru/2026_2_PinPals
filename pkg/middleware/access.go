package middleware

import "net/http"

// Access защищает endpoint проверкой JWT и добавляет user_id в контекст.
// TokenManager читает токен из Authorization: Bearer; cookie поддерживается отдельной
// задачей перехода на cookie-аутентификацию.
func Access(tokens *TokenManager) func(http.Handler) http.Handler {
	return tokens.Middleware
}
