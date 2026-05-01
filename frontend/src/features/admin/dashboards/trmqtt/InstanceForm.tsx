// Instance editor: drawer-hosted DaisyUI 5 form. Password tri-state mirrors
// the backend PATCH contract:
//   - mode "keep"  → omit `password` field entirely (existing kept)
//   - mode "clear" → send `password: ""` (cleared on server)
//   - mode "set"   → send `password: "<plaintext>"` (re-encrypted)
import { useState, type FormEvent } from "react";
import type { TrInstance } from "./types";

export interface InstanceFormValues {
  label: string;
  instanceId: string;
  brokerUrl: string;
  baseTopic: string;
  unitTopic: string;
  messageTopic: string;
  username: string;
  qos: number;
  tlsSkipVerify: boolean;
  enabled: boolean;
  passwordMode: "keep" | "clear" | "set";
  password: string;
}

const DEFAULTS: InstanceFormValues = {
  label: "",
  instanceId: "",
  brokerUrl: "tcp://localhost:1883",
  baseTopic: "trunk-recorder",
  unitTopic: "",
  messageTopic: "",
  username: "",
  qos: 0,
  tlsSkipVerify: false,
  enabled: true,
  passwordMode: "set",
  password: "",
};

function fromInstance(inst: TrInstance): InstanceFormValues {
  return {
    label: inst.label,
    instanceId: inst.instanceId,
    brokerUrl: inst.brokerUrl,
    baseTopic: inst.baseTopic,
    unitTopic: inst.unitTopic ?? "",
    messageTopic: inst.messageTopic ?? "",
    username: inst.username ?? "",
    qos: inst.qos,
    tlsSkipVerify: inst.tlsSkipVerify,
    enabled: inst.enabled,
    passwordMode: "keep",
    password: "",
  };
}

interface InstanceFormProps {
  editing?: TrInstance;
  submitting: boolean;
  serverError?: string | null;
  onSubmit: (values: InstanceFormValues) => void;
  onCancel: () => void;
}

