# OpenScanner — multi-stage build

# Stage 1: Build frontend (must run before Go so go:embed has files to embed)
FROM node:22-alpine AS node-builder
WORKDIR /src/frontend
COPY frontend/ .
RUN corepack enable && pnpm install --frozen-lockfile && pnpm build

# Stage 2: Build Go binary with embedded frontend
FROM golang:1.25-alpine AS go-builder
WORKDIR /src/backend
COPY backend/ .
# Copy the built frontend dist into the go:embed target path
COPY --from=node-builder /src/frontend/dist ./internal/static/dist/
RUN go build -ldflags="-s -w" -o /openscanner ./cmd/server

# Stage 3: Minimal runtime image
FROM alpine:3.21
RUN apk add --no-cache ffmpeg ca-certificates && \
  adduser -D -u 1001 appuser && \
  mkdir -p /data && chown appuser /data
WORKDIR /app
COPY --from=go-builder /openscanner ./openscanner
USER appuser
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:3000/api/health || exit 1
ENTRYPOINT ["./openscanner", "--db-file", "/data/openscanner.db", "--recordings-dir", "/data/recordings"]
