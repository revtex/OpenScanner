import { api } from "@/app/api";
import type {
  AdminUser,
  AdminSystem,
  AdminTalkgroup,
  AdminUnit,
  AdminGroup,
  AdminTag,
  AdminApiKey,
  AdminApiKeyCreateResponse,
  AdminDirwatch,
  AdminDownstream,
  AdminWebhook,
  AdminSetting,
  AdminLog,
  CreateUserPayload,
  UpdateUserPayload,
} from "@/types";

// --- Generic CRUD payload types ---

type CreatePayload<T> = Omit<T, "id">;
type UpdatePayload<T> = { id: number } & Partial<Omit<T, "id">>;
type CreateApiKeyPayload = {
  ident: string | null;
  disabled: number;
  systemsJson: string | null;
  order: number;
  key?: string | null;
};
type UpdateApiKeyPayload = {
  ident: string | null;
  disabled: number;
  systemsJson: string | null;
  order: number;
  key?: string | null;
};

// --- Log query params ---

interface LogQueryParams {
  from?: number;
  to?: number;
  level?: string;
}

interface ServerDirectoryEntry {
  name: string;
  path: string;
}

interface ServerDirectoryListResponse {
  path: string;
  parent: string | null;
  directories: ServerDirectoryEntry[];
}

export interface MissingAudioCall {
  id: number;
  dateTime: number;
  audioPath: string;
  audioName: string;
  reason: string;
}

export interface MissingAudioResponse {
  recordingsDir: string;
  limit: number;
  offset: number;
  totalCalls: number;
  checked: number;
  missing: MissingAudioCall[];
}

export interface MissingAudioCleanupResponse {
  requested: number;
  deleted: number;
  skipped: MissingAudioCall[];
}

// --- Admin RTK Query endpoints ---

