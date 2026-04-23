# Rulesets

GitHub ruleset definitions for branch and tag protection. These are the
source of truth for the repo's protection policy — committing them means
rules can be reviewed, diffed, and restored like code.

## Files

- **`protect-main.json`** — protects `refs/heads/main`:
  - No force push, no deletion, linear history only
  - All changes go through a PR
  - CI checks must pass: `Go tests`, `Frontend tests`, `CHANGELOG updated`
  - Stale reviews dismissed on new pushes
  - Merge methods allowed: merge commit or squash (no rebase — keeps the
    release history readable)
  - The `RepositoryRole` bypass actor (role id `5` = Admin) can merge
    without approvals but must still use a PR
- **`protect-tags.json`** — protects `refs/tags/v*`:
  - No force-update, no deletion — once a release tag is pushed it's
    immutable
  - Admin can bypass if you ever need to retag during a recovery
  - SemVer tag format (`vX.Y.Z`) is enforced by convention, not by the
    ruleset — GitHub's `tag_name_pattern` rule is not accepted on
    tag-target rulesets

## Import

### Via the GitHub UI

1. Open repo → **Settings** → **Rules** → **Rulesets**
2. Click **New ruleset** → **Import a ruleset**
3. Paste the contents of the JSON file
4. Review and save

### Via the API

```bash
# Replace OWNER/REPO.
gh api --method POST \
  -H "Accept: application/vnd.github+json" \
  /repos/OWNER/REPO/rulesets \
  --input .github/rulesets/protect-main.json

gh api --method POST \
  -H "Accept: application/vnd.github+json" \
  /repos/OWNER/REPO/rulesets \
  --input .github/rulesets/protect-tags.json
```

## Updating a ruleset

```bash
# List existing rulesets to find the id:
gh api /repos/OWNER/REPO/rulesets

# Update in place:
gh api --method PUT \
  -H "Accept: application/vnd.github+json" \
  /repos/OWNER/REPO/rulesets/<id> \
  --input .github/rulesets/protect-main.json
```

## Notes

- Status-check **context names** in `protect-main.json` must exactly
  match the `name:` field of each job in `.github/workflows/ci.yml`.
  Current matches: `backend` → `Go tests`, `frontend` → `Frontend tests`,
  `changelog` → `CHANGELOG updated`.
- The `actor_id: 5` in `bypass_actors` corresponds to the built-in
  **Admin** repository role. To look up other actor ids (teams,
  specific users, apps), use:

  ```bash
  gh api /repos/OWNER/REPO/rulesets/rule-suites
  gh api /orgs/ORG/teams
  ```

- Changing these files in the repo does **not** automatically update the
  live rulesets on GitHub — you must re-import or PUT via the API.
