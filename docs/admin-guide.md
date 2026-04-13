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

The sidebar contains 9 navigation items, a **Scanner** link (Home icon) to navigate back to the scanner page, and a **Sign Out** button.

Each admin panel includes a help description paragraph below its heading explaining the panel’s purpose.

## Admin Panels

### Activity Dashboard

The Activity Dashboard is the first item in the admin sidebar navigation. It provides a real-time overview of system activity:

- **Stat cards** — Calls today, calls this week, total calls, active listeners, server uptime
- **24-hour activity chart** — Hourly call volume bar chart covering the last 24 hours
- **Top 10 talkgroups table** — Most active talkgroups by call count in the last 24 hours
- **Auto-refresh** — All data refreshes automatically every 30 seconds

The dashboard is only available when the `activityDashboard` setting is enabled in Options.

### Users

Manage user accounts. Each row shows username, role badge (`admin` / `listener`), disabled status, expiration date, and connection limit.

- **Create** — Modal form: username, password, role selector, optional expiration and connection limit
- **Edit** — Modal form for username, optional password reset, role, disabled flag, expiration, and connection limit
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

### Dir Watches

Configure directory monitoring for automatic call ingest from local recorder output.

- **Fields** — Directory path, recorder type (dropdown: trunk-recorder, sdrtrunk, rtlsdr-airband, etc.), file mask, extension filter, delay, delete-after toggle
- **CRUD** — Create, edit, delete watch configurations

### Downstreams

Configure forwarding of calls to remote OpenScanner instances.

- **Fields** — URL, API key, system grants, enable/disable toggle
- **CRUD** — Create, edit, delete downstream configurations

### Options

Application settings form organized into multiple sections:

- **General** — Branding, support email, public access, dark mode, keyboard shortcuts
- **Scanner Behavior** — Auto-populate, ordering/toggle behavior, listener count, max clients, AFS systems
- **Call Processing** — Audio conversion, duplicate detection settings, prune days
- **Display** — Dimmer delay, keypad beep style
- **Sharing & Notifications** — Shareable links, push notifications
- **Webhooks** — Webhook enable/disable switch
- **Transcription** — Enable/disable, binary path, model, language (model/language shown when enabled)
- **Dashboard** — Activity dashboard toggle

All settings use appropriate input types (toggles, numbers, text fields).

### Logs

Virtualized log viewer for application logs.

- **Filters** — Date range pickers, log level dropdown (`info` / `warn` / `error`)
- **Virtual scrolling** — Handles large log volumes efficiently via `@tanstack/react-virtual`

### Tools

Utility operations for import, export, and account management.

- **CSV Import** — Upload talkgroup or unit CSV files
- **JSON Export** — Download full application config as JSON
- **JSON Import** — Upload a JSON config file (transactional; duplicates skipped)
- **Missing Audio Audit** — Scan all systems for call records whose audio files are missing from disk; delete orphaned rows

### Shareable Links

When the `shareableLinks` setting is enabled in Options, each call gets a public URL at `/call/:id` that can be shared with anyone. The share page shows call metadata, an audio player, and any transcript.

### Bookmarks

Users can bookmark calls via the star icon on call cards. The bookmarks panel (accessible via the star button in the scanner) shows all bookmarked calls. Bookmarked calls are protected from automatic pruning.

### Webhooks

Configure webhook endpoints for event notifications. This panel is accessible at `/admin/webhooks` (not shown in the sidebar navigation).

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
| Dirwatches  | `/api/admin/dirwatches`  |
| Downstreams | `/api/admin/downstreams` |
| Webhooks    | `/api/admin/webhooks`    |

### Other Endpoints

- **GET /api/admin/config** — All settings as `[{key, value}, ...]` JSON array
- **PUT /api/admin/config** — Update settings; broadcasts `CFG` to WebSocket clients
- **GET /api/admin/logs** — Query params: `from`, `to` (unix), `level` (`info`/`warn`/`error`)
- **GET /api/admin/fs/directories** — List server directories for the Dir Watches browser
- **POST /api/admin/import/talkgroups** — CSV upload with `system_id`
- **POST /api/admin/import/units** — CSV upload with `system_id`
- **GET /api/admin/export/config** — Full config JSON download
- **POST /api/admin/import/config** — Config JSON import (transactional)
- **GET /api/admin/tools/audio-missing** — Find call rows whose audio files are missing
- **POST /api/admin/tools/audio-missing/cleanup** — Delete confirmed missing-audio call rows
- **GET /api/admin/ws** — Admin WebSocket endpoint
