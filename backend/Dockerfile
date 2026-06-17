# --- Build stage ---
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install git and ca-certificates (required for some go modules)
RUN apk add --no-cache git ca-certificates

# Copy go mod and go sum
COPY go.mod go.sum ./

# 1. ESTE ES EL CAMBIO CLAVE: Descargamos dependencias en lugar de copiar carpeta
RUN go mod download

# Copy the source code
COPY . .

# 2. ESTE ES EL OTRO CAMBIO: Quitamos el -mod=vendor
# Ahora dejamos que Go use los módulos descargados en el paso anterior
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/main.go

# --- Run stage ---
# (El resto de tu archivo puede quedar igual)
FROM alpine:3.20
# Install curl for the healthcheck and create an unprivileged user
RUN apk add --no-cache curl ca-certificates su-exec \
    && adduser -D -u 10001 appuser \
    && mkdir -p /app/uploads \
    && chown appuser:appuser /app/uploads

WORKDIR /app

COPY --from=builder /app/main .
COPY docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod +x /app/docker-entrypoint.sh

EXPOSE 8080

ENTRYPOINT ["/app/docker-entrypoint.sh"]
CMD ["./main"]
