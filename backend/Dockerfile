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

# ... (etapa builder igual)

# Run stage
FROM alpine:latest

# Instalar curl para el healthcheck
RUN apk add --no-cache curl

WORKDIR /app

COPY --from=builder /app/main .

EXPOSE 8080

CMD ["./main"]