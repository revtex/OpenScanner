# RadioReference Talkgroup Enrichment Plan

## Objective

Add an Admin-only workflow to enrich local talkgroup details from RadioReference using both API and CSV sources.

Primary goals:

- Support API-based enrichment with hierarchy selection: Country -> State -> County -> System
- Support direct RadioReference CSV import/enrichment
- Match local talkgroups by Talkgroup ID within a selected system
- Fill missing talkgroup details safely, with configurable merge behavior
- Never sync frequency from RadioReference

## Scope

Included:

- Backend endpoints and service layer for RadioReference API and CSV ingestion
- Frontend Admin UX under Tools
- Preview + apply flow with summary and row-level errors
- Admin JWT protection on all new endpoints

Excluded:

- Public or listener access
- Scheduled/background sync jobs
- Frequency updates from RR data

## Product Decisions

- API mode requires user login for RR access
- RR credentials are session-only (not persisted in settings/database)
- Matching key is `(system_id, talkgroup_id)`
- Default update policy is "fill missing only"
- Support per-field update toggles, but frequency is always excluded

## Data Contract

Normalized enrichment candidate fields (shared by API and CSV paths):

- talkgroupId (required)
- label (optional)
- name (optional)
- group (optional)
- tag (optional)
- led (optional)
- order (optional)

Hard exclusion:

- frequency must never be imported/updated from RR

Result payload shape:

- processed
- matched
- updated
- skipped
- errors
- rowErrors[] (row/index + reason)

## Architecture

### 1) Backend Provider Layer

Add a RadioReference integration package (example: `backend/internal/radioref/`) with clear boundaries:

- API auth/session client
- Hierarchy lookup client (country/state/county/system)
- Talkgroup detail fetch client
- CSV parser adapter (RR CSV -> normalized candidates)

### 2) Backend API Layer (Admin)

Add new routes under existing admin middleware chain (`JWTAuth + RequireAdmin`):

- RR login/session validation endpoint
- Hierarchy endpoints:
  - list countries
  - list states by country
  - list counties by state
  - list systems by county
- API preview endpoint (fetch + normalize RR system talkgroups)
- CSV preview endpoint (upload + normalize RR CSV)
- Apply endpoint (apply normalized candidates with selected merge policy)

### 3) Merge Engine

Create one shared merge engine used by API and CSV flows:

- Input: normalized candidates + selected system + merge options
- Match: local talkgroup by `(system_id, talkgroup_id)`
- Apply only selected fields
- Enforce frequency exclusion at engine level (defense in depth)
- Return deterministic result counts and row-level errors

### 4) Data Layer Integration

Use existing talkgroup constraints and sqlc query patterns.

- Reuse upsert/selective update semantics for talkgroups
- Add/select sqlc queries as needed for per-field update behavior
- Group/tag reconciliation:
  - Prefer existing local mapping
  - Optionally support create-if-missing (feature flag/toggle)
  - Otherwise skip unresolved and report in rowErrors

## Frontend Plan (Admin Area)

Add a new section/card in Admin Tools:

- Title: RadioReference Enrichment
- Mode selector: API | CSV

### API Mode UX

1. Login form for RR session
2. Country selector
3. State selector
4. County selector
5. System selector
6. Fetch preview
7. Review + apply

### CSV Mode UX

1. Upload RR CSV file
2. Parse/preview
3. Review + apply

### Shared Preview + Apply UX

- Table with local vs RR candidate values
- Per-row status: match/update/skip
- Merge controls:
  - Fill missing only (default)
  - Overwrite selected fields
  - Per-field toggles (excluding frequency)
- Apply button
- Summary panel (processed/matched/updated/skipped/errors)
- Downloadable row error report

## Security and Reliability Requirements

- All endpoints admin-only via existing middleware
- Never log secrets (credentials, tokens, full raw upstream payloads)
- Request/file size limits and row count limits (align with existing import safety)
- Outbound HTTP timeout + bounded retries/backoff for RR calls
- Redirect policy hardened for upstream requests
- Clear user-safe error messages for auth failure, quota/rate-limit, malformed CSV, unknown system

## Testing Plan

### Backend

- Route auth/role enforcement (401/403)
- RR session login/validation behavior
- Hierarchy endpoint validation
- CSV parsing and normalization
- Merge policy behavior (fill missing, overwrite selected fields)
- Frequency exclusion enforcement
- Partial-failure behavior and rowErrors content

### Frontend

- API flow wizard behavior (selectors and dependency resets)
- CSV upload + preview rendering
- Merge controls and field toggle behavior
- Apply success/failure UX and summary rendering
- Error handling states (auth failure, upstream failure, bad CSV)

### Integration/Manual

- End-to-end API enrichment by Talkgroup ID in selected system
- End-to-end CSV enrichment by Talkgroup ID in selected system
- Verify only intended fields changed and frequency unchanged
- Verify non-admin and listener roles cannot access RR endpoints

## Implementation Phases

1. Backend provider + endpoint skeletons + DTOs
2. API mode hierarchy + preview pipeline
3. CSV parser + preview pipeline
4. Shared merge/apply engine + sqlc updates
5. Frontend Tools UI for API and CSV modes
6. Test coverage + hardening + docs

## Relevant Files

Backend:

- `backend/internal/api/routes.go`
- `backend/internal/api/import.go`
- `backend/internal/api/crud.go`
- `backend/sqlc/queries/talkgroups.sql`
- `backend/migrations/007_create_talkgroups.sql`

Frontend:

- `frontend/src/components/admin/ToolsPanel.tsx`
- `frontend/src/app/slices/adminSlice.ts`
- `frontend/src/types/index.ts`

## Acceptance Criteria

- Admin can enrich talkgroups from RR via API flow and CSV flow
- API flow supports Country -> State -> County -> System selection
- Matching is by Talkgroup ID within selected system
- Frequency is never updated from RR
- Preview + apply workflow provides clear summary and row-level errors
- All new endpoints are admin-protected and test-covered
