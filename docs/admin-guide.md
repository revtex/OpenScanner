# OpenScanner — Admin Guide

The admin dashboard is available at `/admin` and requires an admin-role JWT. On first visit the auth guard checks for a valid token and redirects to `/login` if missing or non-admin.

## Getting Started

### First-Run Setup

On a fresh install, navigating to the app shows the **Setup Wizard** (`/setup`). Enter an admin username and password (min 8 characters). After setup, you are redirected to `/login`.

### Logging In

Enter admin credentials at `/login`. If `passwordNeedChange` is set (first login), a change-password modal appears before proceeding. Successful admin login redirects to `/admin/users`.

## Dashboard Layout

The admin dashboard uses a responsive sidebar:

- **Mobile** — Hamburger menu opens a DaisyUI drawer overlay
- **Medium screens** — Icon-only sidebar
- **Large screens** — Icons + text labels

The sidebar contains 10 navigation items plus a **Sign Out** button.

## Admin Panels

### Users

Manage user accounts. Each row shows username, role badge (`admin` / `listener`), disabled status, expiration date, and connection limit.

- **Create** — Modal form: username, password, role selector, optional expiration and connection limit
- **Edit** — Inline editing of role, disabled toggle, expiration, connection limit
- **Delete** — Cannot delete your own account

### Systems

Manage radio systems with nested talkgroups and units.

- **Drag-to-reorder** — Reposition systems via `@dnd-kit` drag handles
- **Expand** — Click a system row to reveal its talkgroups (virtualized scrolling for large lists) and units in nested sub-tables
- **CRUD** — Create, edit, and delete systems, talkgroups, and units

### Groups & Tags

Two side-by-side tables for organizing talkgroups:

- **Groups** — Logical groupings (e.g., Fire, Law, EMS). Add, rename, or delete
- **Tags** — Classification tags. Add, rename, or delete

### API Keys

Manage upload API keys used by recorders to authenticate call uploads.

- **Generate** — Auto-generates UUID v4 key; copy-to-clipboard button
- **Drag-to-reorder** — Reposition keys via drag handles
- **System grants** — Control which systems each key can upload to
- **Enable/Disable** — Toggle key active status

### Accesses

Manage access codes for listener (non-admin) authentication.

- **Fields** — Code, ident, expiration date, concurrent connection limit
- **System grants** — Control which systems each access code can listen to

### Dir Watches

Configure directory monitoring for automatic call ingest from local recorder output.

- **Fields** — Directory path, recorder type (dropdown: trunk-recorder, sdrtrunk, rtlsdr-airband, etc.), file mask, extension filter, delay, delete-after toggle
- **CRUD** — Create, edit, delete watch configurations

### Downstreams

Configure forwarding of calls to remote OpenScanner instances.

- **Fields** — URL, API key, system grants, enable/disable toggle
- **CRUD** — Create, edit, delete downstream configurations

### Options

Application settings form organized into 6 sections:

- **General** — `publicAccess` toggle (with warning badge), `maxClients`, `autoPopulate`
- **Sharing** — Shareable link settings
- **Audio** — FFmpeg mode, audio conversion settings
- **Webhooks** — Webhook delivery settings
- **Transcription** — Enable/disable, Whisper model size, GPU toggle (fields shown conditionally when transcription is enabled)
- **Dashboard** — Display and UI preferences

All settings use appropriate input types (toggles, numbers, text fields).

### Logs

Virtualized log viewer for application logs.

- **Filters** — Date range pickers, log level dropdown (`info` / `warn` / `error`)
- **Virtual scrolling** — Handles large log volumes efficiently via `@tanstack/react-virtual`

### Tools

Utility operations for import, export, and account management.

- **CSV Import** — Upload talkgroup or unit CSV files with system selector
- **JSON Export** — Download full application config as JSON
- **JSON Import** — Upload a JSON config file (transactional; duplicates skipped)
- **Change Password** — Change your own admin password

### Webhooks

Configure webhook endpoints for event notifications.

- **Fields** — URL, webhook type (generic / discord), enabled toggle
- **Type badges** — Visual indicator for webhook type
- **CRUD** — Create, edit, delete webhook configurations

## Admin API Reference

All admin endpoints are under `/api/admin/` and require `Authorization: Bearer <jwt>` with `admin` role. Full request/response details are in [api.md](api.md#admin-crud-endpoints).

### Resource CRUD

Each resource supports **GET** (list), **POST** (create), **PUT /:id** (update), **DELETE /:id** (delete):

| Resource    | Base Path                |
| ----------- | ------------------------ |
| Users       | `/api/admin/users`       |
| Systems     | `/api/admin/systems`     |
| Talkgroups  | `/api/admin/talkgroups`  |
| Units       | `/api/admin/units`       |
| Groups      | `/api/admin/groups`      |
| Tags        | `/api/admin/tags`        |
| API Keys    | `/api/admin/apikeys`     |
| Accesses    | `/api/admin/accesses`    |
| Dirwatches  | `/api/admin/dirwatches`  |
| Downstreams | `/api/admin/downstreams` |
| Webhooks    | `/api/admin/webhooks`    |

### Other Endpoints

- **GET /api/admin/config** — All settings as `{key: value}` JSON
- **PUT /api/admin/config** — Update settings; broadcasts `CFG` to WebSocket clients
- **GET /api/admin/logs** — Query params: `from`, `to` (unix), `level` (`info`/`warn`/`error`)
- **POST /api/admin/import/talkgroups** — CSV upload with `system_id`
- **POST /api/admin/import/units** — CSV upload with `system_id`
- **GET /api/admin/export/config** — Full config JSON download
- **POST /api/admin/import/config** — Config JSON import (transactional)
