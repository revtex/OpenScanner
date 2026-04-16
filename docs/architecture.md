# OpenScanner Architecture

## Overview

OpenScanner is a single-process Go server with an embedded React SPA.

Core runtime responsibilities:

- HTTP API and auth
- real-time WebSocket fanout
- recorder ingest (HTTP and DirMonitor)
- audio processing and storage
- admin operations and tooling

## High-Level Data Flow

1. Recorder uploads calls via API or writes files for DirMonitor.
2. Metadata is validated/resolved (system/talkgroup, settings, grants).
3. Audio is stored and optionally converted by FFmpeg workers.
4. Call metadata is persisted to SQLite.
5. Call events are broadcast to listeners over WebSocket.
6. Optional downstream forwarding and webhook processing executes.

## Backend Structure

- backend/cmd/server: process startup, config load, service lifecycle, HTTP/TLS servers
- backend/internal/api: Gin handlers and route registration
- backend/internal/middleware: request logging, auth, role checks, size/rate controls
- backend/internal/auth: JWT/password/token/rate-limit primitives
- backend/internal/db: migrations + sqlc query layer
- backend/internal/audio: audio storage/conversion/pruning
- backend/internal/ws: listener/admin websocket hub and client handling
- backend/internal/dirmonitor: directory watchers, parsers, mask extraction
- backend/internal/downstream: forwarding accepted calls to remote OpenScanner endpoints
- backend/internal/seed: initial app settings/groups/tags seeding

## API Surface

- REST base path: /api
- Listener websocket: /ws
- Admin websocket: /api/admin/ws

Route contracts are documented through Swagger annotations and generated docs.

## Auth and Access

- JWT auth for user/admin routes
- API key auth for ingest endpoints
- optional JWT middleware for public-capable listener routes
- admin role enforcement on /api/admin routes
- share token endpoints for public call playback

## Storage Model

- SQLite (WAL mode) for configuration and call metadata
- filesystem for audio payloads under configured recordings directory
- relative audio paths persisted in DB

## Frontend Structure

- React + TypeScript SPA
- Redux Toolkit + RTK Query state/data layer
- scanner UI: live controls, hold/avoid/select/search, bookmarks, sharing
- admin UI: CRUD, operations, tools, logs, and shared-link management
- styling: Tailwind CSS 4 + DaisyUI custom themes in src/index.css

## Runtime Services

- DirMonitor service: file watcher ingest, parser dispatch, mask enrichment
- Downstream service: async fanout to remote upload endpoints
- Call pruner loop: periodic retention cleanup
- Optional TLS with cert files or autocert

## Operational Notes

- Root make build embeds frontend artifacts into backend static dist before Go build.
- Backend make build regenerates Swagger docs.
- Health endpoint: /api/health
