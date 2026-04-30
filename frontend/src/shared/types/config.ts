// Scanner / system / talkgroup configuration delivered via WS CFG/VER events.

export interface SystemConfig {
  id: number;
  systemId: number;
  label: string;
  ledColor: string;
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
  transcriptionEnabled: boolean;
  liveTranscriptDisplay: boolean;
}
