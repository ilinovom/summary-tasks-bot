# Build stage
FROM golang:1.20 AS build
WORKDIR /src
COPY go.mod .
# go.sum might not exist
RUN --mount=type=cache,target=/root/.cache/go-build go mod download
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build go build -o bot ./cmd/bot

# Runtime stage
FROM debian:buster-slim
WORKDIR /app
COPY --from=build /src/bot /app/bot
COPY options.json options.json
COPY tariff.json tariff.json
CMD ["/app/bot"]
