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
  branding: string;
  email: string;
  version: string;
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
  | "PIN"
  | "VER"
  | "TRN";

// Setup status from GET /api/setup/status
export interface SetupStatus {
  needsSetup: boolean;
  publicAccess: boolean;
}

// Auth login response
export interface LoginResponse {
  token: string;
  role: string;
  username: string;
  passwordNeedChange: boolean;
}

// For avoid timer tracking
export interface AvoidEntry {
  talkgroupId: number;
  expiresAt: number; // unix ms timestamp, 0 = permanent
}

// Connection status for WS
export type ConnectionStatus = "connecting" | "connected" | "disconnected";
