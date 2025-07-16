FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o /db-init-server ./cmd/dbinit

FROM alpine:latest
COPY --from=builder /db-init-server /db-init-server
ENTRYPOINT ["/db-init-server"]
