.PHONY: fmt test build run up down

fmt:
	gofmt -w ./cmd ./internal

test:
	go test ./...

build:
	go build ./cmd/server

run:
	go run ./cmd/server

up:
	docker compose up --build

down:
	docker compose down
