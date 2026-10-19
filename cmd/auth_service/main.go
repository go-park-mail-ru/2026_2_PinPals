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
