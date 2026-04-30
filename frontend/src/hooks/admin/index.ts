// Transitional barrel — only the activity / logs hooks remain here until
// they move into features/admin/dashboards (per the TR-MQTT plan) and
// features/admin/logs (Phase 5). The chrome hooks (useAdminWebSocket,
// useAdminWsOps, useNavigationGuard, useWsQuery) moved to
// @/features/admin/_shell.
export * from "./useAdminActivity";
export * from "./useAdminLogs";
