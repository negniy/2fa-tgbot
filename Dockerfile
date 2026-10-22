FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bot ./cmd

FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder /app/bot .
# Миграции читаются раннером из каталога ./migrations на старте.
COPY --from=builder /app/migrations ./migrations

CMD ["./bot"]