const adminApi = api.injectEndpoints({
  endpoints: (builder) => ({
    // ── Users ──
    listUsers: builder.query<AdminUser[], void>({
      query: () => "/admin/users",
      providesTags: ["Users"],
    }),
    createUser: builder.mutation<AdminUser, CreateUserPayload>({
      query: (body) => ({ url: "/admin/users", method: "POST", body }),
      invalidatesTags: ["Users"],
    }),
    updateUser: builder.mutation<AdminUser, { id: number } & UpdateUserPayload>(
      {
        query: ({ id, ...body }) => ({
          url: `/admin/users/${id}`,
          method: "PUT",
          body,
        }),
        async onQueryStarted({ id, ...body }, { dispatch, queryFulfilled }) {
          const patch = dispatch(
            adminApi.util.updateQueryData("listUsers", undefined, (draft) => {
              const user = draft.find((u) => u.id === id);
              if (!user) return;
              Object.assign(user, body);
            }),
          );
          try {
            await queryFulfilled;
          } catch {
            patch.undo();
          }
        },
        invalidatesTags: ["Users"],
      },
    ),
    deleteUser: builder.mutation<void, number>({
      query: (id) => ({ url: `/admin/users/${id}`, method: "DELETE" }),
      invalidatesTags: ["Users"],
    }),

    // ── Systems ──
    listSystems: builder.query<AdminSystem[], void>({
      query: () => "/admin/systems",
      providesTags: ["Systems"],
    }),
    createSystem: builder.mutation<AdminSystem, CreatePayload<AdminSystem>>({
      query: (body) => ({ url: "/admin/systems", method: "POST", body }),
      invalidatesTags: ["Systems"],
    }),
    updateSystem: builder.mutation<AdminSystem, UpdatePayload<AdminSystem>>({
      query: ({ id, ...body }) => ({
        url: `/admin/systems/${id}`,
        method: "PUT",
        body,
      }),
      async onQueryStarted({ id, ...body }, { dispatch, queryFulfilled }) {
        const patch = dispatch(
          adminApi.util.updateQueryData("listSystems", undefined, (draft) => {
            const sys = draft.find((s) => s.id === id);
            if (!sys) return;
            Object.assign(sys, body);
          }),
        );
        try {
          await queryFulfilled;
        } catch {
          patch.undo();
        }
      },
      invalidatesTags: ["Systems"],
    }),
    reorderSystems: builder.mutation<
      void,
      Array<{ id: number; order: number }>
    >({
      query: (systems) => ({
        url: "/admin/systems/reorder",
        method: "PUT",
        body: { systems },
      }),
      async onQueryStarted(systems, { dispatch, queryFulfilled }) {
        const patch = dispatch(
          adminApi.util.updateQueryData("listSystems", undefined, (draft) => {
            const nextOrder = new Map(systems.map((s) => [s.id, s.order]));
            for (const sys of draft) {
              const order = nextOrder.get(sys.id);
              if (order !== undefined) {
                sys.order = order;
              }
            }
          }),
        );
        try {
          await queryFulfilled;
        } catch {
          patch.undo();
        }
      },
    }),
    deleteSystem: builder.mutation<void, number>({
      query: (id) => ({ url: `/admin/systems/${id}`, method: "DELETE" }),
      invalidatesTags: ["Systems"],
    }),

    // ── Talkgroups ──
    listTalkgroups: builder.query<AdminTalkgroup[], void>({
      query: () => "/admin/talkgroups",
      providesTags: ["Talkgroups"],
    }),
    createTalkgroup: builder.mutation<
      AdminTalkgroup,
      CreatePayload<AdminTalkgroup>
    >({
      query: (body) => ({ url: "/admin/talkgroups", method: "POST", body }),
      invalidatesTags: ["Talkgroups"],
    }),
    updateTalkgroup: builder.mutation<
      AdminTalkgroup,
      UpdatePayload<AdminTalkgroup>
    >({
      query: ({ id, ...body }) => ({
        url: `/admin/talkgroups/${id}`,
        method: "PUT",
        body,
      }),
      invalidatesTags: ["Talkgroups"],
    }),
    deleteTalkgroup: builder.mutation<void, number>({
      query: (id) => ({ url: `/admin/talkgroups/${id}`, method: "DELETE" }),
      invalidatesTags: ["Talkgroups"],
    }),

    // ── Units ──
    listUnits: builder.query<AdminUnit[], void>({
      query: () => "/admin/units",
      providesTags: ["Units"],
    }),
    createUnit: builder.mutation<AdminUnit, CreatePayload<AdminUnit>>({
      query: (body) => ({ url: "/admin/units", method: "POST", body }),
      invalidatesTags: ["Units"],
    }),
    updateUnit: builder.mutation<AdminUnit, UpdatePayload<AdminUnit>>({
      query: ({ id, ...body }) => ({
        url: `/admin/units/${id}`,
        method: "PUT",
        body,
      }),
      invalidatesTags: ["Units"],
    }),
    deleteUnit: builder.mutation<void, number>({
      query: (id) => ({ url: `/admin/units/${id}`, method: "DELETE" }),
      invalidatesTags: ["Units"],
    }),

    // ── Groups ──
    listGroups: builder.query<AdminGroup[], void>({
      query: () => "/admin/groups",
      providesTags: ["Groups"],
    }),
    createGroup: builder.mutation<AdminGroup, CreatePayload<AdminGroup>>({
      query: (body) => ({ url: "/admin/groups", method: "POST", body }),
      invalidatesTags: ["Groups"],
    }),
    updateGroup: builder.mutation<AdminGroup, UpdatePayload<AdminGroup>>({
      query: ({ id, ...body }) => ({
        url: `/admin/groups/${id}`,
        method: "PUT",
        body,
      }),
      invalidatesTags: ["Groups"],
    }),
    deleteGroup: builder.mutation<void, number>({
      query: (id) => ({ url: `/admin/groups/${id}`, method: "DELETE" }),
      invalidatesTags: ["Groups"],
    }),

    // ── Tags ──
    listTags: builder.query<AdminTag[], void>({
      query: () => "/admin/tags",
      providesTags: ["Tags"],
    }),
    createTag: builder.mutation<AdminTag, CreatePayload<AdminTag>>({
      query: (body) => ({ url: "/admin/tags", method: "POST", body }),
      invalidatesTags: ["Tags"],
    }),
    updateTag: builder.mutation<AdminTag, UpdatePayload<AdminTag>>({
      query: ({ id, ...body }) => ({
        url: `/admin/tags/${id}`,
        method: "PUT",
        body,
      }),
      invalidatesTags: ["Tags"],
    }),
    deleteTag: builder.mutation<void, number>({
      query: (id) => ({ url: `/admin/tags/${id}`, method: "DELETE" }),
      invalidatesTags: ["Tags"],
    }),

    // ── API Keys ──
    listApiKeys: builder.query<AdminApiKey[], void>({
      query: () => "/admin/apikeys",
      providesTags: ["ApiKeys"],
    }),
    createApiKey: builder.mutation<
      AdminApiKeyCreateResponse,
      CreateApiKeyPayload
    >({
      query: (body) => ({ url: "/admin/apikeys", method: "POST", body }),
      invalidatesTags: ["ApiKeys"],
    }),
    updateApiKey: builder.mutation<
      AdminApiKey,
      { id: number } & UpdateApiKeyPayload
    >({
      query: ({ id, ...body }) => ({
        url: `/admin/apikeys/${id}`,
        method: "PUT",
        body,
      }),
      async onQueryStarted({ id, ...body }, { dispatch, queryFulfilled }) {
        const patch = dispatch(
          adminApi.util.updateQueryData("listApiKeys", undefined, (draft) => {
            const key = draft.find((k) => k.id === id);
            if (!key) return;
            Object.assign(key, body);
          }),
        );
        try {
          await queryFulfilled;
        } catch {
          patch.undo();
        }
      },
      invalidatesTags: ["ApiKeys"],
    }),
    reorderApiKeys: builder.mutation<
      void,
      Array<{ id: number; order: number }>
    >({
      query: (apiKeys) => ({
        url: "/admin/apikeys/reorder",
        method: "PUT",
        body: { apiKeys },
      }),
      async onQueryStarted(apiKeys, { dispatch, queryFulfilled }) {
        const patch = dispatch(
          adminApi.util.updateQueryData("listApiKeys", undefined, (draft) => {
            const nextOrder = new Map(apiKeys.map((k) => [k.id, k.order]));
            for (const key of draft) {
              const order = nextOrder.get(key.id);
              if (order !== undefined) {
                key.order = order;
              }
            }
          }),
        );
        try {
          await queryFulfilled;
        } catch {
          patch.undo();
        }
      },
    }),
    deleteApiKey: builder.mutation<void, number>({
      query: (id) => ({ url: `/admin/apikeys/${id}`, method: "DELETE" }),
      invalidatesTags: ["ApiKeys"],
    }),

    // ── Dirwatches ──
    listServerDirectories: builder.query<
      ServerDirectoryListResponse,
      { path: string }
    >({
      query: ({ path }) => ({
        url: "/admin/fs/directories",
        params: { path },
      }),
    }),
    listDirwatches: builder.query<AdminDirwatch[], void>({
      query: () => "/admin/dirwatches",
      providesTags: ["Dirwatches"],
    }),
    createDirwatch: builder.mutation<
      AdminDirwatch,
      CreatePayload<AdminDirwatch>
    >({
      query: (body) => ({ url: "/admin/dirwatches", method: "POST", body }),
      invalidatesTags: ["Dirwatches"],
    }),
    updateDirwatch: builder.mutation<
      AdminDirwatch,
      UpdatePayload<AdminDirwatch>
    >({
      query: ({ id, ...body }) => ({
        url: `/admin/dirwatches/${id}`,
        method: "PUT",
        body,
      }),
      async onQueryStarted({ id, ...body }, { dispatch, queryFulfilled }) {
        const patch = dispatch(
          adminApi.util.updateQueryData(
            "listDirwatches",
            undefined,
            (draft) => {
              const dw = draft.find((d) => d.id === id);
              if (!dw) return;
              Object.assign(dw, body);
            },
          ),
        );
        try {
          await queryFulfilled;
        } catch {
          patch.undo();
        }
      },
      invalidatesTags: ["Dirwatches"],
    }),
    deleteDirwatch: builder.mutation<void, number>({
      query: (id) => ({ url: `/admin/dirwatches/${id}`, method: "DELETE" }),
      invalidatesTags: ["Dirwatches"],
    }),

    // ── Downstreams ──
    listDownstreams: builder.query<AdminDownstream[], void>({
      query: () => "/admin/downstreams",
      providesTags: ["Downstreams"],
    }),
    createDownstream: builder.mutation<
      AdminDownstream,
      CreatePayload<AdminDownstream>
    >({
      query: (body) => ({ url: "/admin/downstreams", method: "POST", body }),
      invalidatesTags: ["Downstreams"],
    }),
    updateDownstream: builder.mutation<
      AdminDownstream,
      UpdatePayload<AdminDownstream>
    >({
      query: ({ id, ...body }) => ({
        url: `/admin/downstreams/${id}`,
        method: "PUT",
        body,
      }),
      async onQueryStarted({ id, ...body }, { dispatch, queryFulfilled }) {
        const patch = dispatch(
          adminApi.util.updateQueryData(
            "listDownstreams",
            undefined,
            (draft) => {
              const ds = draft.find((d) => d.id === id);
              if (!ds) return;
              Object.assign(ds, body);
            },
          ),
        );
        try {
          await queryFulfilled;
        } catch {
          patch.undo();
        }
      },
      invalidatesTags: ["Downstreams"],
    }),
    deleteDownstream: builder.mutation<void, number>({
      query: (id) => ({ url: `/admin/downstreams/${id}`, method: "DELETE" }),
      invalidatesTags: ["Downstreams"],
    }),

    // ── Webhooks ──
    listWebhooks: builder.query<AdminWebhook[], void>({
      query: () => "/admin/webhooks",
      providesTags: ["Webhooks"],
    }),
    createWebhook: builder.mutation<AdminWebhook, CreatePayload<AdminWebhook>>({
      query: (body) => ({ url: "/admin/webhooks", method: "POST", body }),
      invalidatesTags: ["Webhooks"],
    }),
    updateWebhook: builder.mutation<AdminWebhook, UpdatePayload<AdminWebhook>>({
      query: ({ id, ...body }) => ({
        url: `/admin/webhooks/${id}`,
        method: "PUT",
        body,
      }),
      async onQueryStarted({ id, ...body }, { dispatch, queryFulfilled }) {
        const patch = dispatch(
          adminApi.util.updateQueryData("listWebhooks", undefined, (draft) => {
            const wh = draft.find((w) => w.id === id);
            if (!wh) return;
            Object.assign(wh, body);
          }),
        );
        try {
          await queryFulfilled;
        } catch {
          patch.undo();
        }
      },
      invalidatesTags: ["Webhooks"],
    }),
    deleteWebhook: builder.mutation<void, number>({
      query: (id) => ({ url: `/admin/webhooks/${id}`, method: "DELETE" }),
      invalidatesTags: ["Webhooks"],
    }),

    // ── Config (Settings) ──
    getConfig: builder.query<AdminSetting[], void>({
      query: () => "/admin/config",
      providesTags: ["Config"],
    }),
    updateConfig: builder.mutation<void, AdminSetting[]>({
      query: (body) => ({ url: "/admin/config", method: "PUT", body }),
      invalidatesTags: ["Config"],
    }),

    // ── Logs ──
    getLogs: builder.query<AdminLog[], LogQueryParams>({
      query: (params) => ({
        url: "/admin/logs",
        params: {
          from: params.from,
          to: params.to,
          level: params.level,
        },
      }),
      providesTags: ["Logs"],
    }),

    // ── Import / Export ──
    importTalkgroups: builder.mutation<void, FormData>({
      query: (body) => ({
        url: "/admin/import/talkgroups",
        method: "POST",
        body,
      }),
      invalidatesTags: ["Talkgroups"],
    }),
    importUnits: builder.mutation<void, FormData>({
      query: (body) => ({
        url: "/admin/import/units",
        method: "POST",
        body,
      }),
      invalidatesTags: ["Units"],
    }),
    exportConfig: builder.query<unknown, void>({
      query: () => "/admin/export/config",
    }),
    importConfig: builder.mutation<void, unknown>({
      query: (body) => ({
        url: "/admin/import/config",
        method: "POST",
        body,
      }),
      invalidatesTags: [
        "Users",
        "Systems",
        "Talkgroups",
        "Units",
        "Groups",
        "Tags",
        "ApiKeys",
        "Dirwatches",
        "Downstreams",
        "Webhooks",
        "Config",
      ],
    }),

    // ── Maintenance ──
    getMissingAudioCalls: builder.query<
      MissingAudioResponse,
      { limit?: number; offset?: number } | void
    >({
      query: (params) => ({
        url: "/admin/tools/audio-missing",
        params: {
          limit: params?.limit,
          offset: params?.offset,
        },
      }),
    }),
    cleanupMissingAudioCalls: builder.mutation<
      MissingAudioCleanupResponse,
      { confirm: boolean; callIds: number[] }
    >({
      query: (body) => ({
        url: "/admin/tools/audio-missing/cleanup",
        method: "POST",
        body,
      }),
    }),
  }),
});

