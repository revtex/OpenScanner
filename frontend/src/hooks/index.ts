// Barrel re-export for the hooks tree. Prefer specific imports
// (`@/hooks/shared/useTheme`) in new code; this barrel exists as a
// safety net for callers that want to grab a hook without thinking
// about which subfolder it lives in.
export * from "./admin";
export * from "./scanner";
export * from "./shared";
