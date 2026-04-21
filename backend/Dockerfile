# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install git and ca-certificates (required for some go modules)
RUN apk add --no-cache git ca-certificates

# Copy go mod and vendor files
COPY go.mod go.sum ./
COPY vendor/ ./vendor/

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -o main ./cmd/main.go

# Run stage
FROM alpine:latest

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/main .
# Copy .env.example as a template if .env is not provided
COPY --from=builder /app/.env.example ./.env

EXPOSE 8080

CMD ["./main"]
