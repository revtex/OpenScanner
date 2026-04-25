import { describe, it, expect } from "vitest";
import {
  callsSlice,
  toggleSystemFilter,
  toggleTalkgroupFilter,
  toggleGroupFilter,
  toggleTagFilter,
  setSystemFilters,
  setTalkgroupFilters,
  setGroupFilters,
  setTagFilters,
  setDateFrom,
  setDateTo,
  setSort,
  setPage,
  setLimit,
  setBookmarkedOnly,
  setDownloadMode,
  setTranscript,
  resetFilters,
} from "@/app/slices/scanner/callsSlice";

const reducer = callsSlice.reducer;

describe("callsSlice", () => {
  describe("toggleSystemFilter / setSystemFilters", () => {
    it("toggles system IDs", () => {
      let state = reducer(undefined, toggleSystemFilter(42));
      expect(state.systemIds).toEqual([42]);
      state = reducer(state, toggleSystemFilter(42));
      expect(state.systemIds).toEqual([]);
    });

    it("sets system IDs in bulk", () => {
      const state = reducer(undefined, setSystemFilters([1, 2, 3]));
      expect(state.systemIds).toEqual([1, 2, 3]);
    });

    it("resets page to 1", () => {
      let state = reducer(undefined, setPage(5));
      state = reducer(state, toggleSystemFilter(42));
      expect(state.page).toBe(1);
    });
  });

  describe("toggleTalkgroupFilter / setTalkgroupFilters", () => {
    it("toggles talkgroup IDs", () => {
      let state = reducer(undefined, toggleTalkgroupFilter(99));
      expect(state.talkgroupIds).toEqual([99]);
      state = reducer(state, toggleTalkgroupFilter(99));
      expect(state.talkgroupIds).toEqual([]);
    });

    it("sets talkgroup IDs in bulk", () => {
      const state = reducer(undefined, setTalkgroupFilters([11, 22]));
      expect(state.talkgroupIds).toEqual([11, 22]);
    });

    it("resets page to 1", () => {
      let state = reducer(undefined, setPage(3));
      state = reducer(state, toggleTalkgroupFilter(99));
      expect(state.page).toBe(1);
    });
  });

  describe("toggleGroupFilter / setGroupFilters", () => {
    it("toggles group filters", () => {
      let state = reducer(undefined, toggleGroupFilter("Police"));
      expect(state.groupFilters).toEqual(["Police"]);
      state = reducer(state, toggleGroupFilter("Police"));
      expect(state.groupFilters).toEqual([]);
    });

    it("sets group filters in bulk", () => {
      const state = reducer(undefined, setGroupFilters(["Police", "Fire"]));
      expect(state.groupFilters).toEqual(["Police", "Fire"]);
    });

    it("resets page to 1", () => {
      let state = reducer(undefined, setPage(4));
      state = reducer(state, toggleGroupFilter("Fire"));
      expect(state.page).toBe(1);
    });
  });

  describe("toggleTagFilter / setTagFilters", () => {
    it("toggles tag filters", () => {
      let state = reducer(undefined, toggleTagFilter("Law"));
      expect(state.tagFilters).toEqual(["Law"]);
      state = reducer(state, toggleTagFilter("Law"));
      expect(state.tagFilters).toEqual([]);
    });

    it("sets tag filters in bulk", () => {
      const state = reducer(undefined, setTagFilters(["Law", "EMS"]));
      expect(state.tagFilters).toEqual(["Law", "EMS"]);
    });

    it("resets page to 1", () => {
      let state = reducer(undefined, setPage(2));
      state = reducer(state, toggleTagFilter("EMS"));
      expect(state.page).toBe(1);
    });
  });

  describe("setDateFrom / setDateTo", () => {
    it("setDateFrom sets date filter", () => {
      const state = reducer(undefined, setDateFrom("2025-01-01"));
      expect(state.dateFrom).toBe("2025-01-01");
    });

    it("setDateTo sets date filter", () => {
      const state = reducer(undefined, setDateTo("2025-12-31"));
      expect(state.dateTo).toBe("2025-12-31");
    });

    it("setDateFrom resets page to 1", () => {
      let state = reducer(undefined, setPage(5));
      state = reducer(state, setDateFrom("2025-06-01"));
      expect(state.page).toBe(1);
    });

    it("setDateTo resets page to 1", () => {
      let state = reducer(undefined, setPage(5));
      state = reducer(state, setDateTo("2025-06-30"));
      expect(state.page).toBe(1);
    });

    it("clears date filters with undefined", () => {
      let state = reducer(undefined, setDateFrom("2025-01-01"));
      state = reducer(state, setDateFrom(undefined));
      expect(state.dateFrom).toBeUndefined();
    });
  });

  describe("setSort", () => {
    it("changes sort to asc", () => {
      const state = reducer(undefined, setSort("asc"));
      expect(state.sort).toBe("asc");
    });

    it("changes sort to desc", () => {
      let state = reducer(undefined, setSort("asc"));
      state = reducer(state, setSort("desc"));
      expect(state.sort).toBe("desc");
    });

    it("resets page to 1", () => {
      let state = reducer(undefined, setPage(3));
      state = reducer(state, setSort("asc"));
      expect(state.page).toBe(1);
    });
  });

  describe("setPage", () => {
    it("changes page number", () => {
      const state = reducer(undefined, setPage(7));
      expect(state.page).toBe(7);
    });
  });

  describe("setLimit", () => {
    it("changes limit", () => {
      const state = reducer(undefined, setLimit(50));
      expect(state.limit).toBe(50);
    });

    it("resets page to 1", () => {
      let state = reducer(undefined, setPage(3));
      state = reducer(state, setLimit(50));
      expect(state.page).toBe(1);
    });
  });

  describe("setBookmarkedOnly", () => {
    it("toggles bookmarked filter on", () => {
      const state = reducer(undefined, setBookmarkedOnly(true));
      expect(state.bookmarkedOnly).toBe(true);
    });

    it("toggles bookmarked filter off", () => {
      let state = reducer(undefined, setBookmarkedOnly(true));
      state = reducer(state, setBookmarkedOnly(false));
      expect(state.bookmarkedOnly).toBe(false);
    });

    it("resets page to 1", () => {
      let state = reducer(undefined, setPage(4));
      state = reducer(state, setBookmarkedOnly(true));
      expect(state.page).toBe(1);
    });
  });

  describe("setDownloadMode", () => {
    it("toggles download mode on", () => {
      const state = reducer(undefined, setDownloadMode(true));
      expect(state.downloadMode).toBe(true);
    });

    it("toggles download mode off", () => {
      let state = reducer(undefined, setDownloadMode(true));
      state = reducer(state, setDownloadMode(false));
      expect(state.downloadMode).toBe(false);
    });

    it("does NOT reset page", () => {
      let state = reducer(undefined, setPage(3));
      state = reducer(state, setDownloadMode(true));
      expect(state.page).toBe(3);
    });
  });

  describe("setTranscript", () => {
    it("sets transcript filter", () => {
      const state = reducer(undefined, setTranscript("fire"));
      expect(state.transcript).toBe("fire");
    });

    it("clears transcript filter with undefined", () => {
      let state = reducer(undefined, setTranscript("test"));
      state = reducer(state, setTranscript(undefined));
      expect(state.transcript).toBeUndefined();
    });

    it("resets page to 1", () => {
      let state = reducer(undefined, setPage(5));
      state = reducer(state, setTranscript("hello"));
      expect(state.page).toBe(1);
    });
  });

  describe("resetFilters", () => {
    it("resets all filters to initial state", () => {
      let state = reducer(undefined, toggleSystemFilter(1));
      state = reducer(state, toggleTalkgroupFilter(2));
      state = reducer(state, toggleGroupFilter("Police"));
      state = reducer(state, toggleTagFilter("Law"));
      state = reducer(state, setDateFrom("2025-01-01"));
      state = reducer(state, setDateTo("2025-12-31"));
      state = reducer(state, setSort("asc"));
      state = reducer(state, setBookmarkedOnly(true));
      state = reducer(state, setDownloadMode(true));
      state = reducer(state, setTranscript("test"));
      state = reducer(state, resetFilters());

      expect(state.systemIds).toEqual([]);
      expect(state.talkgroupIds).toEqual([]);
      expect(state.dateFrom).toBeUndefined();
      expect(state.dateTo).toBeUndefined();
      expect(state.sort).toBe("desc");
      expect(state.page).toBe(1);
      expect(state.limit).toBe(25);
      expect(state.bookmarkedOnly).toBe(false);
      expect(state.downloadMode).toBe(false);
      expect(state.transcript).toBeUndefined();
      expect(state.groupFilters).toEqual([]);
      expect(state.tagFilters).toEqual([]);
    });
  });
});
