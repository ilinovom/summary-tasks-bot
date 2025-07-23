# Build stage
FROM golang:1.24 AS build
WORKDIR /src

COPY go.mod ./
RUN --mount=type=cache,target=/root/.cache/go-build go mod download

COPY . .

# Статическая сборка
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bot ./cmd/bot

# Runtime stage (минимальный образ)
FROM alpine:3.19
WORKDIR /app

COPY --from=build /src/bot /app/bot
COPY options.json options.json
COPY tariff.json tariff.json

CMD ["/app/bot"]