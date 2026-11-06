package middleware

import "net/http"

// Access защищает endpoint проверкой JWT и добавляет user_id в контекст.
// TokenManager читает JWT из HttpOnly cookie access_token; Bearer оставлен для совместимости.
func Access(tokens *TokenManager) func(http.Handler) http.Handler {
	return tokens.Middleware
}

// OptionalAccess допускает анонимный запрос, но проверяет токен, если он передан.
func OptionalAccess(tokens *TokenManager) func(http.Handler) http.Handler {
	return tokens.OptionalMiddleware
}
