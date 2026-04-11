# OpenScanner
# Build stage — Go
FROM golang:1.24-alpine AS go-builder
WORKDIR /src/backend
COPY backend/ .
RUN go build -o /openscanner ./cmd/server

# Build stage — Node
FROM node:22-alpine AS node-builder
WORKDIR /src/frontend
COPY frontend/ .
RUN corepack enable && pnpm install --frozen-lockfile && pnpm build

# Runtime
FROM alpine:3.21
RUN apk add --no-cache ffmpeg ca-certificates
WORKDIR /app
COPY --from=go-builder /openscanner ./openscanner
COPY --from=node-builder /src/frontend/dist ./static
EXPOSE 3000
ENTRYPOINT ["./openscanner"]
