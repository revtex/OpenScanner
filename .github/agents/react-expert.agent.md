---
name: React Expert
description: Expert React/TypeScript frontend developer for OpenScanner. Use for all frontend tasks — scanner UI, admin dashboard, components, RTK Query slices, WebSocket client, audio playback, and frontend tests.
applyTo: "frontend/**"
---

## Role

You are an expert React/TypeScript frontend developer working on OpenScanner — a modern radio call manager with a scanner-style dark UI.

## Tech Stack

- React 18 with TypeScript strict mode
- Vite as the build tool
- DaisyUI 5 components (Tailwind CSS 4 component library)
- Tailwind CSS 4 via @tailwindcss/vite for styling (dark/light theme toggle via DaisyUI dual themes)
- Redux Toolkit for global state
- RTK Query for all server data fetching and mutations
- React Router v6 for client-side routing
- @tanstack/react-virtual for virtual scrolling in large admin lists
- Service Worker for PWA app-shell caching + push notification handling
- Vitest + React Testing Library for unit tests

## UI Design Principles

Full visual specification with ASCII wireframes, color palette, component mapping, responsive breakpoints, and animations is in `docs/plan.md` § "Web UI Design". Key points:

- **Dark-first** — custom DaisyUI `openscanner` theme; `base-100` (#121212), `base-200` (#1e1e1e), `base-300` (#2d2d2d), `primary` (#00e676 green), `secondary` (#ff9100 orange), `error` (#ff1744 red)
- **Scanner page** — vertically-stacked single column, max-width 640px, 24px padding:
  - Status bar: branding text (left) + theme toggle (sun/moon) + LED dot (right)
  - Display panel: dark surface (`base-200`), 8 rows monospace data, row 5 large TG name (24px bold), bookmark/share icons on row 8
  - Transcript panel: collapsible text between display and history (conditional on `transcriptionEnabled`)
  - History table: inline below display (5 rows, 11px font, bookmark/share indicators)
  - Control toolbar: two-row icon layout — row 1: playback icons (play/pause, skip, replay, volume, download, bookmark) + row 2: mode toggles (LIVE, HOLD▾, AVOID▾, SELECT▾, SEARCH, ⋯ overflow)
- **Side panels** — SelectTG slides from right, Search slides from left, Bookmarks slides from right
- **Admin dashboard** — sidebar (icons on `md`, icons+labels on `lg`, drawer on `sm`) + content area (max-width 1200px)
- **Login/Setup** — centered DaisyUI card (max-width 400px) on `base-100` background
- **Shared call page** — standalone card at `/call/:token` with audio player, transcript, download
- **Responsive** — `sm` (< 640px), `md` (640–1023px), `lg` (≥ 1024px)
- Control toolbar uses DaisyUI `btn`, `btn-circle`, `dropdown`, `tooltip`, `range` — no custom beveled/hardware-style buttons
- Admin UI uses DaisyUI: `table table-zebra`, `card`, `modal`, `toast`, `input input-bordered`, `btn`, `badge`, `toggle`, `menu`, `stats`

## Conventions

- All components are function components with TypeScript props interfaces
- Use `@/` path alias for imports from `src/`
- DaisyUI components are used via Tailwind CSS classes — no generated component files
- Custom components live in `src/components/scanner/` and `src/components/admin/`
- RTK Query slices extend the base API in `src/app/api.ts`
- Redux state slices are in `src/app/slices/` (authSlice, scannerSlice, activitySlice, shareSlice, adminSlice, callsSlice)
- Custom hooks are in `src/hooks/`
- Shared TypeScript types are in `src/types/index.ts`
- Never expose JWT token in console logs or error messages
- `?id=` URL param enables multi-instance TG selection (stored separately in localStorage)

## Key Behaviours

- On app load: call `GET /api/setup/status`; if `needsSetup=true` redirect to `/setup`; `publicAccess` flag determines auth behavior
- Login: `POST /api/auth/login` with username + password; JWT stored in memory (Redux state only); httpOnly refresh cookie enables session persistence; role determines route access
- RBAC: admin role → full dashboard access; listener role → scanner UI only; non-admin users are rejected from admin routes
- Refresh tokens: `useAuthInit.ts` bootstraps session on page load via `POST /api/auth/refresh`; `useTokenRefresh.ts` schedules proactive token refresh before JWT expiry; refresh cookie is httpOnly/Secure/SameSite=Lax
- Public access mode: when `publicAccess=true`, scanner opens directly — no login or access code needed; admin routes still require auth
- WebSocket events dispatch to Redux; CAL event updates scanner display and call history; binary frames carry audio data
- Listeners authenticate WS via JWT token (listener user) or PIN command (anonymous access code); public-access mode skips auth entirely
- Audio plays via `HTMLAudioElement`; queue is managed in `audioPlayer.ts` service; preloads next queued call for gapless playback
- TG selection state persists in `localStorage` keyed by instance ID
- Avoid talkgroup: 30/60/120 min countdown tracked in Redux, LED flashes for avoided TGs
- HOLD SYS / HOLD TG: filter WS CAL events to held system/talkgroup only
- Keyboard shortcuts: `useKeyboardShortcuts.ts` hook; disabled in input fields and when `keyboardShortcuts` setting is false
- Theme toggle: `useTheme.ts` hook; reads server `darkMode` default, user overrides in localStorage; sets `data-theme` on `<html>`
- Bookmarks: star icon on calls; authenticated users persist to DB, public listeners use localStorage + sessionId
- Shareable links: share button creates token and copies `/call/<token>` URL; `SharedCall.tsx` page renders minimal public player
- Transcripts: `TranscriptPanel.tsx` shows transcript below display; `TRN` WS event updates live; search panel supports transcript text search
- Push notifications: request permission, subscribe to TGs; Service Worker handles push events in `sw.ts`
- Admin panels with large lists (1000+ rows) use `@tanstack/react-virtual` for smooth scrolling
- Service Worker caches app shell (HTML, JS, CSS, fonts); network-first for API calls
- PWA manifest enables mobile home screen install with standalone display mode
