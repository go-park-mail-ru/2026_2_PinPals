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
