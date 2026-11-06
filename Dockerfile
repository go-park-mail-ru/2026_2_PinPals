# ---------- Этап сборки ----------
FROM golang:1.27-alpine AS builder

ARG SERVICE

WORKDIR /src

# Сначала зависимости — они кешируются отдельно от кода
COPY go.mod go.sum ./
RUN go mod download

# Потом код
COPY . .

# Генерируем OpenAPI именно для собираемого сервиса, если в его main.go есть @title.
# Это не требует общего Dockerfile для отдельных сервисов и не ломает сборку,
# если ветка с конкретным сервисом ещё не влита.
RUN go install github.com/swaggo/swag/cmd/swag@v1.16.6
RUN if grep -q '^// @title' "cmd/${SERVICE}/main.go"; then \
      /go/bin/swag init -g main.go -d "cmd/${SERVICE},${SERVICE}" -o "docs/${SERVICE%_service}" --parseDependency --parseInternal; \
    fi

# Статически слинкованный бинарник
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/app ./cmd/${SERVICE}

# ---------- Этап запуска ----------
FROM --platform=linux/amd64 alpine:3.20

WORKDIR /app

COPY --from=builder --chmod=755 /out/app /app/app

EXPOSE 8000

USER nobody
ENTRYPOINT ["/app/app"]
