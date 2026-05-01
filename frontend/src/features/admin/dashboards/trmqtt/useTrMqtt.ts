// Selector hook + bundled mutations for the Trunk Recorder dashboards.
import { useAppSelector } from "@/app/store";
import type { TrMqttState } from "./trMqttSlice";

export interface TrMqttRoot {
  trMqtt: TrMqttState;
}

export function useTrMqttState(): TrMqttState {
  return useAppSelector((s: TrMqttRoot) => s.trMqtt);
}

export function useTrInstanceConnection(id: number) {
  return useAppSelector(
    (s: TrMqttRoot) =>
      s.trMqtt.instances[id] ?? { connected: false, lastError: undefined },
  );
}

export function useTrRates(id: number) {
  return useAppSelector((s: TrMqttRoot) => s.trMqtt.rates[id] ?? []);
}

export function useTrUnitEvents(id: number) {
  return useAppSelector((s: TrMqttRoot) => s.trMqtt.unitEvents[id] ?? []);
}

export function useTrMessages(id: number) {
  return useAppSelector((s: TrMqttRoot) => s.trMqtt.trunkingMessages[id] ?? []);
}

export function useTrLagWarning(id: number, withinMs = 5000): boolean {
  return useAppSelector((s: TrMqttRoot) => {
    const t = s.trMqtt.lagWarning[id];
    if (!t) return false;
    return Date.now() - t < withinMs;
  });
}
