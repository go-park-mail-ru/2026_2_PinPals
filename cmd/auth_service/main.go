// @title PinPals Authentication API
// @version 1.0
// @description API регистрации, входа и выхода. JWT выдаётся в HttpOnly cookie access_token; CSRF-токен — в cookie csrf_token.
// @BasePath /api/v1
// @securityDefinitions.apikey CookieAuth
// @in header
// @name Cookie
// @description Браузер автоматически отправляет cookie access_token. Для изменяющих состояние запросов требуется X-CSRF-Token с тем же значением, что и cookie csrf_token.
package main

import (
	"context"
	"os"

	"2026_2_PinPals/auth_service"
)

func main() {
	if err := auth_service.Run(context.Background()); err != nil {
		os.Exit(1)
	}
}
