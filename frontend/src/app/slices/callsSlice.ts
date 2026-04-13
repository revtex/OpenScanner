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
  transcript: string;
  bookmarked: boolean;
}

export interface CallSearchResponse {
  calls: CallSearchResult[];
  total: number;
}

export interface CallSearchParams {
  systemId?: number;
  talkgroupId?: number;
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
          system_id: params.systemId,
          talkgroup_id: params.talkgroupId,
          date_from: params.dateFrom,
          date_to: params.dateTo,
          page: params.page,
          limit: params.limit,
          sort: params.sort,
          bookmarked_only: params.bookmarkedOnly ? "true" : undefined,
        },
      }),
    }),
  }),
});

export const { useSearchCallsQuery } = callsApi;

// --- Search filter state slice ---

interface SearchFilters {
  systemId?: number;
  talkgroupId?: number;
  groupFilter?: string;
  tagFilter?: string;
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
    setSystemFilter(state, action: PayloadAction<number | undefined>) {
      state.systemId = action.payload;
      state.talkgroupId = undefined;
      state.page = 1;
    },
    setTalkgroupFilter(state, action: PayloadAction<number | undefined>) {
      state.talkgroupId = action.payload;
      state.page = 1;
    },
    setGroupFilter(state, action: PayloadAction<string | undefined>) {
      state.groupFilter = action.payload;
      state.page = 1;
    },
    setTagFilter(state, action: PayloadAction<string | undefined>) {
      state.tagFilter = action.payload;
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
} = callsSlice.actions;
