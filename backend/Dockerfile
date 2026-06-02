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

# Build the application (static binary)
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -o main ./cmd/main.go

# Run stage — pinned dig’d minimal base (audit findings A-02, M-06)
FROM alpine:3.20

# Install curl for the healthcheck and create an unprivileged user
RUN apk add --no-cache curl ca-certificates \
    && adduser -D -u 10001 appuser

WORKDIR /app

COPY --from=builder /app/main .

# Drop root privileges (audit finding A-02)
USER appuser

EXPOSE 8080

CMD ["./main"]
