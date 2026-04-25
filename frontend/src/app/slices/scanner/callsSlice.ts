import { createSlice, type PayloadAction } from "@reduxjs/toolkit";
import { api } from "@/app/api";

// --- RTK Query types ---

export interface CallSearchResult {
  id: number;
  audioName: string;
  audioType: string;
  dateTime: number;
  frequency: number;
  duration: number;
  source: number;
  systemId: number;
  talkgroupId: number;
  systemLabel: string;
  talkgroupLabel: string;
  talkgroupName: string;
  talkgroupTag: string;
  talkgroupGroup: string;
  talkgroupLed: string;
  site?: string;
  channel?: string;
  decoder?: string;
  errorCount?: number;
  spikeCount?: number;
  talkerAlias?: string;
  transcript: string;
  bookmarked: boolean;
}

export interface CallSearchResponse {
  calls: CallSearchResult[];
  total: number;
}

export interface CallSearchParams {
  systemIds?: number[];
  talkgroupIds?: number[];
  groupFilters?: string[];
  tagFilters?: string[];
  transcript?: string;
  dateFrom?: number;
  dateTo?: number;
  page?: number;
  limit?: number;
  sort?: "asc" | "desc";
  bookmarkedOnly?: boolean;
}

// --- RTK Query endpoint ---

const callsApi = api.injectEndpoints({
  endpoints: (builder) => ({
    searchCalls: builder.query<CallSearchResponse, CallSearchParams>({
      query: (params) => ({
        url: "/calls",
        params: {
          system_ids:
            params.systemIds && params.systemIds.length > 0
              ? params.systemIds.join(",")
              : undefined,
          talkgroup_ids:
            params.talkgroupIds && params.talkgroupIds.length > 0
              ? params.talkgroupIds.join(",")
              : undefined,
          groups:
            params.groupFilters && params.groupFilters.length > 0
              ? params.groupFilters.join(",")
              : undefined,
          tags:
            params.tagFilters && params.tagFilters.length > 0
              ? params.tagFilters.join(",")
              : undefined,
          transcript: params.transcript,
          date_from: params.dateFrom,
          date_to: params.dateTo,
          page: params.page,
          limit: params.limit,
          sort: params.sort,
          bookmarked_only: params.bookmarkedOnly ? "true" : undefined,
        },
      }),
      providesTags: [{ type: "Calls", id: "LIST" }],
    }),
  }),
});

export const { useSearchCallsQuery } = callsApi;

// --- Search filter state slice ---

interface SearchFilters {
  systemIds: number[];
  talkgroupIds: number[];
  groupFilters: string[];
  tagFilters: string[];
  dateFrom?: string;
  dateTo?: string;
  sort: "asc" | "desc";
  page: number;
  limit: number;
  bookmarkedOnly: boolean;
  downloadMode: boolean;
  transcript?: string;
}

const initialState: SearchFilters = {
  systemIds: [],
  talkgroupIds: [],
  groupFilters: [],
  tagFilters: [],
  sort: "desc",
  page: 1,
  limit: 25,
  bookmarkedOnly: false,
  downloadMode: false,
};

export const callsSlice = createSlice({
  name: "calls",
  initialState,
  reducers: {
    toggleSystemFilter(state, action: PayloadAction<number>) {
      const id = action.payload;
      if (state.systemIds.includes(id)) {
        state.systemIds = state.systemIds.filter((v) => v !== id);
      } else {
        state.systemIds.push(id);
      }
      state.page = 1;
    },
    toggleTalkgroupFilter(state, action: PayloadAction<number>) {
      const id = action.payload;
      if (state.talkgroupIds.includes(id)) {
        state.talkgroupIds = state.talkgroupIds.filter((v) => v !== id);
      } else {
        state.talkgroupIds.push(id);
      }
      state.page = 1;
    },
    toggleGroupFilter(state, action: PayloadAction<string>) {
      const label = action.payload;
      if (state.groupFilters.includes(label)) {
        state.groupFilters = state.groupFilters.filter((v) => v !== label);
      } else {
        state.groupFilters.push(label);
      }
      state.page = 1;
    },
    toggleTagFilter(state, action: PayloadAction<string>) {
      const label = action.payload;
      if (state.tagFilters.includes(label)) {
        state.tagFilters = state.tagFilters.filter((v) => v !== label);
      } else {
        state.tagFilters.push(label);
      }
      state.page = 1;
    },
    setSystemFilters(state, action: PayloadAction<number[]>) {
      state.systemIds = [...action.payload];
      state.page = 1;
    },
    setTalkgroupFilters(state, action: PayloadAction<number[]>) {
      state.talkgroupIds = [...action.payload];
      state.page = 1;
    },
    setGroupFilters(state, action: PayloadAction<string[]>) {
      state.groupFilters = [...action.payload];
      state.page = 1;
    },
    setTagFilters(state, action: PayloadAction<string[]>) {
      state.tagFilters = [...action.payload];
      state.page = 1;
    },
    setDateFrom(state, action: PayloadAction<string | undefined>) {
      state.dateFrom = action.payload;
      state.page = 1;
    },
    setDateTo(state, action: PayloadAction<string | undefined>) {
      state.dateTo = action.payload;
      state.page = 1;
    },
    setSort(state, action: PayloadAction<"asc" | "desc">) {
      state.sort = action.payload;
      state.page = 1;
    },
    setPage(state, action: PayloadAction<number>) {
      state.page = action.payload;
    },
    setLimit(state, action: PayloadAction<number>) {
      state.limit = action.payload;
      state.page = 1;
    },
    setBookmarkedOnly(state, action: PayloadAction<boolean>) {
      state.bookmarkedOnly = action.payload;
      state.page = 1;
    },
    setDownloadMode(state, action: PayloadAction<boolean>) {
      state.downloadMode = action.payload;
    },
    setTranscript(state, action: PayloadAction<string | undefined>) {
      state.transcript = action.payload;
      state.page = 1;
    },
    resetFilters() {
      return initialState;
    },
  },
});

export const {
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
} = callsSlice.actions;
