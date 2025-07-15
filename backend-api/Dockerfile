FROM golang:1.20-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o /backend-api-server ./cmd/server

FROM alpine:latest
COPY --from=builder /backend-api-server /backend-api-server
ENTRYPOINT ["/backend-api-server"]
