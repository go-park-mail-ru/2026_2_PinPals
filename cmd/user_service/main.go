// @title PinPals User Profile API
// @version 1.0
// @description API просмотра, изменения и удаления профиля текущего пользователя.
// @BasePath /api/v1
// @securityDefinitions.apikey CookieAuth
// @in header
// @name Cookie
// @description Браузер автоматически отправляет cookie access_token. Для изменяющих состояние запросов требуется X-CSRF-Token.
package main

import (
	"context"
	"os"

	"2026_2_PinPals/user_service"
)

func main() {
	if err := user_service.Run(context.Background()); err != nil {
		os.Exit(1)
	}
}
