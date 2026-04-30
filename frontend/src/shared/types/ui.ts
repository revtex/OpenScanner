// Purely client-side view-state types.

// Avoid timer tracking
export interface AvoidEntry {
  talkgroupId: number;
  expiresAt: number; // unix ms timestamp, 0 = permanent
}
