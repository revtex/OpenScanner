// Admin DTOs (mirrors of ADM/ADMRES payloads and admin REST envelopes).

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
  autoPopulateTalkgroups: number;
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

export interface AdminDownstream {
  id: number;
  url: string;
  hasApiKey: boolean;
  systemsJson: string | null;
  disabled: number;
  order: number;
}

export interface AdminDownstreamCreate {
  url: string;
  apiKey: string;
  systemsJson: string | null;
  disabled: number;
  order: number;
}

export interface AdminDownstreamUpdate {
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

// --- Shared Links (admin) ---

export interface SharedLinkAdmin {
  id: number;
  callId: number;
  token: string;
  createdAt: number;
  sharedBy: string;
  dateTime: number;
  duration: number;
  systemLabel: string;
  talkgroupLabel: string;
  talkgroupName: string;
  expiresAt: number | null;
}

// --- Server filesystem types ---

export interface ServerDirectoryEntry {
  name: string;
  path: string;
}

export interface ServerDirectoryListResponse {
  path: string;
  parent: string | null;
  directories: ServerDirectoryEntry[];
}

// --- Transcription types ---

export interface TranscriptionStatus {
  enabled: boolean;
  url: string;
  model: string;
  language: string;
  diarize: boolean;
  liveDisplay: boolean;
  connected: boolean;
}

export interface WhisperModel {
  id: string;
  object: string;
  path: string;
  created: number;
  owned_by: string;
}

export interface TranscriptionModelsResponse {
  object: string;
  models: WhisperModel[];
}

export interface TranscriptionStats {
  total: number;
  recent24h: number;
  avgDurationMs: number;
  minDurationMs: number;
  maxDurationMs: number;
  queueDepth: number;
  poolEnabled: boolean;
  byLanguage: { language: string; count: number }[];
  byModel: { model: string; count: number }[];
}
