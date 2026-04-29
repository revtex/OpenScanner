# OpenScanner — Project Layout & Conventions

Reference for human contributors and subagents. Terse on purpose. When in doubt, follow the existing tree; when the existing tree contradicts this doc, this doc wins and the tree is the bug.

Companion to [.github/copilot-instructions.md](copilot-instructions.md). Per-domain detail lives in [.github/agents/](agents/).

---

## 1. Top-level layout

```
openscanner/
  backend/         Go binary, sqlc, migrations
  frontend/        React + TypeScript SPA
  docs/            Tracked user/design docs (admin-guide, deployment-guide, recorder-guide)
  docs/plans/      Local-only working notes (gitignored — never reference from tracked files)
  .github/         Workflows, agents, this doc, copilot-instructions
  .devcontainer/   Codespaces / dev container
  scripts/         One-off operator scripts
  reference/       Vendored upstream reference (rdio-scanner) — read-only
  systems-config/  Sample TR / SDR configs
```

Industry analogues: backend follows [golang-standards/project-layout](https://github.com/golang-standards/project-layout) (`cmd/` + `internal/`), frontend follows [Bulletproof React](https://github.com/alan2207/bulletproof-react) (feature-scoped, no shared `utils/` dumping ground).

---

## 2. Backend (`backend/`)

```
backend/
  cmd/
    server/                main package — wires everything, runs the HTTP server
    migrate/               migration CLI (separate binary)
  internal/                everything is internal — no public Go API
    admin/                 transport-agnostic admin business logic
    audio/                 FFmpeg pipeline + bounded worker pool
    auth/                  JWT, refresh-token, cookie helpers, bcrypt
    cli/                   shared flag/env/INI parsing
    config/                config loading + validation
    db/                    sqlc-generated code + DB connection
    dirmonitor/            filesystem watchers for recorder ingest
    downstream/            forwarding to other rdio-scanner / openscanner instances
    handler/               HTTP handlers (Gin) — feature-scoped
      auth/                /api/v1/auth/*
      bookmarks/           /api/v1/bookmarks/*
      calls/               /api/v1/calls/*
      health/              /health
      setup/               /api/v1/setup/*
      share/               /api/v1/share/*
      shared/              DTOs + helpers used across handler subpackages
      admin/               /api/v1/admin/* — one subpackage per feature
        imports/
        legacyusage/
        radioreference/
        transcriptions/
      routes/              wiring: instantiates handlers, calls Register on each
    logging/               slog setup
    middleware/            Gin middleware (auth, request-id, recover, ratelimit)
    radioref/              RadioReference client + cache
    safehttp/              HTTP client with SSRF-style defaults (no redirects, capped body)
    seed/                  DB seeding for fresh installs
    static/                frontend `dist/` embed via go:embed
    ws/                    WebSocket hub + protocol (listener + admin)
  migrations/              numbered .sql files, append-only
  sqlc/
    sqlc.yaml
    queries/               one .sql file per table
    schema/schema.sql      mirror of final-state schema (regenerated)
  docs/                    swaggo output (swagger.json, swagger.yaml, docs.go)
  tools.go                 build-tag pinned dev tools (sqlc, migrate, swag)
```

### 2.1 Package boundaries

- **`cmd/`** is the only place `main.go` is allowed. Wire dependencies, then call into `internal/`.
- **`internal/`** subpackages are each a single feature or cross-cutting concern. No package re-exports another package's types.
- **Transport vs business logic split:** HTTP and WS are transports under `handler/` and `ws/`; business logic that admin operations both call lives in `internal/admin/`. Same rule applies to anything else with two transports (e.g. CLI + HTTP).
- A package owns its own DTOs, its own errors, and its own Swaggo annotations.
- **No `utils/`, `helpers/`, `common/` packages.** If two packages need the same code, find a real domain name for it (`safehttp`, `radioref`, `logging`).

### 2.2 When to add a package

Add an `internal/<name>/` package when **one** of these is true:

- It owns long-lived state (a hub, a watcher, a worker pool).
- It encapsulates a third-party integration (RadioReference, FFmpeg, whisper).
- It has a clearly different lifecycle from its caller (background goroutine, separate context).
- It is reused by ≥ 2 unrelated transports (HTTP + WS, HTTP + CLI).

Don't add a package for "things that feel related." A 200-line `.go` file inside an existing package is fine.

### 2.3 When to split a `.go` file

Default file size budget: ~500–800 lines. Split when:

- File exceeds ~800 lines **and** has clear seams (one file per resource, one per protocol opcode group).
- Multiple unrelated tests dominate the file (move tests next to the symbol under test).
- A single struct's methods grow past one screen — split methods into `<type>_methods.go` only if the file is over budget.

Convention: feature handler subpackages use `handler.go` (or per-route file like `login.go`, `refresh.go`), `dto.go` (request/response types), and `<feature>_test.go` adjacent.

### 2.4 Naming

- Files: lowercase, `_` for word separation: `refresh_token.go`, `replay_cache.go`.
- Tests: `<file>_test.go`. Table-driven tests: `Test<Func>_<Scenario>` with cases as struct slice.
- Packages: short, lowercase, no underscores. Match the directory name.
- Exported symbols: `CamelCase`. Unexported: `camelCase`. Stutter (`auth.AuthHandler`) is wrong — prefer `auth.Handler`.

### 2.5 Errors, logging, security

- Always return errors; never `panic` in HTTP/WS handlers. Recovery middleware is a safety net, not a strategy.
- Wrap with `fmt.Errorf("doing X: %w", err)`; only sentinel errors are bare `errors.New`.
- Logging: `log/slog` only. No `log.Println`, no `fmt.Println` in production code paths.
- Never log secrets — passwords, JWTs, refresh tokens, decrypted `enc::` values, API keys (log a truncated identifier only).
- All SQL via sqlc; no string concatenation.
- External processes via `exec.CommandContext` with arg slice; never a shell string.
- Outbound HTTP via `safehttp.Client`.
- See [.github/copilot-instructions.md § Security Rules](copilot-instructions.md) for the full OWASP-aligned list.

### 2.6 Database conventions

- One query file per table in `backend/sqlc/queries/<table>.sql`.
- Migrations in `backend/migrations/NNN_<verb>_<noun>.sql`, zero-padded 3 digits, **append-only** — never rewrite a committed migration.
- `backend/sqlc/schema/schema.sql` mirrors the final-state schema; regenerate after adding migrations.
- After editing `.sql` files: `cd backend/sqlc && sqlc generate`.
- Every index must correspond to a real query path; remove indexes that no query uses.

### 2.7 Swagger / API contract

- Every public `/api/*` (and `/api/v1/*`) endpoint has Swaggo annotations on the handler.
- Regenerate with `make swag`. Commit the generated [backend/docs/swagger.{json,yaml}](backend/docs/) and [backend/docs/docs.go](backend/docs/docs.go).
- Response envelopes for v1: `{"error":{"code","message","details"}}` with stable string codes (`validation_failed`, `unauthorized`, `forbidden`, `not_found`, `conflict`, `unprocessable`, `rate_limited`, `internal`).

---

## 3. Frontend (`frontend/`)

```
frontend/
  index.html
  vite.config.ts
  tsconfig.json
  eslint.config.js
  sw.ts                    PWA service worker (push notifications)
  public/manifest.json     PWA manifest
  src/
    main.tsx               entry — wires store, router, providers
    index.css              Tailwind + DaisyUI imports
    test-setup.ts          Vitest globals, MSW handlers (if used)
    app/
      api.ts               single shared RTK Query api object (one createApi call)
      store.ts             Redux store assembly
      audioListenerMiddleware.ts
      slices/
        admin/             feature slices for the admin surface
        scanner/           feature slices for the scanner surface
        shared/            slices used by both
    pages/
      Login.tsx            top-level routed pages — thin shells, no business logic
      Setup.tsx
      Scanner.tsx
      Admin.tsx
      SharedCall.tsx
    components/
      admin/               panels rendered inside Admin.tsx
      scanner/             panels rendered inside Scanner.tsx
    hooks/
      admin/               admin-only hooks
      scanner/             scanner-only hooks
      shared/              hooks usable by either surface
    services/
      audio/               player + beep
      ws/                  listener + admin WebSocket clients
      util/                (avoid growing this — promote to a real service when patterns emerge)
    types/
      index.ts             barrel — re-exports the others
      api.ts  auth.ts  call.ts  config.ts  admin.ts  ws.ts  ui.ts
```

Industry analogues: feature folders + page shells follow [Bulletproof React](https://github.com/alan2207/bulletproof-react). Folder-as-component pattern for panels is documented below.

### 3.1 Imports

- All imports use the `@/` alias for `src/`. No relative `../../` chains.
- Cross-feature imports flow through public barrels (`@/components/admin/UsersPanel`), never reach into siblings (`@/components/admin/UsersPanel/UsersTable` — forbidden, see § 3.4).

### 3.2 Pages vs components

- `pages/*` are **thin shells**. They handle routing, top-level layout, and dispatch — not business logic.
- Page-specific UI lives under `components/<surface>/`. A page imports panels; a panel never imports a page.
- A page that grows past ~300 lines is a smell — push panels out.

### 3.3 State

- Server data: RTK Query, single shared `api` object in [frontend/src/app/api.ts](frontend/src/app/api.ts). No per-feature `createApi` calls.
- Client state: Redux Toolkit slices under `app/slices/<surface>/`. One slice per coherent state domain (e.g. `scannerSlice`, `callsSlice`, `shareSlice`).
- WebSocket events dispatch into Redux via the WS client + middleware — components never parse WS frames.
- JWT tokens live in **Redux memory only**. No `localStorage`, no `sessionStorage`. Refresh token is an httpOnly cookie scoped to `/api`.

### 3.4 Folder-as-component panel convention

Panels under `components/admin/` and `components/scanner/` use the **folder-as-component** layout when they have ≥ 2 clear seams:

```
components/admin/UsersPanel/
  UsersPanel.tsx          named root file (default export)
  UsersTable.tsx          private subcomponent
  UserEditor.tsx          private subcomponent
  helpers.ts              local-only helpers (only when ≥ 2 sibling files use them)
  UsersPanel.test.tsx     public-surface test
  index.ts                one-line barrel: export { default } from "./UsersPanel";
```

Rules:

- **Named root file**, never `index.tsx`. Stack traces, editor tabs, and `git blame` need a real name.
- **Tiny `index.ts` barrel** (`.ts`, not `.tsx`) — re-export only, no JSX, no logic.
- **Sibling folders are opaque.** Enforced by eslint `no-restricted-imports` against `@/components/{admin,scanner}/*/!(index)`.
- **One default export per file.** Subcomponent files don't bundle multiple components.
- **Seam test:** split into a folder when the panel has ≥ 2 obvious seams (e.g. table ↔ editor, filters ↔ list ↔ pagination). A 600-line panel with no seams stays single-file.
- **Don't over-extract.** A subcomponent that needs > 3 props or a callback chain ≥ 2 hops to live outside its parent has the wrong seam — leave it inline.

Pages and small components (banners, buttons, < ~250 lines, no seams) stay as single files.

### 3.5 Hooks

- Topical sub-folders: `hooks/{admin,scanner,shared}/`. Each has a barrel `index.ts`.
- Prefer specific imports (`@/hooks/shared/useTheme`) over the root barrel.
- A custom hook that wraps **only** an `api.ts` query/mutation does not need to exist — call the RTK Query hook directly from the component.

### 3.6 Types

- Topic-scoped modules under `types/`: `api`, `auth`, `call`, `config`, `admin`, `ws`, `ui`.
- `types/index.ts` is an exhaustive barrel so `@/types` continues to work.
- Mirror server DTO names where practical; document any divergence in the same file.

### 3.7 Strictness

- TypeScript strict mode. **No `any`. No `@ts-ignore`.** Use `unknown` and narrow.
- No `dangerouslySetInnerHTML`.
- DaisyUI 5 classes for UI primitives; no hand-rolled equivalents.
- All async work goes through RTK Query or explicit Redux thunks — never naked `fetch` in components.

### 3.8 Naming

- Components: `PascalCase.tsx` matching the default export name.
- Hooks: `useCamelCase.ts`.
- Slices: `<domain>Slice.ts`.
- Services: lowercase noun (`player.ts`, `beep.ts`, `client.ts`).
- Tests: colocated, `<file>.test.ts(x)`. Test descriptions present-tense (`it("renders empty state when ...")`).

---

## 4. Cross-cutting

### 4.1 Tests

- **Backend:** table-driven, `httptest.NewServer` for handler tests, `mochi-co/mqtt/v2` (or similar in-process) for transport tests, real sqlite (modernc.org) — never mock the DB.
- **Frontend:** Vitest + React Testing Library. Test the **component** (queries, user events), not implementation details. RTK Query endpoints are tested by mocking the network (MSW) at the boundary.
- Tests live next to source: `foo.go` → `foo_test.go`, `Bar.tsx` → `Bar.test.tsx`.
- No fixture directories named `__mocks__/` or `__fixtures__/` unless absolutely necessary; prefer inline test data.

### 4.2 Tooling expectations

- Search: VS Code `grep_search` first; in the terminal use `rg` (ripgrep), never plain `grep`.
- Listing: `list_dir` or `file_search`, never shell `find`.
- Validation after a backend change: `cd backend && go vet ./... && go build ./...`.
- Validation after a frontend change: `cd frontend && npx tsc --noEmit && pnpm test`.
- Backend live reload: [air](https://github.com/cosmtrek/air). Frontend dev server: [Vite](https://vitejs.dev/). Single `make dev` runs both.
- SQL: `sqlc generate` after every `.sql` change. `swag init` (via `make swag`) after every Swaggo change.

### 4.3 Build / release / changelog

- Versioning: [SemVer](https://semver.org/). Tags `vX.Y.Z`.
- `CHANGELOG.md` follows [Keep a Changelog](https://keepachangelog.com/). Bullets describe **what changed in the product**, never reference plan filenames.
- Any user-visible change ships with a release tag in the same session — never leave a user-visible bullet sitting in `[Unreleased]`.
- Pure CI / internal-docs / refactor changes can batch under `[Unreleased]` or use the `skip-changelog` PR label.
- See [docs/plans/release-guide.md](../docs/plans/release-guide.md) for the local checklist (gitignored — operator-side).

### 4.4 Local-only planning docs

- The entire `docs/plans/` directory is **gitignored**. Files there are personal scratchpads.
- **Never** reference plan files from any tracked file — CHANGELOG, committed docs, commit messages, PR titles, code comments.
- If you find a tracked file that links into `docs/plans/`, that's a bug — remove the reference.

### 4.5 Subagent delegation

Domain work goes to its expert agent:

| Domain                                      | Agent            |
| ------------------------------------------- | ---------------- |
| Backend Go                                  | Go Expert        |
| React / TypeScript                          | React Expert     |
| Schema, migrations, sqlc                    | Database Expert  |
| User-facing docs                            | Docs Expert      |
| Security / quality review                   | Reviewer         |
| Writing tests                               | Testing Expert   |
| Dead code, unused imports, stale files      | Cleanup Expert   |
| Trunk-recorder log analysis, SDR tuning     | TR Tuning Expert |
| Read-only investigation across the codebase | Explore          |

The top-level conversation coordinates and reports; agents do the work. Inline handling is acceptable only for trivial one-liners.

---

## 5. Industry references

- [golang-standards/project-layout](https://github.com/golang-standards/project-layout) — `cmd/` + `internal/`.
- [Bulletproof React](https://github.com/alan2207/bulletproof-react) — feature folders, no shared `utils/`.
- [Keep a Changelog](https://keepachangelog.com/) — CHANGELOG format.
- [Semantic Versioning](https://semver.org/) — version numbers.
- [Conventional Commits](https://www.conventionalcommits.org/) — commit message format (`feat:`, `fix:`, `refactor:`, `chore:`, `docs:`, `test:`).
- [The Twelve-Factor App](https://12factor.net/) — config via env, logs as event streams, stateless processes.
- [OWASP Top 10](https://owasp.org/www-project-top-ten/) — the security baseline (see copilot-instructions.md § Security Rules).

---

## 6. When the rules conflict with reality

- If you find code that violates this doc, the code is the bug — fix it in your PR if it's in scope, otherwise note it and move on.
- If a rule blocks legitimate work, propose an amendment to this doc in the same PR rather than working around it silently.
- Pragmatism beats dogma. The seam test, the package-boundary test, and the file-size budgets are guidelines — explain in the PR description when you deviate.
