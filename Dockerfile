FROM golang:1.20-alpine AS build
WORKDIR /app
COPY . .
RUN go build -o telegrambot ./cmd/telegrambot

FROM alpine:latest
WORKDIR /app
COPY --from=build /app/telegrambot .
CMD ["./telegrambot"]

