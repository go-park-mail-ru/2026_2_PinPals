.PHONY: help build build-auth build-pin build-user run tests fmt up down cover generate-mocks swagger swagger-auth swagger-pin swagger-user

help:
	@echo "Available commands:"
	@echo "  make build          - build backend services"
	@echo "  make build-auth     - build auth service"
	@echo "  make build-pin      - build pin service"
	@echo "  make build-user     - build user service"
	@echo "  make run            - start application with docker compose"
	@echo "  make tests          - run tests"
	@echo "  make cover          - show coverage, excluding generated mocks; fail at <= 60%"
	@echo "  make generate-mocks - regenerate gomock mocks"
	@echo "  make swagger        - regenerate OpenAPI docs (run after all three branches are merged)"
	@echo "  make fmt            - format Go code"
	@echo "  make down           - stop docker compose"

build:
	go build ./cmd/...

build-auth:
	go build ./cmd/auth_service

build-pin:
	go build ./cmd/pin_service

build-user:
	go build ./cmd/user_service

run:
	docker compose up --build

tests:
	go test ./...

fmt:
	gofmt -w ./cmd ./pkg ./auth_service ./pin_service ./user_service

down:
	docker compose down

generate-mocks:
	go run go.uber.org/mock/mockgen@v0.6.0 -source=auth_service/usecase/service.go -destination=auth_service/usecase/mocks/mock_user_repository.go -package=mocks
	go run go.uber.org/mock/mockgen@v0.6.0 -source=user_service/usecase/service.go -destination=user_service/usecase/mocks/mock_user_repository.go -package=mocks
	go run go.uber.org/mock/mockgen@v0.6.0 -source=pin_service/usecase/service.go -destination=pin_service/usecase/mocks/mock_user_repository.go -package=mocks

cover:
	@go test -coverprofile=coverage.raw.out ./...
	@grep -vE '(^|/)mocks/|mock_[^:]*\.go:' coverage.raw.out > coverage.out
	@go tool cover -func=coverage.out
	@coverage=$$(go tool cover -func=coverage.out | awk '/^total:/ {gsub("%", "", $$3); print $$3}'); \
	 echo "Total coverage (generated mocks excluded): $$coverage%"; \
	 awk -v coverage="$$coverage" 'BEGIN { if (coverage+0 <= 60.0) { print "Coverage must be greater than 60%"; exit 1 } }'

# OpenAPI documentation is generated after the delivery/use-case refactoring.
swagger: swagger-auth swagger-pin swagger-user

swagger-auth:
	go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g main.go -d cmd/auth_service,auth_service -o docs/auth --parseDependency --parseInternal

swagger-pin:
	go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g main.go -d cmd/pin_service,pin_service -o docs/pin --parseDependency --parseInternal

swagger-user:
	go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g main.go -d cmd/user_service,user_service -o docs/user --parseDependency --parseInternal
