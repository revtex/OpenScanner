// Top-level Trunk Recorder MQTT panel — internal sub-tabs (Instances /
// Dashboard / Units / Messages). Selected instance is held locally; the
// list view persists across sub-tabs.
import { useState } from "react";
import InstancesPanel from "./InstancesPanel";
import DashboardView from "./DashboardView";
import UnitsView from "./UnitsView";
import TrunkingMessagesView from "./TrunkingMessagesView";
import { useListTrInstancesQuery } from "./trMqttApi";
import type { TrInstance } from "./types";

type SubTab = "instances" | "dashboard" | "units" | "messages";

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
      <div role="tablist" className="tabs tabs-boxed bg-base-200">
        <button
          role="tab"
          className={`tab ${tab === "instances" ? "tab-active" : ""}`}
          onClick={() => setTab("instances")}
        >
          Instances
        </button>
        <button
          role="tab"
          className={`tab ${tab === "dashboard" ? "tab-active" : ""}`}
          onClick={() => setTab("dashboard")}
          disabled={!selected}
        >
          Dashboard
        </button>
        <button
          role="tab"
          className={`tab ${tab === "units" ? "tab-active" : ""}`}
          onClick={() => setTab("units")}
          disabled={!selected}
        >
          Units
        </button>
        <button
          role="tab"
          className={`tab ${tab === "messages" ? "tab-active" : ""}`}
          onClick={() => setTab("messages")}
          disabled={!selected}
        >
          Messages
        </button>
      </div>

      {tab === "instances" && <InstancesPanel onSelect={selectAndOpen} />}
      {tab === "dashboard" && selected && <DashboardView instance={selected} />}
      {tab === "units" && selected && <UnitsView instance={selected} />}
      {tab === "messages" && selected && (
        <TrunkingMessagesView instance={selected} />
      )}
      {tab !== "instances" && !selected && (
        <div className="text-base-content/60">No instance selected.</div>
      )}
    </div>
  );
}
