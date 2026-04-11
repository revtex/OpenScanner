import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { ControlToolbar } from "@/components/scanner/ControlToolbar";
import type { AvoidEntry } from "@/types";

function defaultProps() {
  return {
    isPlaying: false,
    isPaused: false,
    isLive: true,
    volume: 0.8,
    heldSystem: null as number | null,
    heldTG: null as number | null,
    avoidList: [] as AvoidEntry[],
    currentCallTgId: undefined as number | undefined,
    currentCallSystemId: undefined as number | undefined,
    onTogglePause: vi.fn(),
    onToggleLive: vi.fn(),
    onSkip: vi.fn(),
    onReplay: vi.fn(),
    onDownload: vi.fn(),
    onSetVolume: vi.fn(),
    onHoldSystem: vi.fn(),
    onHoldTG: vi.fn(),
    onAddAvoid: vi.fn(),
    onClearAvoids: vi.fn(),
  };
}

describe("ControlToolbar", () => {
  it("renders Pause button when not paused", () => {
    const props = defaultProps();
    render(<ControlToolbar {...props} />);
    expect(screen.getByRole("button", { name: "Pause" })).toBeInTheDocument();
  });

  it("renders Resume button when paused", () => {
    const props = defaultProps();
    props.isPaused = true;
    render(<ControlToolbar {...props} />);
    expect(screen.getByRole("button", { name: "Resume" })).toBeInTheDocument();
  });

  it("calls onTogglePause when play/pause button clicked", () => {
    const props = defaultProps();
    render(<ControlToolbar {...props} />);
    fireEvent.click(screen.getByRole("button", { name: "Pause" }));
    expect(props.onTogglePause).toHaveBeenCalledOnce();
  });

  it("calls onSkip when skip button clicked", () => {
    const props = defaultProps();
    render(<ControlToolbar {...props} />);
    fireEvent.click(screen.getByRole("button", { name: "Skip" }));
    expect(props.onSkip).toHaveBeenCalledOnce();
  });

  it("calls onReplay when replay button clicked", () => {
    const props = defaultProps();
    render(<ControlToolbar {...props} />);
    fireEvent.click(screen.getByRole("button", { name: "Replay" }));
    expect(props.onReplay).toHaveBeenCalledOnce();
  });

  it("calls onToggleLive when LIVE button clicked", () => {
    const props = defaultProps();
    render(<ControlToolbar {...props} />);
    fireEvent.click(screen.getByText("LIVE"));
    expect(props.onToggleLive).toHaveBeenCalledOnce();
  });

  it("LIVE button has active style when isLive is true", () => {
    const props = defaultProps();
    props.isLive = true;
    render(<ControlToolbar {...props} />);
    const liveBtn = screen.getByText("LIVE").closest("button")!;
    expect(liveBtn.className).toContain("btn-primary");
  });

  it("LIVE button has ghost style when isLive is false", () => {
    const props = defaultProps();
    props.isLive = false;
    render(<ControlToolbar {...props} />);
    const liveBtn = screen.getByText("LIVE").closest("button")!;
    expect(liveBtn.className).toContain("btn-ghost");
  });

  it("volume slider changes value via onSetVolume", () => {
    const props = defaultProps();
    render(<ControlToolbar {...props} />);
    // The desktop range input
    const sliders = document.querySelectorAll('input[type="range"]');
    expect(sliders.length).toBeGreaterThan(0);
    fireEvent.change(sliders[0], { target: { value: "0.5" } });
    expect(props.onSetVolume).toHaveBeenCalledWith(0.5);
  });

  it("calls onSetVolume(0) when mute button clicked and not muted", () => {
    const props = defaultProps();
    props.volume = 0.8;
    render(<ControlToolbar {...props} />);
    fireEvent.click(screen.getByRole("button", { name: "Mute" }));
    expect(props.onSetVolume).toHaveBeenCalledWith(0);
  });

  it("calls onSetVolume(0.8) when unmute button clicked and muted", () => {
    const props = defaultProps();
    props.volume = 0;
    render(<ControlToolbar {...props} />);
    fireEvent.click(screen.getByRole("button", { name: "Unmute" }));
    expect(props.onSetVolume).toHaveBeenCalledWith(0.8);
  });

  it('HOLD dropdown shows "Hold System" and "Hold Talkgroup" options', () => {
    const props = defaultProps();
    render(<ControlToolbar {...props} />);
    expect(screen.getByText("Hold System")).toBeInTheDocument();
    expect(screen.getByText("Hold Talkgroup")).toBeInTheDocument();
  });

  it('HOLD dropdown shows "Release System" when heldSystem is set', () => {
    const props = defaultProps();
    props.heldSystem = 42;
    render(<ControlToolbar {...props} />);
    expect(screen.getByText("Release System")).toBeInTheDocument();
  });

  it('HOLD dropdown shows "Release Talkgroup" when heldTG is set', () => {
    const props = defaultProps();
    props.heldTG = 99;
    render(<ControlToolbar {...props} />);
    expect(screen.getByText("Release Talkgroup")).toBeInTheDocument();
  });

  it("calls onHoldSystem when Hold System clicked", () => {
    const props = defaultProps();
    props.currentCallSystemId = 42;
    render(<ControlToolbar {...props} />);
    fireEvent.click(screen.getByText("Hold System"));
    expect(props.onHoldSystem).toHaveBeenCalledWith(42);
  });

  it("calls onHoldTG when Hold Talkgroup clicked", () => {
    const props = defaultProps();
    props.currentCallTgId = 99;
    render(<ControlToolbar {...props} />);
    fireEvent.click(screen.getByText("Hold Talkgroup"));
    expect(props.onHoldTG).toHaveBeenCalledWith(99);
  });

  it("AVOID dropdown shows duration options", () => {
    const props = defaultProps();
    render(<ControlToolbar {...props} />);
    expect(screen.getByText("30 minutes")).toBeInTheDocument();
    expect(screen.getByText("60 minutes")).toBeInTheDocument();
    expect(screen.getByText("120 minutes")).toBeInTheDocument();
    expect(screen.getByText("Permanent")).toBeInTheDocument();
  });

  it("calls onAddAvoid with correct entry when avoid option clicked", () => {
    const props = defaultProps();
    props.currentCallTgId = 200;
    render(<ControlToolbar {...props} />);
    fireEvent.click(screen.getByText("Permanent"));
    expect(props.onAddAvoid).toHaveBeenCalledWith({
      talkgroupId: 200,
      expiresAt: 0,
    });
  });

  it("shows Clear All in AVOID when avoidList is non-empty", () => {
    const props = defaultProps();
    props.avoidList = [{ talkgroupId: 10, expiresAt: 0 }];
    render(<ControlToolbar {...props} />);
    expect(screen.getByText("Clear All")).toBeInTheDocument();
  });

  it("does not show Clear All in AVOID when avoidList is empty", () => {
    const props = defaultProps();
    render(<ControlToolbar {...props} />);
    expect(screen.queryByText("Clear All")).toBeNull();
  });

  it("calls onClearAvoids when Clear All clicked", () => {
    const props = defaultProps();
    props.avoidList = [{ talkgroupId: 10, expiresAt: 0 }];
    render(<ControlToolbar {...props} />);
    fireEvent.click(screen.getByText("Clear All"));
    expect(props.onClearAvoids).toHaveBeenCalledOnce();
  });

  it("shows avoid count badge when avoidList is non-empty", () => {
    const props = defaultProps();
    props.avoidList = [
      { talkgroupId: 10, expiresAt: 0 },
      { talkgroupId: 20, expiresAt: 0 },
    ];
    render(<ControlToolbar {...props} />);
    expect(screen.getByText("2")).toBeInTheDocument();
  });

  it("renders Download button", () => {
    const props = defaultProps();
    render(<ControlToolbar {...props} />);
    expect(
      screen.getByRole("button", { name: "Download" }),
    ).toBeInTheDocument();
  });

  it("calls onDownload when Download button clicked", () => {
    const props = defaultProps();
    render(<ControlToolbar {...props} />);
    fireEvent.click(screen.getByRole("button", { name: "Download" }));
    expect(props.onDownload).toHaveBeenCalledOnce();
  });
});
