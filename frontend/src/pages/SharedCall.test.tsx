import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";

const mockUseParams = vi.fn();
vi.mock("react-router-dom", async () => {
  const actual =
    await vi.importActual<typeof import("react-router-dom")>(
      "react-router-dom",
    );
  return { ...actual, useParams: () => mockUseParams() };
});

const mockUseGetSharedCallQuery = vi.fn();
vi.mock("@/app/slices/shareSlice", () => ({
  useGetSharedCallQuery: (...args: unknown[]) =>
    mockUseGetSharedCallQuery(...args),
}));

import SharedCall from "@/pages/SharedCall";

describe("SharedCall", () => {
  it("shows loading spinner when fetching", () => {
    mockUseParams.mockReturnValue({ token: "abc-123-def" });
    mockUseGetSharedCallQuery.mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
    });

    const { container } = render(<SharedCall />);
    expect(container.querySelector(".loading-spinner")).toBeInTheDocument();
  });

  it('shows "Call not found" when error', () => {
    mockUseParams.mockReturnValue({ token: "abc-123-def" });
    mockUseGetSharedCallQuery.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
    });

    render(<SharedCall />);
    expect(screen.getByText("Call not found")).toBeInTheDocument();
  });

  it("shows call details when data loaded", () => {
    mockUseParams.mockReturnValue({ token: "abc-123-def" });
    mockUseGetSharedCallQuery.mockReturnValue({
      data: {
        token: "abc-123-def",
        dateTime: 1700000000,
        systemLabel: "Test System",
        talkgroupLabel: "TG Alpha",
        talkgroupName: "Alpha Group",
        frequency: 851_000_000,
        duration: 5000,
        source: 123,
        audioUrl: "/api/shared/abc-123-def/audio",
      },
      isLoading: false,
      isError: false,
    });

    render(<SharedCall />);
    expect(screen.getByText("Shared Call")).toBeInTheDocument();
    expect(screen.getByText("Test System")).toBeInTheDocument();
    expect(screen.getByText(/TG Alpha/)).toBeInTheDocument();
    expect(screen.getByText("851.0000 MHz")).toBeInTheDocument();
  });

  it('shows "Call not found" when no data and not loading', () => {
    mockUseParams.mockReturnValue({ token: "nonexistent-token" });
    mockUseGetSharedCallQuery.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: false,
    });

    render(<SharedCall />);
    expect(screen.getByText("Call not found")).toBeInTheDocument();
  });

  it("falls back when API returns an unsafe audio URL", () => {
    mockUseParams.mockReturnValue({ token: "abc-123-def" });
    mockUseGetSharedCallQuery.mockReturnValue({
      data: {
        token: "abc-123-def",
        dateTime: 1700000000,
        systemLabel: "Test System",
        talkgroupLabel: "TG Alpha",
        talkgroupName: "Alpha Group",
        frequency: 851_000_000,
        duration: 5000,
        source: 123,
        audioUrl: "javascript:alert(1)",
      },
      isLoading: false,
      isError: false,
    });

    render(<SharedCall />);
    expect(screen.getByText("Call not found")).toBeInTheDocument();
    expect(screen.queryByText("Download")).not.toBeInTheDocument();
  });
});
