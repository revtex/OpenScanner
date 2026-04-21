# OpenScanner — multi-stage build

# Stage 1: Build frontend (must run before Go so go:embed has files to embed)
FROM node:22-alpine AS node-builder
WORKDIR /src/frontend
COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN corepack enable && pnpm install --frozen-lockfile
COPY frontend/ .
RUN pnpm build

# Stage 2: Build Go binary with embedded frontend
FROM golang:1.25-alpine AS go-builder
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
RUN go install github.com/swaggo/swag/cmd/swag@latest
COPY backend/ .
# Copy the built frontend dist into the go:embed target path
COPY --from=node-builder /src/frontend/dist ./internal/static/dist/
# Generate Swagger docs (gitignored, must be built in CI)
RUN swag init -g cmd/server/main.go --parseDependency --parseInternal
RUN go build -ldflags="-s -w" -o /openscanner ./cmd/server

# Stage 3: Minimal runtime image
FROM alpine:3.21
RUN apk add --no-cache ffmpeg ca-certificates tzdata && \
  adduser -D -u 1001 appuser && \
  mkdir -p /data/recordings && chown -R appuser:appuser /data
WORKDIR /app
COPY --from=go-builder /openscanner ./openscanner
USER appuser
# Defaults for standalone docker run; override via environment or compose.
ENV OPENSCANNER_LISTEN=0.0.0.0:3022
ENV OPENSCANNER_DB_FILE=/data/openscanner.db
ENV OPENSCANNER_RECORDINGS_DIR=/data/recordings
EXPOSE 3022
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:3022/api/health || exit 1
ENTRYPOINT ["./openscanner"]
