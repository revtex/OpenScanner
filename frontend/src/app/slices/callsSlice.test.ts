import { describe, it, expect } from "vitest";
import {
  callsSlice,
  setSystemFilter,
  setTalkgroupFilter,
  setGroupFilter,
  setTagFilter,
  setDateFrom,
  setDateTo,
  setSort,
  setPage,
  setLimit,
  setBookmarkedOnly,
  setDownloadMode,
  setTranscript,
  resetFilters,
} from "@/app/slices/callsSlice";

const reducer = callsSlice.reducer;

describe("callsSlice", () => {
  describe("setSystemFilter", () => {
    it("sets the systemId filter", () => {
      const state = reducer(undefined, setSystemFilter(42));
      expect(state.systemId).toBe(42);
    });

    it("clears talkgroupId when system changes", () => {
      let state = reducer(undefined, setTalkgroupFilter(10));
      state = reducer(state, setSystemFilter(42));
      expect(state.talkgroupId).toBeUndefined();
    });

    it("resets page to 1", () => {
      let state = reducer(undefined, setPage(5));
      state = reducer(state, setSystemFilter(42));
      expect(state.page).toBe(1);
    });

    it("clears systemId with undefined", () => {
      let state = reducer(undefined, setSystemFilter(42));
      state = reducer(state, setSystemFilter(undefined));
      expect(state.systemId).toBeUndefined();
    });
  });

  describe("setTalkgroupFilter", () => {
    it("sets the talkgroupId filter", () => {
      const state = reducer(undefined, setTalkgroupFilter(99));
      expect(state.talkgroupId).toBe(99);
    });

    it("resets page to 1", () => {
      let state = reducer(undefined, setPage(3));
      state = reducer(state, setTalkgroupFilter(99));
      expect(state.page).toBe(1);
    });
  });

  describe("setGroupFilter", () => {
    it("sets the group filter", () => {
      const state = reducer(undefined, setGroupFilter("Police"));
      expect(state.groupFilter).toBe("Police");
    });

    it("resets page to 1", () => {
      let state = reducer(undefined, setPage(4));
      state = reducer(state, setGroupFilter("Fire"));
      expect(state.page).toBe(1);
    });
  });

  describe("setTagFilter", () => {
    it("sets the tag filter", () => {
      const state = reducer(undefined, setTagFilter("Law"));
      expect(state.tagFilter).toBe("Law");
    });

    it("resets page to 1", () => {
      let state = reducer(undefined, setPage(2));
      state = reducer(state, setTagFilter("EMS"));
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
      let state = reducer(undefined, setSystemFilter(1));
      state = reducer(state, setTalkgroupFilter(2));
      state = reducer(state, setDateFrom("2025-01-01"));
      state = reducer(state, setDateTo("2025-12-31"));
      state = reducer(state, setSort("asc"));
      state = reducer(state, setBookmarkedOnly(true));
      state = reducer(state, setDownloadMode(true));
      state = reducer(state, setTranscript("test"));
      state = reducer(state, resetFilters());

      expect(state.systemId).toBeUndefined();
      expect(state.talkgroupId).toBeUndefined();
      expect(state.dateFrom).toBeUndefined();
      expect(state.dateTo).toBeUndefined();
      expect(state.sort).toBe("desc");
      expect(state.page).toBe(1);
      expect(state.limit).toBe(25);
      expect(state.bookmarkedOnly).toBe(false);
      expect(state.downloadMode).toBe(false);
      expect(state.transcript).toBeUndefined();
      expect(state.groupFilter).toBeUndefined();
      expect(state.tagFilter).toBeUndefined();
    });
  });
});
