# Build stage — pinned base image (audit finding M-06)
FROM node:20-alpine AS builder

WORKDIR /app

# Copy manifests and install from the lockfile for reproducible builds
# (audit finding M-07). Falls back to npm install if no lockfile is present.
COPY package*.json ./
RUN npm ci || npm install

# Copy source code and build
COPY . .
ENV NODE_OPTIONS="--max-old-space-size=2048"
RUN npm run build

# Run stage — unprivileged Nginx image (runs as uid 101, listens on 8080)
# (audit finding A-02).
FROM nginxinc/nginx-unprivileged:1.27-alpine

# Copy built files from builder stage
COPY --from=builder /app/dist /usr/share/nginx/html

# Nginx configuration as a TEMPLATE: the image's entrypoint renders it into
# /etc/nginx/conf.d/default.conf at startup. Only CRM_* variables are
# substituted, so nginx's own $variables are left alone.
COPY nginx.conf.template /etc/nginx/templates/default.conf.template
ENV NGINX_ENVSUBST_FILTER="^CRM_"

# Embedded Tasks for the CRM: CRM_ORIGIN is set at RUNTIME (Coolify / compose),
# not at build time. This script validates it and derives what the template
# needs; without it the embedded view stays closed. See
# docs/features/crm-tareas-embebido.md.
COPY --chmod=0755 docker-entrypoint.d/18-crm-origin.envsh /docker-entrypoint.d/18-crm-origin.envsh

EXPOSE 8080

CMD ["nginx", "-g", "daemon off;"]
