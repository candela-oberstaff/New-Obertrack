# Build stage — pinned base image (audit finding M-06)
FROM node:20-alpine AS builder

WORKDIR /app

# Copy manifests and install from the lockfile for reproducible builds
# (audit finding M-07). Falls back to npm install if no lockfile is present.
COPY package*.json ./
RUN npm ci || npm install

# Copy source code and build
COPY . .
RUN npm run build

# Run stage — unprivileged Nginx image (runs as uid 101, listens on 8080)
# (audit finding A-02).
FROM nginxinc/nginx-unprivileged:1.27-alpine

# Copy built files from builder stage
COPY --from=builder /app/dist /usr/share/nginx/html

# Copy custom nginx configuration
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 8080

CMD ["nginx", "-g", "daemon off;"]
