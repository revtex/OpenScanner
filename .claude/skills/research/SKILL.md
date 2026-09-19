---
name: research
description: Investigate a question against high-trust primary sources and capture the findings as a Markdown file in the repo. Use when the user wants a topic researched, docs or API facts gathered, or reading legwork delegated to a background agent.
---

Spin up a **background agent** to do the research, so you keep working while it reads.

Its job:

1. Investigate the question against **primary sources** — official docs, source code, specs, first-party APIs — not a secondary write-up of them. Follow every claim back to the source that owns it.
2. Write the findings to a single Markdown file in `docs/research/`, named for the question (`ios-background-audio.md`, not `notes.md`), citing each claim's source inline.
3. Report back what it wrote and where.

Rules that make the output worth keeping:

- **Commit the file.** A research note that only exists in a comment reference is
  worthless to the next reader — if the code cites it, the file has to be there.
- **Separate verified from claimed.** A vendor's assertion, another project's code
  comment, and a spec paragraph carry different weight. Say which one a claim came
  from, and say plainly when a claim could not be confirmed from a primary source.
- **A real-device or runtime observation outranks documentation.** Where the two
  disagree, record both and say which was tested.
- `docs/research/` is tracked; `docs/plans/` is gitignored scratch. Research goes in
  the former, and nothing tracked may reference the latter.
