# ---------- Этап сборки ----------
FROM golang:1.27-alpine AS builder

WORKDIR /src

# Сначала зависимости — они кешируются отдельно от кода
COPY go.mod go.sum ./
RUN go mod download

# Потом код
COPY . .

# Статически слинкованный бинарник
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/app ./cmd/server/main.go

# ---------- Этап запуска ----------
FROM --platform=linux/amd64 alpine:3.20

WORKDIR /app

COPY --from=builder --chmod=755 /out/app /app/app

COPY templates /app/images

EXPOSE 8080

USER nobody
ENTRYPOINT ["/app/app"]