export default function InstanceForm({
  editing,
  submitting,
  serverError,
  onSubmit,
  onCancel,
}: InstanceFormProps) {
  const [values, setValues] = useState<InstanceFormValues>(
    editing ? fromInstance(editing) : DEFAULTS,
  );
  const [error, setError] = useState<string | null>(null);
  const [editingId, setEditingId] = useState<number | undefined>(editing?.id);

  // Reset values when the editor switches between create / edit / different
  // instance. Compare by identity (id) rather than reference to avoid
  // cascading renders.
  if (editing?.id !== editingId) {
    setEditingId(editing?.id);
    setValues(editing ? fromInstance(editing) : DEFAULTS);
  }

  const isEdit = !!editing;
  const passwordModeOptions: Array<InstanceFormValues["passwordMode"]> = isEdit
    ? ["keep", "clear", "set"]
    : ["keep", "set"];

  function handleChange<K extends keyof InstanceFormValues>(
    key: K,
    value: InstanceFormValues[K],
  ) {
    setValues((v) => ({ ...v, [key]: value }));
  }

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!values.label.trim()) {
      setError("Label is required");
      return;
    }
    if (!values.instanceId.trim()) {
      setError("Instance ID is required (must match TR config.instance_id)");
      return;
    }
    if (!values.brokerUrl.trim()) {
      setError("Broker URL is required");
      return;
    }
    if (!values.baseTopic.trim()) {
      setError("Base topic is required");
      return;
    }
    setError(null);
    onSubmit(values);
  }

  return (
    <form
      onSubmit={handleSubmit}
      className="space-y-3"
      data-testid="tr-instance-form"
    >
      <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
        <label className="flex flex-col gap-1">
          <span className="label-text">Label *</span>
          <input
            className="input input-bordered"
            value={values.label}
            onChange={(e) => handleChange("label", e.target.value)}
            required
            data-testid="tr-form-label"
          />
        </label>
        <label className="flex flex-col gap-1">
          <span className="label-text">Instance ID *</span>
          <input
            className="input input-bordered"
            value={values.instanceId}
            onChange={(e) => handleChange("instanceId", e.target.value)}
            placeholder="config.instance_id from trunk-recorder"
            required
          />
        </label>
        <label className="flex flex-col gap-1 md:col-span-2">
          <span className="label-text">Broker URL *</span>
          <input
            className="input input-bordered font-mono text-sm"
            value={values.brokerUrl}
            onChange={(e) => handleChange("brokerUrl", e.target.value)}
            placeholder="tcp://host:1883"
            required
          />
        </label>
        <label className="flex flex-col gap-1">
          <span className="label-text">Base topic *</span>
          <input
            className="input input-bordered font-mono text-sm"
            value={values.baseTopic}
            onChange={(e) => handleChange("baseTopic", e.target.value)}
            required
          />
        </label>
        <label className="flex flex-col gap-1">
          <span className="label-text">Unit topic</span>
          <input
            className="input input-bordered font-mono text-sm"
            value={values.unitTopic}
            onChange={(e) => handleChange("unitTopic", e.target.value)}
            placeholder="optional"
          />
        </label>
        <label className="flex flex-col gap-1">
          <span className="label-text">Message topic</span>
          <input
            className="input input-bordered font-mono text-sm"
            value={values.messageTopic}
            onChange={(e) => handleChange("messageTopic", e.target.value)}
            placeholder="optional"
          />
        </label>
        <label className="flex flex-col gap-1">
          <span className="label-text">QoS</span>
          <select
            className="select select-bordered"
            value={values.qos}
            onChange={(e) => handleChange("qos", Number(e.target.value))}
          >
            <option value={0}>0</option>
            <option value={1}>1</option>
            <option value={2}>2</option>
          </select>
        </label>
        <label className="flex flex-col gap-1">
          <span className="label-text">Username</span>
          <input
            className="input input-bordered"
            value={values.username}
            onChange={(e) => handleChange("username", e.target.value)}
            autoComplete="off"
          />
        </label>
        <div className="flex flex-col gap-1 md:col-span-2">
          <span className="label-text">Password</span>
          <div className="join">
            {passwordModeOptions.map((mode) => (
              <button
                key={mode}
                type="button"
                className={`btn btn-sm join-item ${
                  values.passwordMode === mode ? "btn-primary" : ""
                }`}
                onClick={() => handleChange("passwordMode", mode)}
                data-testid={`tr-form-pwmode-${mode}`}
              >
                {mode === "keep"
                  ? "Keep existing"
                  : mode === "clear"
                    ? "Clear"
                    : "Set new"}
              </button>
            ))}
          </div>
          {values.passwordMode === "set" && (
            <input
              type="password"
              className="input input-bordered mt-2"
              value={values.password}
              onChange={(e) => handleChange("password", e.target.value)}
              autoComplete="new-password"
              data-testid="tr-form-password"
            />
          )}
        </div>
        <label className="label cursor-pointer justify-start gap-2">
          <input
            type="checkbox"
            className="checkbox"
            checked={values.tlsSkipVerify}
            onChange={(e) => handleChange("tlsSkipVerify", e.target.checked)}
          />
          <span className="label-text">TLS skip verify (insecure)</span>
        </label>
        <label className="label cursor-pointer justify-start gap-2">
          <input
            type="checkbox"
            className="checkbox"
            checked={values.enabled}
            onChange={(e) => handleChange("enabled", e.target.checked)}
          />
          <span className="label-text">Enabled</span>
        </label>
      </div>

      {(error || serverError) && (
        <div className="alert alert-error text-sm" role="alert">
          {error ?? serverError}
        </div>
      )}

      <div className="flex justify-end gap-2 pt-2">
        <button
          type="button"
          className="btn btn-ghost"
          onClick={onCancel}
          disabled={submitting}
        >
          Cancel
        </button>
        <button
          type="submit"
          className="btn btn-primary"
          disabled={submitting}
          data-testid="tr-form-submit"
        >
          {submitting ? "Saving…" : isEdit ? "Save" : "Create"}
        </button>
      </div>
    </form>
  );
}