export const {
  // Users
  useListUsersQuery,
  useCreateUserMutation,
  useUpdateUserMutation,
  useDeleteUserMutation,
  // Systems
  useListSystemsQuery,
  useCreateSystemMutation,
  useUpdateSystemMutation,
  useReorderSystemsMutation,
  useDeleteSystemMutation,
  // Talkgroups
  useListTalkgroupsQuery,
  useCreateTalkgroupMutation,
  useUpdateTalkgroupMutation,
  useDeleteTalkgroupMutation,
  // Units
  useListUnitsQuery,
  useCreateUnitMutation,
  useUpdateUnitMutation,
  useDeleteUnitMutation,
  // Groups
  useListGroupsQuery,
  useCreateGroupMutation,
  useUpdateGroupMutation,
  useDeleteGroupMutation,
  // Tags
  useListTagsQuery,
  useCreateTagMutation,
  useUpdateTagMutation,
  useDeleteTagMutation,
  // API Keys
  useListApiKeysQuery,
  useCreateApiKeyMutation,
  useUpdateApiKeyMutation,
  useReorderApiKeysMutation,
  useDeleteApiKeyMutation,
  // Dirwatches
  useLazyListServerDirectoriesQuery,
  useListDirwatchesQuery,
  useCreateDirwatchMutation,
  useUpdateDirwatchMutation,
  useDeleteDirwatchMutation,
  // Downstreams
  useListDownstreamsQuery,
  useCreateDownstreamMutation,
  useUpdateDownstreamMutation,
  useDeleteDownstreamMutation,
  // Webhooks
  useListWebhooksQuery,
  useCreateWebhookMutation,
  useUpdateWebhookMutation,
  useDeleteWebhookMutation,
  // Config
  useGetConfigQuery,
  useUpdateConfigMutation,
  // Logs
  useGetLogsQuery,
  // Import / Export
  useImportTalkgroupsMutation,
  useImportUnitsMutation,
  useExportConfigQuery,
  useLazyExportConfigQuery,
  useImportConfigMutation,
  useLazyGetMissingAudioCallsQuery,
  useCleanupMissingAudioCallsMutation,
} = adminApi;
