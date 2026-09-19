// Instances table — list, edit, test, reconnect, delete TR MQTT instances.
import { useState } from "react";
import { Plus, Pencil, Trash2, Plug, Activity } from "lucide-react";
import {
  useListTrInstancesQuery,
  useCreateTrInstanceMutation,
  useUpdateTrInstanceMutation,
  useDeleteTrInstanceMutation,
  useTestTrInstanceMutation,
  useReconnectTrInstanceMutation,
} from "./trMqttApi";
import { useAppDispatch } from "@/app/store";
import { forgetInstance } from "./trMqttSlice";
import { useTrMqttState } from "./useTrMqtt";
import InstanceForm, { type InstanceFormValues } from "./InstanceForm";
import { valuesToCreateBody, valuesToUpdateBody } from "./instanceFormBody";
import type { TrInstance } from "./types";

function StatusBadge({
  inst,
  liveConnected,
  liveError,
}: {
  inst: TrInstance;
  liveConnected: boolean | undefined;
  liveError: string | undefined;
}) {
  if (!inst.enabled) {
    return <span className="badge badge-ghost badge-sm">disabled</span>;
  }
  if (liveConnected) {
    return <span className="badge badge-success badge-sm">connected</span>;
  }
  if (liveError) {
    return (
      <span className="badge badge-error badge-sm" title={liveError}>
        error
      </span>
    );
  }
  return <span className="badge badge-warning badge-sm">disconnected</span>;
}

export default function InstancesPanel({
  onSelect,
}: {
  onSelect?: (inst: TrInstance) => void;
}) {
  const { data: instances = [], isLoading, error } = useListTrInstancesQuery();
  const [createInst, { isLoading: creating }] = useCreateTrInstanceMutation();
  const [updateInst, { isLoading: updating }] = useUpdateTrInstanceMutation();
  const [deleteInst] = useDeleteTrInstanceMutation();
  const [testInst, { isLoading: testing }] = useTestTrInstanceMutation();
  const [reconnect] = useReconnectTrInstanceMutation();
  const trState = useTrMqttState();
  const dispatch = useAppDispatch();

  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<TrInstance | undefined>();
  const [serverError, setServerError] = useState<string | null>(null);
  const [testResult, setTestResult] = useState<{
    id: number;
    ok: boolean;
    msg: string;
  } | null>(null);

  function openCreate() {
    setEditing(undefined);
    setServerError(null);
    setFormOpen(true);
  }
  function openEdit(inst: TrInstance) {
    setEditing(inst);
    setServerError(null);
    setFormOpen(true);
  }

  async function handleSubmit(values: InstanceFormValues) {
    try {
      if (editing) {
        await updateInst({
          id: editing.id,
          body: valuesToUpdateBody(values),
        }).unwrap();
      } else {
        await createInst(valuesToCreateBody(values)).unwrap();
      }
      setServerError(null);
      setFormOpen(false);
    } catch (e: unknown) {
      // Surface the backend error so users can see why a save failed.
      const err = e as { data?: { error?: string }; status?: number };
      const msg =
        err?.data?.error ??
        (err?.status ? `HTTP ${err.status}` : "Save failed");
      setServerError(msg);
      console.error("save failed", e);
    }
  }

  async function handleDelete(inst: TrInstance) {
    if (!window.confirm(`Delete TR instance "${inst.label}"?`)) return;
    await deleteInst(inst.id).unwrap();
    dispatch(forgetInstance(inst.id));
  }

  async function handleTest(inst: TrInstance) {
    try {
      const res = await testInst(inst.id).unwrap();
      setTestResult({
        id: inst.id,
        ok: res.ok,
        msg: res.ok ? "Broker reachable" : (res.error ?? "Test failed"),
      });
    } catch (e) {
      setTestResult({ id: inst.id, ok: false, msg: String(e) });
    }
  }

  if (error) {
    return (
      <div className="alert alert-warning">
        Trunk Recorder MQTT is not enabled or unavailable.
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex justify-between items-center">
        <h3 className="text-lg font-semibold">Trunk Recorder instances</h3>
        <button className="btn btn-primary btn-sm" onClick={openCreate}>
          <Plus className="w-4 h-4" /> Add instance
        </button>
      </div>

      {isLoading ? (
        <div
          className="loading loading-spinner"
          aria-label="Loading instances"
        />
      ) : instances.length === 0 ? (
        <div className="text-base-content/60">No instances configured yet.</div>
      ) : (
        <div className="overflow-x-auto">
          <table className="table table-sm">
            <thead>
              <tr>
                <th>Label</th>
                <th>Instance ID</th>
                <th>Broker</th>
                <th>Status</th>
                <th className="text-right">Actions</th>
              </tr>
            </thead>
            <tbody>
              {instances.map((inst) => {
                const live = trState.instances[inst.id];
                return (
                  <tr key={inst.id} data-testid={`tr-row-${inst.id}`}>
                    <td>
                      <button
                        className="link link-hover font-medium"
                        onClick={() => onSelect?.(inst)}
                      >
                        {inst.label}
                      </button>
                    </td>
                    <td className="font-mono text-xs">{inst.instanceId}</td>
                    <td className="font-mono text-xs">{inst.brokerUrl}</td>
                    <td>
                      <StatusBadge
                        inst={inst}
                        liveConnected={live?.connected}
                        liveError={live?.lastError}
                      />
                    </td>
                    <td className="text-right">
                      <div className="join">
                        <button
                          type="button"
                          className="btn btn-xs join-item"
                          onClick={() => handleTest(inst)}
                          disabled={testing}
                          title="Test broker connection"
                        >
                          <Plug className="w-3 h-3" /> Test
                        </button>
                        <button
                          type="button"
                          className="btn btn-xs join-item"
                          onClick={() => reconnect(inst.id)}
                          title="Force reconnect"
                          data-testid={`tr-reconnect-${inst.id}`}
                        >
                          <Activity className="w-3 h-3" /> Reconnect
                        </button>
                        <button
                          type="button"
                          className="btn btn-xs join-item"
                          onClick={() => openEdit(inst)}
                        >
                          <Pencil className="w-3 h-3" />
                        </button>
                        <button
                          type="button"
                          className="btn btn-xs btn-error join-item"
                          onClick={() => handleDelete(inst)}
                        >
                          <Trash2 className="w-3 h-3" />
                        </button>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {testResult && (
        <div
          className={`alert ${testResult.ok ? "alert-success" : "alert-error"} text-sm`}
          role="status"
        >
          <span>{testResult.msg}</span>
          <button
            type="button"
            className="btn btn-ghost btn-xs"
            onClick={() => setTestResult(null)}
          >
            Dismiss
          </button>
        </div>
      )}

      {formOpen && (
        <dialog open className="modal modal-open">
          <div className="modal-box max-w-3xl">
            <h3 className="font-bold text-lg mb-2">
              {editing ? `Edit "${editing.label}"` : "Add TR instance"}
            </h3>
            <InstanceForm
              editing={editing}
              submitting={creating || updating}
              serverError={serverError}
              onSubmit={handleSubmit}
              onCancel={() => {
                setServerError(null);
                setFormOpen(false);
              }}
            />
          </div>
          <button
            type="button"
            className="modal-backdrop"
            onClick={() => setFormOpen(false)}
            aria-label="Close"
          />
        </dialog>
      )}
    </div>
  );
}
