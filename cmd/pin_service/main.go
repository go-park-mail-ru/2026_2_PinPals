// @title PinPals Pins and Desks API
// @version 1.0
// @description API пинов, загрузки изображений, досок и лайков.
// @BasePath /api/v1
// @securityDefinitions.apikey CookieAuth
// @in header
// @name Cookie
// @description Браузер автоматически отправляет cookie access_token. Для изменяющих состояние запросов требуется X-CSRF-Token.
package main

import (
	"context"
	"os"

	"2026_2_PinPals/pin_service"
)

func main() {
	if err := pin_service.Run(context.Background()); err != nil {
		os.Exit(1)
	}
}
