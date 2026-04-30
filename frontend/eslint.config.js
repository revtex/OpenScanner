import js from "@eslint/js";
import tseslint from "typescript-eslint";
import reactHooks from "eslint-plugin-react-hooks";
import reactRefresh from "eslint-plugin-react-refresh";
import globals from "globals";

export default tseslint.config(
  { ignores: ["dist", "coverage", "tmp", "sw.ts"] },
  {
    extends: [js.configs.recommended, ...tseslint.configs.recommended],
    files: ["**/*.{ts,tsx}"],
    languageOptions: {
      globals: globals.browser,
      ecmaVersion: 2022,
      sourceType: "module",
    },
    plugins: {
      "react-hooks": reactHooks,
      "react-refresh": reactRefresh,
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      "react-refresh/only-export-components": [
        "warn",
        { allowConstantExport: true },
      ],

      // Unused variables — allow _-prefixed for intentional discards.
      "@typescript-eslint/no-unused-vars": [
        "error",
        {
          argsIgnorePattern: "^_",
          varsIgnorePattern: "^_",
          caughtErrorsIgnorePattern: "^_",
          destructuredArrayIgnorePattern: "^_",
        },
      ],

      // Type-only imports keep runtime bundles lean and prevent circular import cycles.
      "@typescript-eslint/consistent-type-imports": [
        "error",
        {
          prefer: "type-imports",
          fixStyle: "inline-type-imports",
          disallowTypeAnnotations: false,
        },
      ],
      "@typescript-eslint/no-import-type-side-effects": "error",

      // Catch sloppy type-system usage beyond the default `recommended` tier.
      "@typescript-eslint/no-non-nullable-type-assertion-style": "off",
      "@typescript-eslint/prefer-as-const": "error",
      "@typescript-eslint/no-duplicate-enum-values": "error",
      "@typescript-eslint/no-inferrable-types": "error",
      "@typescript-eslint/no-for-in-array": "error",

      // Core JS hygiene.
      eqeqeq: ["error", "always", { null: "ignore" }],
      "no-var": "error",
      "prefer-const": "error",
      "no-debugger": "error",
      "no-console": ["warn", { allow: ["warn", "error", "info"] }],
      "no-implicit-globals": "error",
      "no-throw-literal": "error",
      "no-unreachable-loop": "error",
      "no-self-compare": "error",
      "no-template-curly-in-string": "error",
      "no-promise-executor-return": "error",
      "require-atomic-updates": "off", // too many false positives with async effects

      // Bulletproof-style feature boundaries. See docs/plans/frontend-features-migration-plan.md.
      // Direction: pages/ → features/ → shared/ → app/. Sibling features may only cross via barrels.
      "no-restricted-imports": [
        "error",
        {
          patterns: [
            {
              group: ["@/features/*/*", "!@/features/*/index"],
              message:
                "Import from a feature's public barrel only (e.g. @/features/scanner). Reaching into a feature's internals is forbidden.",
            },
            {
              group: ["@/features/admin/*/*", "!@/features/admin/*/index"],
              message:
                "Admin sub-features are opaque to each other. Import from the sub-feature barrel (e.g. @/features/admin/users) only.",
            },
          ],
        },
      ],
    },
  },
  {
    // shared/ is the lowest layer — it cannot depend on features/.
    files: ["src/shared/**/*.{ts,tsx}"],
    rules: {
      "no-restricted-imports": [
        "error",
        {
          patterns: [
            {
              group: ["@/features/*", "@/pages/*"],
              message:
                "shared/ cannot depend on features/ or pages/. Move the symbol up to the feature, or down into shared/ if it's truly cross-feature.",
            },
          ],
        },
      ],
    },
  },
  {
    files: ["src/main.tsx"],
    rules: {
      "react-refresh/only-export-components": "off",
    },
  },
  {
    // Tests use vitest globals and relax type strictness.
    files: ["**/*.test.{ts,tsx}", "src/test-setup.ts"],
    rules: {
      "@typescript-eslint/no-explicit-any": "off",
      "@typescript-eslint/no-non-null-assertion": "off",
      "no-console": "off",
    },
  },
);
