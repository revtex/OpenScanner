---
name: resolving-merge-conflicts
description: "Use when you need to resolve an in-progress git merge/rebase conflict."
---

1. **See the current state** of the merge/rebase. Check git history, and the conflicting files.

2. **Find the primary sources** for each conflict. Understand deeply why each change was made, and what the original intent was. Read the commit messages, check the PRs (`gh pr view`), check original issues.

3. **Resolve each hunk.** Preserve both intents where possible. Where incompatible, pick the one matching the merge's stated goal and note the trade-off. Do **not** invent new behaviour. Always resolve; never `--abort`.

4. Discover the project's **automated checks** and run them:

   ```bash
   cd backend  && go vet ./... && go build ./... && go test ./...
   cd frontend && npx tsc --noEmit && npx vitest --run
   make lint
   ```

   Fix anything the merge broke.

5. **Finish the merge/rebase.** Stage everything and commit. If rebasing, continue the rebase process until all commits are rebased.

Two conflicts specific to this repo:

- **Migrations are append-only.** A conflict on a numbered file in `backend/migrations/`
  is resolved by renumbering the incoming one to the next free number, never by
  rewriting a migration that has already been committed.
- **sqlc output is generated.** Never hand-resolve a conflict inside `backend/internal/db/`;
  resolve the `.sql` files, then re-run `make generate` and take its output.
