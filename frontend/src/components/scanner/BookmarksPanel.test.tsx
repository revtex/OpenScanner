import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";

// Mock RTK Query hooks before importing the component
const mockGetBookmarkCallsQuery = vi.fn();
const mockToggleBookmarkMutation = vi.fn();

vi.mock("@/app/api", () => ({
  useGetBookmarkCallsQuery: (...args: unknown[]) =>
    mockGetBookmarkCallsQuery(...args),
  useToggleBookmarkMutation: () => mockToggleBookmarkMutation(),
}));

vi.mock("@/shared/services/audio/player", () => ({
  audioPlayer: {
    play: vi.fn(),
  },
}));

vi.mock("@/app/slices/shared/authSlice", () => ({
  selectToken: () => "fake-token",
}));

vi.mock("@/app/store", () => ({
  useAppSelector: (selector: (state: unknown) => unknown) =>
    selector({ scanner: { config: null } }),
}));

vi.mock("@/app/slices/scanner/shareSlice", () => ({
  useShareCallMutation: () => [vi.fn(), {}],
}));

import BookmarksPanel from "@/components/scanner/BookmarksPanel";

describe("BookmarksPanel", () => {
  function setup(overrides?: { calls?: unknown[]; isLoading?: boolean }) {
    const calls = overrides?.calls ?? [];
    const isLoading = overrides?.isLoading ?? false;

    mockGetBookmarkCallsQuery.mockReturnValue({
      data: { calls },
      isLoading,
    });
    mockToggleBookmarkMutation.mockReturnValue([vi.fn(), {}]);
  }

  it('shows "Bookmarks" heading when open', () => {
    setup();
    render(<BookmarksPanel isOpen={true} onClose={vi.fn()} />);
    expect(screen.getByText("Bookmarks")).toBeInTheDocument();
  });

  it("does not show panel content visibly when closed", () => {
    setup();
    const { container } = render(
      <BookmarksPanel isOpen={false} onClose={vi.fn()} />,
    );
    // The panel should have translate-x-full (off-screen) when closed
    const panel = container.firstElementChild as HTMLElement;
    expect(panel.className).toContain("translate-x-full");
  });

  it("shows empty state when no bookmarks", () => {
    setup({ calls: [] });
    render(<BookmarksPanel isOpen={true} onClose={vi.fn()} />);
    expect(screen.getByText("No bookmarked calls")).toBeInTheDocument();
  });

  it("calls onClose when close button clicked", () => {
    setup();
    const onClose = vi.fn();
    render(<BookmarksPanel isOpen={true} onClose={onClose} />);
    fireEvent.click(screen.getByLabelText("Close bookmarks"));
    expect(onClose).toHaveBeenCalledOnce();
  });
});
