// WS framing types.

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

// Connection status for WS
export type ConnectionStatus = "connecting" | "connected" | "disconnected";
