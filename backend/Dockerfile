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

# Seeder de datos de demostración (servicio "seeder" del compose de desarrollo).
# Es un binario aparte: el servidor nunca lo ejecuta.
RUN CGO_ENABLED=0 GOOS=linux go build -o seed ./cmd/seed

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
COPY --from=builder /app/seed .

# BUILD_COMMIT viaja hasta /integrations/obersuite/version, que es donde
# Obersuite comprueba QUÉ está desplegado. La imagen se construye sin el .git,
# así que Go no puede incrustar el commit por su cuenta: hay que pasárselo.
#
#   docker compose build --build-arg BUILD_COMMIT=$(git rev-parse --short HEAD)
#
# Va en ESTA etapa, no en la de compilación: un ENV del builder no llega a la
# imagen final, y el endpoint contestaría "desconocido" en cada despliegue.
# Se declara al final para no invalidar la caché de las capas de arriba.
# Sin valor por defecto A PROPOSITO. Poniendo aqui "desconocido" la variable
# llega SIEMPRE definida, el codigo la da por buena y el endpoint contesta
# commit_source=BUILD_COMMIT con un commit que nadie puso: justo la mentira que
# commit_source existe para evitar. Vacia significa "no me la pasaron", y
# entonces se prueban las que publica la plataforma.
ARG BUILD_COMMIT=
ENV BUILD_COMMIT=$BUILD_COMMIT
COPY docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod +x /app/docker-entrypoint.sh

EXPOSE 8080

ENTRYPOINT ["/app/docker-entrypoint.sh"]
CMD ["./main"]
