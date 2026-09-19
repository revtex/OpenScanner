// Dashboards panel — sub-tab chrome hosting ActivityPanel and TrMqttPanel.
// Active sub-tab is persisted in `?dashTab=…` to mirror Admin.tsx URL state.
import { useSearchParams } from "react-router-dom";
import ActivityPanel from "./activity/ActivityPanel";
import { TrMqttPanel } from "./trmqtt";

type DashTab = "activity" | "trmqtt";

const VALID: readonly DashTab[] = ["activity", "trmqtt"] as const;

export default function DashboardsPanel() {
  const [params, setParams] = useSearchParams();
  const raw = params.get("dashTab");
  const active: DashTab = (VALID as readonly string[]).includes(raw ?? "")
    ? (raw as DashTab)
    : "activity";

  function switchTab(tab: DashTab) {
    const next = new URLSearchParams(params);
    next.set("dashTab", tab);
    setParams(next, { replace: true });
  }

  return (
    <div className="space-y-4">
      <div role="tablist" className="tabs tabs-lift">
        <button
          role="tab"
          className={`tab ${active === "activity" ? "tab-active" : ""}`}
          onClick={() => switchTab("activity")}
        >
          Activity
        </button>
        <button
          role="tab"
          className={`tab ${active === "trmqtt" ? "tab-active" : ""}`}
          onClick={() => switchTab("trmqtt")}
        >
          Trunk Recorder
        </button>
      </div>

      <div className="pt-2">
        {active === "activity" ? <ActivityPanel /> : <TrMqttPanel />}
      </div>
    </div>
  );
}
