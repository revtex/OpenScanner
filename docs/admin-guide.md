# OpenScanner Admin Guide

The admin dashboard is available at /admin and requires an admin JWT.

## First Use

1. Complete setup at /setup (create first admin account).
2. Sign in at /login.
3. Open /admin.

If your account is not admin, the UI displays an access denied screen.

## Layout and Navigation

The admin layout uses a responsive sidebar:

- Mobile: drawer menu
- Medium: icon-only sidebar
- Large: icon + label sidebar

Primary nav items:

- Activity
- Users
- Systems
- Groups and Tags
- API Keys
- Monitors
- Downstreams
- Shared Links
- Options
- Logs
- Tools

Additional behavior:

- Scanner link returns to /
- Sign Out clears local credentials
- Webhooks panel route exists at /admin/webhooks but is not in the sidebar menu
- Unsaved-change navigation guard is active for admin forms

## Panels

### Activity

- Calls today/week/total
- Active listeners
- Server uptime
- 24-hour activity chart
- Top talkgroups

### Users

- Create/update/delete users
- Role assignment (admin/listener)
- Disable/enable users
- Expiration and connection limits

### Systems

- Manage systems, talkgroups, and units
- Reorder systems
- Nested talkgroup/unit management

### Groups and Tags

- CRUD for talkgroup grouping metadata

### API Keys

- Create and rotate upload keys
- Enable/disable
- Reorder keys
- Configure per-key system grants

### Monitors (DirMonitors)

- Manage directory ingest definitions
- Recorder type, path, mask, extension, delay/polling, delete-after
- Optional system/talkgroup/frequency overrides

### Downstreams

- Configure forwarding to remote OpenScanner instances
- URL, key, enable/disable, system grants

### Shared Links

- Admin list of all shared call links
- Remove shared links by id

### Options

Settings are grouped into:

- General
- Scanner Behavior
- Call Processing
- Display
- Sharing and Notifications
- Transcription
- Dashboard

Notable keys include publicAccess, autoPopulate, audioConversion, detectDuplicates, duplicateTime, apiKeyCallRate, shareableLinks, pushNotifications, activityDashboard.

### Logs

- Query by date range and level
- Virtualized rendering for large result sets

### Tools

- CSV import (talkgroups/units)
- CSV export (talkgroups/units)
- JSON config export/import
- Missing-audio scan and cleanup
- RadioReference preview/apply workflow

## Admin API Notes

All admin API endpoints are under /api/admin and require admin JWT.

Swagger endpoint contracts are available in /api/admin/docs after POST /api/admin/docs/session.
