# Stage 1: Build the Go binary
FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/processor ./cmd/processor

# Stage 2: Create the final, lightweight image
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/processor .
CMD ["./processor"]
