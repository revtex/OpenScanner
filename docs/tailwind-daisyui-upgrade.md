# Tailwind CSS v4 + DaisyUI v5 Upgrade Plan

> **Status: COMPLETED** — All migration steps have been applied, tests pass (145/145), production build succeeds.

## Why Upgrade?

- **Dev mode is slow**: Tailwind v3 + PostCSS compiles CSS lazily on first request. DaisyUI's theme generation adds ~3–5s delay before the UI renders in dev mode.
- **Tailwind v4** uses a Rust-based engine — CSS compilation is near-instant.
- **DaisyUI v5** is a stable release (5.5.19) with cleaner defaults (borders on by default, simplified classes).
- Since Vite is our bundler, we can use `@tailwindcss/vite` for the best performance — no PostCSS needed.

## Current Versions

| Package           | Current | Target     |
| ----------------- | ------- | ---------- |
| tailwindcss       | 3.4.17  | 4.2.2      |
| @tailwindcss/vite | —       | 4.2.2      |
| daisyui           | 4.12.24 | 5.5.19     |
| autoprefixer      | 10.4.21 | **remove** |
| postcss           | 8.5.4   | **remove** |

## Migration Steps

### Step 1: Update Dependencies

```bash
cd frontend
pnpm remove tailwindcss autoprefixer postcss
pnpm add -D tailwindcss@4.2.2 @tailwindcss/vite@4.2.2
pnpm add -D daisyui@5.5.19
```

### Step 2: Replace PostCSS with Vite Plugin

**Delete** `postcss.config.js` — no longer needed.

**Delete** `tailwind.config.ts` — config moves into CSS.

**Update** `vite.config.ts`:

```ts
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  // ...
});
```

### Step 3: Migrate CSS Entry Point

**Before** (`src/index.css`):

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

@layer components {
  .led-branding {
    letter-spacing: 2px;
  }
  .led-indicator {
    width: 24px;
    height: 12px;
  }
  .history-row {
    font-size: 11px;
    line-height: 21px;
  }
}
```

**After** (`src/index.css`):

```css
@import "tailwindcss";
@plugin "daisyui" {
  themes: false;
}

/* Custom themes */
@plugin "daisyui/theme" {
  name: "openscanner-dark";
  default: true;
  prefersdark: true;
  color-scheme: dark;
  --color-base-100: #121212;
  --color-base-200: #1e1e1e;
  --color-base-300: #2d2d2d;
  --color-base-content: #e0e0e0; /* maps to neutral-content */
  --color-primary: #00e676;
  --color-primary-content: #000000;
  --color-secondary: #ff9100;
  --color-accent: #29b6f6;
  --color-neutral: #1e1e1e;
  --color-neutral-content: #e0e0e0;
  --color-info: #29b6f6;
  --color-success: #00e676;
  --color-warning: #ffea00;
  --color-error: #ff1744;
}

@plugin "daisyui/theme" {
  name: "openscanner-light";
  default: false;
  color-scheme: light;
  --color-base-100: #ffffff;
  --color-base-200: #f5f5f5;
  --color-base-300: #e0e0e0;
  --color-base-content: #1e1e1e;
  --color-primary: #2e7d32;
  --color-primary-content: #ffffff;
  --color-secondary: #e65100;
  --color-accent: #0277bd;
  --color-neutral: #f5f5f5;
  --color-neutral-content: #1e1e1e;
  --color-info: #0277bd;
  --color-success: #2e7d32;
  --color-warning: #f9a825;
  --color-error: #c62828;
}

/* Custom utilities */
@utility led-branding {
  letter-spacing: 2px;
}

@utility led-indicator {
  width: 24px;
  height: 12px;
}

@utility history-row {
  font-size: 11px;
  line-height: 21px;
}

/* Restore cursor: pointer on buttons (TW4 changed default to cursor: default) */
@layer base {
  button:not(:disabled),
  [role="button"]:not(:disabled) {
    cursor: pointer;
  }
}
```

### Step 4: DaisyUI v5 Class Removals

These classes had borders as opt-in in v4; in v5, borders are the default. **Remove** them:

| Class to remove       | Est. occurrences | Files affected |
| --------------------- | ---------------- | -------------- |
| `input-bordered`      | ~45              | 12 files       |
| `select-bordered`     | ~18              | 7 files        |
| `textarea-bordered`   | ~5               | 5 files        |
| `file-input-bordered` | ~3               | 1 file         |

**Find/replace command:**

```bash
# Remove the class from className strings
grep -rn "input-bordered\|select-bordered\|textarea-bordered\|file-input-bordered" \
  --include="*.tsx" --include="*.ts" frontend/src/
