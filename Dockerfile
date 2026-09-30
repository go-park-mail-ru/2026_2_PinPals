# ---------- Этап сборки ----------
FROM golang:1.27-alpine AS builder

WORKDIR /src

# Сначала зависимости — они кешируются отдельно от кода
COPY go.mod go.sum ./
RUN go mod download

# Потом код
COPY . .

# Статически слинкованный бинарник
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/app ./internal/pins

# ---------- Этап запуска ----------
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /out/app /app/app

EXPOSE 8080

USER nobody
ENTRYPOINT ["/app/app"]
