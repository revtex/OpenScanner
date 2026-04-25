import { api } from "@/app/api";
import type { RRPreviewResponse } from "@/types";

// --- Admin RTK Query endpoints (file-upload only; all other admin ops use WebSocket) ---

const adminApi = api.injectEndpoints({
  endpoints: (builder) => ({
    // ── Import (multipart file uploads — cannot use WebSocket) ──
    importTalkgroups: builder.mutation<
      {
        inserted: number;
        updated: number;
        skipped: number;
        failed?: number;
        message?: string;
      },
      FormData
    >({
      query: (body) => ({
        url: "/admin/import/talkgroups",
        method: "POST",
        body,
      }),
      invalidatesTags: ["Talkgroups"],
    }),
    importUnits: builder.mutation<
      {
        inserted: number;
        updated: number;
        skipped: number;
        failed?: number;
      },
      FormData
    >({
      query: (body) => ({
        url: "/admin/import/units",
        method: "POST",
        body,
      }),
      invalidatesTags: ["Units"],
    }),
    importGroups: builder.mutation<
      {
        inserted: number;
        skipped: number;
        failed?: number;
        message?: string;
      },
      FormData
    >({
      query: (body) => ({
        url: "/admin/import/groups",
        method: "POST",
        body,
      }),
      invalidatesTags: ["Groups"],
    }),
    importTags: builder.mutation<
      {
        inserted: number;
        skipped: number;
        failed?: number;
        message?: string;
      },
      FormData
    >({
      query: (body) => ({
        url: "/admin/import/tags",
        method: "POST",
        body,
      }),
      invalidatesTags: ["Tags"],
    }),

    // ── RadioReference CSV preview (multipart file upload) ──
    rrPreviewCSV: builder.mutation<RRPreviewResponse, FormData>({
      query: (body) => ({
        url: "/admin/radioreference/preview/csv",
        method: "POST",
        body,
      }),
    }),
  }),
});

export const {
  useImportTalkgroupsMutation,
  useImportUnitsMutation,
  useImportGroupsMutation,
  useImportTagsMutation,
  useRrPreviewCSVMutation,
} = adminApi;
