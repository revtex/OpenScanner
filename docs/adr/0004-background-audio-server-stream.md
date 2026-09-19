# 0004. Background audio via a server-side stream

- **Status:** Accepted
- **Date:** 2026-09-19

## Context

iOS suspends a backgrounded page shortly after its audio stops. A scanner is
silent between calls, so the ordinary per-call player stops dead once the
screen locks and never gets to play the next Call. A listener who locks their
phone loses the feed — which is the single most common way anyone actually
listens.

The same is true on Android: real-device testing on 2026-09-18 confirmed the
normal LIVE mode does not survive backgrounding there either, so this is not an
iOS-only workaround.

Web Audio is not available as a route: iOS suspends an `AudioContext` when the
page is backgrounded and interrupts it on screen lock — exactly the situation
being solved for.

## Decision

The server exposes `/api/v1/listener/stream`: a never-ending chunked response
of MPEG frames, silence-padded between Calls, with the Listener's Selection,
AVOID entries, and system grants applied server-side. The browser plays it with
a plain `<audio>` element and never stops it. This is the **BKGND** mode.

The transport is deliberately dumb, because a plain `<audio>` element is what
every platform can consume — including iOS with the screen locked. Pacing,
per-listener filtering, and backlog trimming live in `internal/stream`.

## Consequences

- There is no gap for the platform to suspend us in, so no keep-alive, no
  handover timing, and no time limit on how long a listener can stay
  backgrounded.
- It costs a persistent connection and server-side pacing per listener.
- The lock-screen transport cannot do meaningful next/previous, because the
  server owns the timeline. Play/pause work; skipping does not.
- The Queue count is meaningless in this mode — the display shows a dash.
- BKGND is shown on mobile only (`isMobilePlatform()`, which checks iOS,
  Android, **and** a coarse primary pointer, because Chrome for Android's
  "Desktop site" switch rewrites the user agent).

### Verified on real devices

- Audio continues with the phone locked (2026-09-18).
- The lock-screen pause button still works after backgrounding, so iOS keeps
  its side of the `setActionHandler` registration and the Media Session
  handlers do not need rebinding on `visibilitychange` (2026-09-19).
- Silent Mode does not mute the stream, so Safari's default `auto` audio-session
  inference already grants a playback-category session and setting
  `navigator.audioSession.type = "playback"` would change nothing (2026-09-19).

Emulation cannot answer any of these — headless Chrome with a mobile user agent
runs the right engine but does not reproduce device power management.

## Alternatives considered

- **Client-side gap bridging.** Keep one `<audio>` element and, when the queue
  empties, point it at a looping inaudible clip so the audio session never
  closes. [FxllenCode/radio-scout](https://github.com/FxllenCode/radio-scout)
  implements exactly this: a generated ±1-LSB 100 Hz WAV (≈ −90 dBFS, not
  digital silence, because behaviour below WebKit is undocumented), handed over
  300 ms *before* the last Call ends rather than on `ended`, abandoned after a
  five-minute budget.

  It is a genuinely good design and needs no server support, which makes it the
  right answer for a client that cannot change its backend. Rejected here
  because we own the server: it depends on undocumented WebKit behaviour,
  carries a handover race, and gives up after five minutes. Its one real
  advantage is that it keeps a per-Call queue, so lock-screen next/previous
  work — which ours structurally cannot.
- **Web Audio with a silent oscillator.** Rejected: iOS suspends the
  `AudioContext` on background and interrupts it on lock.
- **Web Push to wake the page.** Not a substitute — it reaches a listener who
  has stopped listening, rather than keeping audio flowing for one who has not.
