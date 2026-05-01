import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import InstanceForm from "./InstanceForm";
import { valuesToCreateBody, valuesToUpdateBody } from "./instanceFormBody";
import type { TrInstance } from "./types";

const baseInstance: TrInstance = {
  id: 7,
  label: "trunk-1",
  instanceId: "tr1",
  brokerUrl: "tcp://broker:1883",
  baseTopic: "trunk-recorder",
  unitTopic: undefined,
  messageTopic: undefined,
  username: "user",
  hasPassword: true,
  tlsSkipVerify: false,
  qos: 1,
  enabled: true,
  status: "connected",
  createdAt: 0,
  updatedAt: 0,
};

describe("InstanceForm", () => {
  it("create mode submits with no password field when mode=keep", async () => {
    const onSubmit = vi.fn();
    render(
      <InstanceForm
        submitting={false}
        onSubmit={onSubmit}
        onCancel={() => {}}
      />,
    );
    fireEvent.change(screen.getByTestId("tr-form-label"), {
      target: { value: "lab" },
    });
    fireEvent.change(
      screen.getByPlaceholderText("config.instance_id from trunk-recorder"),
      { target: { value: "tr-x" } },
    );
    fireEvent.click(screen.getByTestId("tr-form-submit"));
    expect(onSubmit).toHaveBeenCalledOnce();
    const body = valuesToCreateBody(onSubmit.mock.calls[0][0]);
    expect(body.password).toBeUndefined();
    expect(body.label).toBe("lab");
    expect(body.instanceId).toBe("tr-x");
  });

  it("edit mode 'clear' password results in empty string in update body", async () => {
    const onSubmit = vi.fn();
    render(
      <InstanceForm
        editing={baseInstance}
        submitting={false}
        onSubmit={onSubmit}
        onCancel={() => {}}
      />,
    );
    fireEvent.click(screen.getByTestId("tr-form-pwmode-clear"));
    fireEvent.click(screen.getByTestId("tr-form-submit"));
    const body = valuesToUpdateBody(onSubmit.mock.calls[0][0]);
    expect(body.password).toBe("");
  });

  it("edit mode 'set' includes plaintext password", async () => {
    const onSubmit = vi.fn();
    render(
      <InstanceForm
        editing={baseInstance}
        submitting={false}
        onSubmit={onSubmit}
        onCancel={() => {}}
      />,
    );
    fireEvent.click(screen.getByTestId("tr-form-pwmode-set"));
    fireEvent.change(screen.getByTestId("tr-form-password"), {
      target: { value: "newpass" },
    });
    fireEvent.click(screen.getByTestId("tr-form-submit"));
    const body = valuesToUpdateBody(onSubmit.mock.calls[0][0]);
    expect(body.password).toBe("newpass");
  });

  it("edit mode 'keep' omits password entirely", async () => {
    const onSubmit = vi.fn();
    render(
      <InstanceForm
        editing={baseInstance}
        submitting={false}
        onSubmit={onSubmit}
        onCancel={() => {}}
      />,
    );
    fireEvent.click(screen.getByTestId("tr-form-submit"));
    const body = valuesToUpdateBody(onSubmit.mock.calls[0][0]);
    expect(body.password).toBeUndefined();
  });

  it("blocks submit when label is empty", () => {
    const onSubmit = vi.fn();
    render(
      <InstanceForm
        submitting={false}
        onSubmit={onSubmit}
        onCancel={() => {}}
      />,
    );
    // label is required attribute — browser-like jsdom blocks submit
    fireEvent.click(screen.getByTestId("tr-form-submit"));
    expect(onSubmit).not.toHaveBeenCalled();
  });
});
