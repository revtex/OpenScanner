// Top-level Trunk Recorder MQTT panel — internal sub-tabs spanning the
// full plugin schema. Each tab is a focused dashboard so heavy frames
// (recorders / messages) don't crowd the live overview.
import { useState } from "react";
import InstancesPanel from "./InstancesPanel";
import DashboardView from "./DashboardView";
import CallsView from "./CallsView";
import RecordersView from "./RecordersView";
import SystemsView from "./SystemsView";
import UnitsView from "./UnitsView";
import TrunkingMessagesView from "./TrunkingMessagesView";
import ConfigView from "./ConfigView";
import { useListTrInstancesQuery } from "./trMqttApi";
import type { TrInstance } from "./types";

type SubTab =
  | "instances"
  | "dashboard"
  | "calls"
  | "recorders"
  | "systems"
  | "units"
  | "messages"
  | "config";

const TABS: { key: SubTab; label: string }[] = [
  { key: "instances", label: "Instances" },
  { key: "dashboard", label: "Dashboard" },
  { key: "calls", label: "Calls" },
  { key: "recorders", label: "Recorders" },
  { key: "systems", label: "Systems" },
  { key: "units", label: "Units" },
  { key: "messages", label: "Messages" },
  { key: "config", label: "Config" },
];

export default function TrMqttPanel() {
  const { data: instances = [], error } = useListTrInstancesQuery();
  const [tab, setTab] = useState<SubTab>("instances");
  const [selectedId, setSelectedId] = useState<number | null>(null);

  const selected =
    instances.find((i) => i.id === selectedId) ??
    (instances.length > 0 ? instances[0] : null);

  function selectAndOpen(inst: TrInstance) {
    setSelectedId(inst.id);
    setTab("dashboard");
  }

  if (error) {
    return (
      <div className="alert alert-warning">
        <span>
          Trunk Recorder MQTT integration is not enabled. Open{" "}
          <span className="font-semibold">
            Admin → Options → Trunk Recorder MQTT
          </span>{" "}
          and toggle{" "}
          <span className="font-mono">Enable Trunk Recorder MQTT</span> on to
          use this dashboard.
        </span>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div role="tablist" className="tabs tabs-boxed bg-base-200 flex-wrap">
        {TABS.map(({ key, label }) => (
          <button
            key={key}
            role="tab"
            className={`tab ${tab === key ? "tab-active" : ""}`}
            onClick={() => setTab(key)}
            disabled={key !== "instances" && !selected}
          >
            {label}
          </button>
        ))}
      </div>

      {tab === "instances" && <InstancesPanel onSelect={selectAndOpen} />}
      {tab === "dashboard" && selected && <DashboardView instance={selected} />}
      {tab === "calls" && selected && <CallsView instance={selected} />}
      {tab === "recorders" && selected && <RecordersView instance={selected} />}
      {tab === "systems" && selected && <SystemsView instance={selected} />}
      {tab === "units" && selected && <UnitsView instance={selected} />}
      {tab === "messages" && selected && (
        <TrunkingMessagesView instance={selected} />
      )}
      {tab === "config" && selected && <ConfigView instance={selected} />}
      {tab !== "instances" && !selected && (
        <div className="text-base-content/60">No instance selected.</div>
      )}
    </div>
  );
}
