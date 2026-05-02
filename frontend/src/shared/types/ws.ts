// WS framing types — native v1 protocol (JSON-object frames).
// See docs/plans/native-api-design-plan.md §4.2.

import type { Call, TranscriptionSegment } from "@/features/scanner";
import type { SystemConfig } from "@/shared/types/config";

// Connection status for WS
export type ConnectionStatus = "connecting" | "connected" | "disconnected";

// ── Listener WS — inbound (server → client) ─────────────────────────────

export interface WsWelcome {
  type: "connection.welcome";
  version?: string;
  branding?: string;
  email?: string;
}

export interface WsScannerConfig {
  type: "scanner.config";
  config: {
    systems?: SystemConfig[];
    branding?: string;
    email?: string;
    version?: string;
    time12hFormat?: boolean | string;
    showListenersCount?: boolean | string;
    shareableLinks?: boolean | string;
    keypadBeeps?: string;
    transcriptionEnabled?: boolean | string;
    liveTranscriptDisplay?: boolean | string;
  };
}

export interface WsCallNew {
  type: "call.new";
  call: Call;
}

export interface WsCallTranscript {
  type: "call.transcript";
  callId: number;
  text: string;
  segments?: TranscriptionSegment[];
}

export interface WsSessionExpired {
  type: "session.expired";
}

export interface WsListenerCount {
  type: "listener.count";
  count: number;
}

export interface WsFeedMapSnapshot {
  type: "listener.feedMap.snapshot";
  feedMap: Record<string, unknown>;
}

export interface WsConnectionRejected {
  type: "connection.rejected";
  reason: string;
}

export type WsListenerInbound =
  | WsWelcome
  | WsScannerConfig
  | WsCallNew
  | WsCallTranscript
  | WsSessionExpired
  | WsListenerCount
  | WsFeedMapSnapshot
  | WsConnectionRejected;

// ── Listener WS — outbound (client → server) ────────────────────────────

export interface WsFeedMapUpdate {
  type: "listener.feedMap.update";
  feedMap: Record<string, unknown>;
}

export type WsListenerOutbound = WsFeedMapUpdate;

// ── Admin WS — inbound (server → client) ────────────────────────────────

export interface WsAdminEvent {
  type: "admin.event";
  topic: string;
  at: number;
  data: unknown;
}

// Mirrors the REST error envelope shape (plan §7).
export interface WsAdminError {
  code: string;
  message: string;
  details?: unknown;
}

export interface WsAdminResponseOk {
  type: "admin.response";
  reqId: string;
  ok: true;
  data: unknown;
}

export interface WsAdminResponseErr {
  type: "admin.response";
  reqId: string;
  ok: false;
  error: WsAdminError;
}

export type WsAdminResponse = WsAdminResponseOk | WsAdminResponseErr;

export type WsAdminInbound = WsAdminEvent | WsAdminResponse | WsSessionExpired;

// ── Admin WS — outbound (client → server) ───────────────────────────────

export interface WsAdminRequest {
  type: "admin.request";
  reqId: string;
  op: string;
  params?: Record<string, unknown>;
}

export type WsAdminOutbound = WsAdminRequest;
