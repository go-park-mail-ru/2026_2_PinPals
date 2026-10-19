.PHONY: help build build-auth build-pin run tests fmt up down cover

help:
	@echo "Available commands:"
	@echo "  make build     - build backend"
	@echo "  make build-auth - build auth service"
	@echo "  make build-pin - build pin service"
	@echo "  make run       - start application with docker compose"
	@echo "  make tests     - run tests"
	@echo "  make fmt       - format Go code"
	@echo "  make down      - stop docker compose"

build:
	go build ./cmd/...

build-auth:
	go build ./cmd/auth_service

build-pin:
	go build ./cmd/pin_service

run:
	docker compose up --build

tests:
	go test ./...

fmt:
	gofmt -w ./cmd ./pkg ./auth_service ./pin_service

down:
	docker compose down

cover:
	@go test -coverprofile=coverage.out -coverpkg=./... ./... > /dev/null
	@go tool cover -func=coverage.out
