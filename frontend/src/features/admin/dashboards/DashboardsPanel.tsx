// Dashboards panel — currently a thin wrapper around ActivityPanel.
// When the TR-MQTT integration lands (see docs/plans/tr-mqtt-plan.md),
// this becomes a sub-tab chrome (DaisyUI 5 tabs) hosting ActivityPanel
// and the new TrMqttPanel side-by-side. Keeping the wrapper now means
// that future change is purely additive — no route rename, no folder
// regroup.
import ActivityPanel from "./activity/ActivityPanel";

export default function DashboardsPanel() {
  return <ActivityPanel />;
}
