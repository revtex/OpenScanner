// Internal barrel for the admin chrome (slice, hooks, providers).
// Sub-features under features/admin/<x>/ import from here. The leading
// underscore marks this as not-a-feature-itself; sibling sub-features
// may import _shell/, but _shell/ may not import sibling sub-features.
export * from "./adminSlice";
export * from "./useAdminWebSocket";
export * from "./useAdminWsOps";
export * from "./useNavigationGuard";
export * from "./useWsQuery";
