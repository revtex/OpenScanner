# iOS background audio for a web scanner client

Investigated 2026-09-19 while deciding whether Squelch's BKGND design should
borrow anything from a competing implementation. Conclusion: no code change.
The decision this informs is [ADR 0004](../adr/0004-background-audio-server-stream.md).

## The constraint

iOS suspends a backgrounded page shortly after its audio stops, and a suspended
page cannot advance its own queue. A scanner is silent between calls, so a
per-call player dies at the first lull after the screen locks.

There are two ways out: never let the audio stop (client-side), or never let
the *stream* stop (server-side). Squelch does the second.

## How radio-scout does the first

[FxllenCode/radio-scout](https://github.com/FxllenCode/radio-scout) (Rust +
PWA, GPL-3.0) advertises working iOS lock-screen audio. Read at commit as of
2026-09-17. Mechanism, from its source:

- **One `<audio>` element, source swapped.** `client/src/components/CallPlayer.tsx`:
  `src={bridging ? keepAliveLoopUrl() : current?.audioUrl}` with `loop={bridging}`.
- **A generated near-silent WAV**, `client/src/lib/silence.ts`: 1 second, 8 kHz,
  mono 16-bit, built as a ±1-LSB 100 Hz square (≈ −90 dBFS). Their stated
  reasoning: WebKit's `computeCanProduceAudio()` checks only playing / has audio
  track / not muted / volume ≠ 0 and never inspects sample values, so digital
  zero would pass *at that layer*, but `mediaserverd`'s behaviour below WebKit is
  undocumented. They deliberately do not attenuate with `volume` or `muted`,
  since muting is the documented way to tell WebKit "I am not audible".
- **Handover 300 ms before the end**, not on `ended` (`HANDOVER = 0.3`). Their
  comment cites WebKit bug 261858: an element reaching `ended` with nothing
  queued is the moment iOS releases a backgrounded PWA, and after that there is
  no JS left to start the keep-alive.
- **A five-minute budget** (`KEEP_ALIVE_LIMIT_MS`), after which they stop
  holding the session open and fall back to Web Push.
- **`navigator.audioSession.type = 'playback'`** (iOS 16.4+).
- **Media Session handlers rebound on every foregrounding**, on the claim that
  iOS forgets them across a backgrounding.

**Caveat on their claims:** their code cites `docs/research/ios-gap-bridging-mechanism.md`
and `docs/research/ios-background-audio.md`. Neither file is committed to their
repository, so the evidence behind the WebKit assertions above is not
inspectable. Treat them as well-reasoned claims, not verified facts.

## What the specs actually say

The [W3C Audio Session spec](https://www.w3.org/TR/audio-session/) defines
`playback` as "used for video or music playback, podcasts, etc. They should not
mix with other playback audio", and `ambient` as "mixable with other types of
audio". `auto` "lets the user agent choose the best audio session type according
the use of audio by the web page". **The spec says nothing about backgrounding
for any type** — the background and silent-switch behaviour is WebKit's mapping
onto AVAudioSession categories, not a specified guarantee.

Reported behaviour ([Adactio](https://adactio.com/journal/19929), WebKit
[commit c393587](https://github.com/WebKit/WebKit/commit/c39358705b79ccf2da3b76a8be6334e7e3dfcfa6)):
`auto` starts out as ambient in Safari, which leaves audio subject to the
ringer switch; setting `playback` keeps audio playing when the phone is muted.

## Real-device results (Squelch, BKGND on)

These outrank the documentation and were the basis for changing nothing.

| Test | Date | Result |
| --- | --- | --- |
| Audio continues with screen locked | 2026-09-18 | Yes |
| Lock-screen pause works after backgrounding | 2026-09-19 | Yes — handlers survive |
| Silent Mode mutes the stream | 2026-09-19 | No — keeps playing |

The second result contradicts radio-scout's rebinding rationale for our case.
The third means `navigator.audioSession.type = 'playback'` would make explicit
something Safari already infers for a plain `<audio>` element playing a long
MPEG resource, with no observable change.

Note the device had no Ring/Silent switch (iPhone 15 Pro or later); Silent Mode
was engaged via the Action Button.

## Why the difference in risk

radio-scout's element drops to a −90 dBFS loop during gaps, which is plausibly
the kind of signal an inference heuristic could categorise as ambient — so they
have more reason to set the category explicitly than we do. Our element plays
continuous real audio, which is the case Safari's inference has always handled
as media playback.
