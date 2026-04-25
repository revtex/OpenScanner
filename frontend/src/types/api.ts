// REST request/response envelopes (non-admin).

// Setup status from GET /api/setup/status
export interface SetupStatus {
  needsSetup: boolean;
  publicAccess: boolean;
}
