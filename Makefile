.PHONY: help build run tests fmt up down cover

help:
	@echo "Available commands:"
	@echo "  make build - build backend"
	@echo "  make run   - start application with docker compose"
	@echo "  make tests - run tests"
	@echo "  make fmt   - format Go code"
	@echo "  make down  - stop docker compose"

build:
	go build ./cmd/server

run:
	docker compose up --build

tests:
	go test ./...

fmt:
	gofmt -w ./cmd ./internal

down:
	docker compose down

cover:
	@go test -coverprofile=coverage.out -coverpkg=./... ./... > /dev/null
	@go tool cover -func=coverage.out
