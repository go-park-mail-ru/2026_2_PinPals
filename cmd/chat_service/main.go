// @title PinPals Chat API
// @version 1.0
// @description API личных чатов, истории сообщений и WebSocket-доставки.
// @BasePath /api/v1
// @securityDefinitions.apikey CookieAuth
// @in header
// @name Cookie
// @description Браузер автоматически отправляет cookie access_token. Для изменяющих состояние HTTP-запросов требуется X-CSRF-Token.
// @host <АДРЕС_ТВОЕГО_BACKEND>
// @schemes https
package main

import (
	"context"
	"os"

	"2026_2_PinPals/chat_service"
)

func main() {
	if err := chat_service.Run(context.Background()); err != nil {
		os.Exit(1)
	}
}
