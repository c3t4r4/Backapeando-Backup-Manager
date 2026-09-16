# Multi-stage build: builder + runtime
# Builder stage: build Vue app with Node
FROM node:24-alpine AS builder

WORKDIR /app

# Copy package files
COPY package*.json ./

# Install dependencies
RUN npm ci

# Copy source code
COPY . .

# Same-origin API calls by default: the nginx proxy (see nginx.conf) forwards
# /api/ to the api service, so the built bundle should call a relative /api,
# not an absolute http://localhost:8081 (which is cross-origin from :3000 and
# gets blocked by the browser — the Go API sends no CORS headers).
ARG VITE_API_URL=
ENV VITE_API_URL=$VITE_API_URL

# Build Vue app
RUN npm run build

# Runtime stage: serve with Nginx
FROM nginx:alpine

# Copy nginx config
COPY nginx.conf /etc/nginx/nginx.conf

# Copy built dist from builder
COPY --from=builder /app/dist /usr/share/nginx/html

# Create non-root user (nginx already exists in nginx:alpine)
# but ensure permissions are correct
RUN chown -R nginx:nginx /usr/share/nginx/html && \
    chown -R nginx:nginx /var/cache/nginx && \
    chown -R nginx:nginx /var/log/nginx && \
    touch /var/run/nginx.pid && \
    chown -R nginx:nginx /var/run/nginx.pid

# Expose port
EXPOSE 80

# Health check: every 30 seconds, timeout 5s
# 127.0.0.1 (not localhost) — nginx.conf only binds IPv4, but `localhost`
# resolves to ::1 first inside the container, causing false "unhealthy" status.
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
    CMD wget -q -O- http://127.0.0.1/health || exit 1

# Run Nginx
CMD ["nginx", "-g", "daemon off;"]
