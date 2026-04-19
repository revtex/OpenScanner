// Call data from WS CAL event or search results
export interface Call {
  id: number;
  audioName: string;
  audioType: string;
  dateTime: number; // unix timestamp
  systemId: number; // radio system ID
  system: number; // DB system ID
  talkgroupId: number; // radio TG ID
  talkgroup: number; // DB TG ID
  frequency?: number; // Hz
  duration?: number; // ms
  source?: number; // unit ID
  sources?: string; // JSON array
  frequencies?: string; // JSON array
  patches?: string; // JSON array
  site?: string; // receiver site name
  channel?: string; // channel identifier
  decoder?: string; // decoder type (e.g. "P25 Phase 1")
  errorCount?: number; // P25 error count
  spikeCount?: number; // P25 spike count
  talkerAlias?: string; // DMR/P25 talker alias
  systemLabel?: string; // populated from config
  talkgroupLabel?: string; // populated from config
  talkgroupName?: string; // populated from config
  talkgroupTag?: string; // populated from config
  talkgroupGroup?: string; // populated from config
  talkgroupLedColor?: string; // CSS color for LED
  transcript?: string;
  audioUrl?: string; // object URL for audio playback
}

// System/talkgroup config from CFG event
export interface SystemConfig {
  id: number;
  systemId: number;
  label: string;
  talkgroups: TalkgroupConfig[];
}

export interface TalkgroupConfig {
  id: number;
  talkgroupId: number;
  label: string;
  name: string;
  tag: string;
  group: string;
  ledColor: string; // CSS color string
  frequency?: number;
}

// Scanner configuration from CFG/VER events
export interface ScannerConfig {
  systems: SystemConfig[];
  branding?: string;
  email?: string;
  version?: string;
  time12hFormat: boolean;
  showListenersCount: boolean;
  playbackGoesLive: boolean;
  shareableLinks: boolean;
  keypadBeeps: string;
}

// WS message: JSON array [command, payload?, flags?]
export type WsCommand =
  | "CAL"
  | "CFG"
  | "XPR"
  | "LCL"
  | "LSC"
  | "LFM"
  | "MAX"
  | "VER"
  | "TRN"
  | "ADM_EVT"
  | "ADM_REQ"
  | "ADM_RES";

// Setup status from GET /api/setup/status
export interface SetupStatus {
  needsSetup: boolean;
  publicAccess: boolean;
}

// Auth login response
export interface LoginResponse {
  token: string;
  user: {
    id: number;
    username: string;
    role: string;
  };
  passwordNeedChange: boolean;
}

// Auth refresh response
export interface RefreshResponse {
  token: string;
  user: {
    id: number;
    username: string;
    role: string;
  };
}

// For avoid timer tracking
export interface AvoidEntry {
  talkgroupId: number;
  expiresAt: number; // unix ms timestamp, 0 = permanent
}

// Connection status for WS
export type ConnectionStatus = "connecting" | "connected" | "disconnected";

// ─── Admin resource types ───

export interface AdminUser {
  id: number;
  username: string;
  role: "admin" | "listener";
  disabled: number; // 0 or 1
  systemsJson: string | null;
  expiration: number | null; // unix timestamp
  limit: number | null; // concurrent connection limit
  createdAt: number;
  updatedAt: number;
}

export interface AdminSystem {
  id: number;
  systemId: number;
  label: string;
  autoPopulate: number;
  blacklistsJson: string | null;
  led: string | null;
  order: number;
}

export interface AdminTalkgroup {
  id: number;
  systemId: number;
  talkgroupId: number;
  label: string | null;
  name: string | null;
  frequency: number | null;
  led: string | null;
  groupId: number | null;
  tagId: number | null;
  order: number;
}

export interface AdminUnit {
  id: number;
  systemId: number;
  unitId: number;
  label: string | null;
  order: number;
}

export interface AdminGroup {
  id: number;
  label: string;
}

export interface AdminTag {
  id: number;
  label: string;
}

export interface AdminApiKey {
  id: number;
  fingerprint: string;
  ident: string | null;
  disabled: number;
  systemsJson: string | null;
  callRateLimit: number | null;
  order: number;
}

export interface AdminApiKeyCreateResponse extends AdminApiKey {
  createdKey: string;
}

export interface AdminDirwatch {
  id: number;
  directory: string;
  type: string;
  mask: string | null;
  extension: string | null;
  frequency: number | null;
  delay: number | null;
  deleteAfter: number;
  usePolling: number;
  disabled: number;
  systemId: number | null;
  talkgroupId: number | null;
  order: number;
}

export interface AdminDownstream {
  id: number;
  url: string;
  apiKey: string;
  systemsJson: string | null;
  disabled: number;
  order: number;
}

export interface AdminWebhook {
  id: number;
  url: string;
  type: string;
  secret: string | null;
  systemsJson: string | null;
  disabled: number;
  order: number;
}

export interface AdminSetting {
  key: string;
  value: string;
}

export interface Capabilities {
  ffmpeg: boolean;
  fdkAac: boolean;
  whisper: boolean;
}

export interface ConfigResponse {
  settings: AdminSetting[];
  capabilities: Capabilities;
}

export interface AdminLog {
  dateTime: number;
  level: string;
  message: string;
  attrs?: Record<string, string>;
}

// Password change request
export interface ChangePasswordRequest {
  currentPassword: string;
  newPassword: string;
}

// User create/update payload
export interface CreateUserPayload {
  username: string;
  password: string;
  role: "admin" | "listener";
  disabled?: number;
  systemsJson?: string | null;
  expiration?: number | null;
  limit?: number | null;
}

export interface UpdateUserPayload {
  username?: string;
  password?: string;
  role?: "admin" | "listener";
  disabled?: number;
  systemsJson?: string | null;
  expiration?: number | null;
  limit?: number | null;
}

export interface AdminDirMonitor {
  id: number;
  directory: string;
  type: string;
  mask: string | null;
  extension: string | null;
  frequency: number | null;
  delay: number | null;
  deleteAfter: number;
  usePolling: number;
  disabled: number;
  systemId: number | null;
  talkgroupId: number | null;
  order: number;
}

// --- RadioReference enrichment types ---

export interface RRTalkgroupCandidate {
  row: number;
  talkgroupId: number;
  label?: string;
  name?: string;
  group?: string;
  tag?: string;
  led?: string;
  order?: number;
}

export interface RRPreviewRow extends RRTalkgroupCandidate {
  matched: boolean;
  wouldUpdate: boolean;
  wouldUpdateFields: string[];
  skipReason?: string;
}

export interface RRRowError {
  row: number;
  reason: string;
}

export interface RRPreviewResponse {
  processed: number;
  matched: number;
  wouldUpdate: number;
  skipped: number;
  errors: number;
  rowErrors: RRRowError[];
  rows: RRPreviewRow[];
}

export interface RRApplyRequest {
  systemId: number;
  candidates: RRTalkgroupCandidate[];
  mergeMode: string;
  selectedFields: string[];
}

export interface RRApplyResponse {
  processed: number;
  matched: number;
  updated: number;
  skipped: number;
  errors: number;
  rowErrors: RRRowError[];
}
