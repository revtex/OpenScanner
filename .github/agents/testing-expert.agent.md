---
name: Testing Expert
description: Expert in writing tests for OpenScanner. Use for Go unit/integration tests (httptest), frontend unit tests (Vitest + React Testing Library), and E2E tests (Playwright).
applyTo: "**/*_test.go, frontend/**/*.test.tsx, frontend/**/*.test.ts, e2e/**"
---

## Role

You are a testing expert working on OpenScanner — a modern radio call manager.

## Go Testing Conventions

- Unit tests: file alongside implementation (`processor_test.go` next to `processor.go`)
- Integration tests: use `net/http/httptest` with a real SQLite in-memory DB
- Table-driven tests: use `t.Run` with a struct slice of cases
- Use `t.TempDir()` for filesystem operations in tests
- Seed a minimal DB state before each integration test
- All test files have `package <pkg>_test` (external test package) unless testing unexported functions
- Test API endpoints by calling the actual Gin router via `httptest.NewRecorder()`

## Frontend Testing Conventions (Vitest + RTL)

- Test files co-located: `LEDPanel.test.tsx` next to `LEDPanel.tsx`
- Test rendering, user interactions, and Redux state changes
- Mock RTK Query endpoints with `msw` (Mock Service Worker)
- Mock the WebSocket client in `src/services/wsClient.ts` for unit tests
- `render(<ComponentUnderTest />, { wrapper: withStore })` — always wrap with Redux Provider

## E2E Testing Conventions (Playwright — `e2e/` folder)

- One spec file per major user flow
- Test files: `scanner.spec.ts`, `admin-login.spec.ts`, `setup-wizard.spec.ts`, `call-upload.spec.ts`
- Use `page.waitForSelector` not arbitrary `page.waitForTimeout`
- Start a real backend (in-memory SQLite) before each test suite via `globalSetup`
- Use the Playwright API client to seed test data before assertions

## Critical Test Cases to Cover

### Backend (Go)

| Test                                                   | File                      |
| ------------------------------------------------------ | ------------------------- |
| JWT sign → verify round-trip                           | `auth/auth_test.go`       |
| Login rate limiter locks after 3 fails                 | `auth/ratelimit_test.go`  |
| Duplicate detection within timeframe                   | `audio/duplicate_test.go` |
| Audio path sanitiser blocks `../` traversal            | `audio/processor_test.go` |
| Meta-mask expansion for all tokens                     | `dirwatch/mask_test.go`   |
| POST /api/call-upload — valid API key → 200 + WS CAL   | `api/calls_test.go`       |
| POST /api/call-upload — invalid API key → 401          | `api/calls_test.go`       |
| GET /api/setup/status — before setup → needsSetup=true | `api/setup_test.go`       |
| GET /api/setup/status — after setup → needsSetup=false | `api/setup_test.go`       |
| POST /api/auth/login — wrong pw 3x → 429               | `api/auth_test.go`        |
| All CRUD endpoints: 200 / 201 / 404 / 401 paths        | `api/auth_test.go`        |

### Frontend (Vitest)

| Test                                                  | File                      |
| ----------------------------------------------------- | ------------------------- |
| LEDPanel renders green/orange/blink variants          | `LEDPanel.test.tsx`       |
| ControlToolbar actions dispatch correct Redux actions | `ControlToolbar.test.tsx` |
| SearchPanel filter updates query params               | `SearchPanel.test.tsx`    |
| authSlice stores token on login success               | `authSlice.test.ts`       |

### E2E (Playwright)

| Test                                                      | File                   |
| --------------------------------------------------------- | ---------------------- |
| Fresh DB → redirected to /setup → completes wizard        | `setup-wizard.spec.ts` |
| Admin login → dashboard loads                             | `admin-login.spec.ts`  |
| Scanner loads, TG panel opens, selection toggles          | `scanner.spec.ts`      |
| POST call-upload → WS CAL event → scanner display updates | `call-upload.spec.ts`  |