```

### Step 5: DaisyUI v5 Structural Changes

#### `form-control` → `fieldset` (~63 occurrences)

The `form-control` class is removed in v5. It previously set `display: flex; flex-direction: column`. Options:

**Option A (minimal):** Replace `form-control` with `flex flex-col` — preserves layout with no DOM changes.

**Option B (semantic):** Refactor `<label class="form-control">` to `<fieldset class="fieldset">` with `<legend>` and `<label>` children.

**Recommendation:** Option A for speed. The layout is identical.

#### `label-text` / `label-text-alt` (~86 occurrences)

These classes are removed in v5. They set `font-size`, `color`, and `opacity`.

**Replace with:** `text-sm` (for label-text) and `text-xs opacity-60` (for label-text-alt).

### Step 6: Tailwind v4 Class Renames

| v3 class     | v4 class     | Occurrences |
| ------------ | ------------ | ----------- |
| `rounded-sm` | `rounded-xs` | 1           |

**Note:** The upgrade tool handles most TW4 renames automatically. Our codebase is clean — only 1 rename needed.

### Step 7: Tailwind v4 Behavior Changes

These don't require class changes but affect visual output:

1. **Default border color** changed from `gray-200` to `currentColor`. If any `border` usage looks wrong, add explicit `border-base-300` or similar.

2. **`hover:` variant** now only applies when device supports hover (`@media (hover: hover)`). Touch devices won't trigger hover styles. This is correct for our use case.

3. **Buttons default to `cursor: default`** instead of `cursor: pointer`. Handled in Step 3 CSS with a base layer rule.

## Files to Modify (Summary)

### Config files (delete/rewrite)

- `frontend/postcss.config.js` — **DELETE**
- `frontend/tailwind.config.ts` — **DELETE**
- `frontend/vite.config.ts` — add `@tailwindcss/vite` plugin
- `frontend/src/index.css` — full rewrite (themes + utilities)

### Component files (class removals)

High-impact (15+ changes):

- `src/components/admin/SystemsPanel.tsx`
- `src/components/admin/DirWatchPanel.tsx`
- `src/components/scanner/SearchPanel.tsx`
- `src/components/admin/UsersPanel.tsx`
- `src/components/admin/OptionsPanel.tsx`

Medium-impact (5–15 changes):

- `src/components/admin/WebhooksPanel.tsx`
- `src/components/admin/DownstreamsPanel.tsx`
- `src/components/admin/APIKeysPanel.tsx`
- `src/components/admin/LogsPanel.tsx`
- `src/components/admin/ToolsPanel.tsx`
- `src/components/scanner/LEDPanel.tsx`

Low-impact (1–4 changes):

- `src/pages/Setup.tsx`
- `src/pages/Login.tsx`

### Test files

- Test files with DaisyUI classes in mock renders may need updating after component changes.

## Execution Order

1. Install new packages, remove old ones
2. Delete `postcss.config.js` and `tailwind.config.ts`
3. Update `vite.config.ts` with `@tailwindcss/vite`
4. Rewrite `src/index.css` with new syntax + themes
5. Bulk find/replace: remove `-bordered` classes
6. Bulk find/replace: `form-control` → `flex flex-col`
7. Bulk find/replace: `label-text` → `text-sm`, `label-text-alt` → `text-xs opacity-60`
8. Single rename: `rounded-sm` → `rounded-xs`
9. `pnpm dev` — test in browser, fix any visual regressions
10. `pnpm build` — verify production build works
11. Run all tests

## Risk Assessment

| Risk                             | Impact | Mitigation                                                                         |
| -------------------------------- | ------ | ---------------------------------------------------------------------------------- |
| Theme colors look different      | Medium | Test both themes visually; DaisyUI v5 may interpret hex colors differently than v4 |
| `form-control` layout breaks     | Low    | Using `flex flex-col` is identical to what `form-control` did                      |
| Custom CSS `@layer` breaks       | Low    | Already handled — moving to `@utility`                                             |
| Test failures from class changes | Low    | Tests use DaisyUI classes in rendered output — update assertions if needed         |
