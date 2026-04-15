import { useCallback, useEffect, useMemo, useRef } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import {
  X,
  Play,
  Download,
  Star,
  ChevronLeft,
  ChevronRight,
  RotateCcw,
  RefreshCw,
} from "lucide-react";
import { useAppSelector, useAppDispatch } from "@/app/store";
import {
  useSearchCallsQuery,
  type CallSearchParams,
  type CallSearchResult,
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
  setBookmarkedOnly,
  setTranscript,
  resetFilters,
} from "@/app/slices/callsSlice";
import { useGetBookmarkIDsQuery, useToggleBookmarkMutation } from "@/app/api";
import { selectToken } from "@/app/slices/authSlice";
import { audioPlayer } from "@/services/audioPlayer";
import type { Call } from "@/types";

interface SearchPanelProps {
  isOpen: boolean;
  onClose: () => void;
}

function formatTime(unix: number): string {
  const d = new Date(unix * 1000);
  return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

function formatDate(unix: number): string {
  const d = new Date(unix * 1000);
  return d.toLocaleDateString([], { month: "short", day: "numeric" });
}

export default function SearchPanel({ isOpen, onClose }: SearchPanelProps) {
  const dispatch = useAppDispatch();
  const filters = useAppSelector((s) => s.calls);
  const config = useAppSelector((s) => s.scanner.config);
  const token = useAppSelector(selectToken);
  const isAuthenticated = !!token;

  const { data: bookmarkData } = useGetBookmarkIDsQuery(undefined, {
    skip: !isAuthenticated,
  });
  const [toggleBookmark] = useToggleBookmarkMutation();
  const bookmarkedCallIds = bookmarkData?.callIds ?? [];
  const parentRef = useRef<HTMLDivElement>(null);

  const systems = useMemo(() => config?.systems ?? [], [config]);

  const allTalkgroups = useMemo(() => {
    return systems.flatMap((sys) =>
      (sys.talkgroups ?? []).map((tg) => ({
        ...tg,
        systemId: sys.id,
        systemLabel: sys.label,
      })),
    );
  }, [systems]);

  const filterRows = useMemo(
    () =>
      allTalkgroups.map((tg) => ({
        systemId: tg.systemId,
        talkgroupId: tg.id,
        group: tg.group ?? "",
        tag: tg.tag ?? "",
      })),
    [allTalkgroups],
  );

  const selectedSystemIds = filters.systemIds;
  const selectedTalkgroupIds = filters.talkgroupIds;
  const selectedGroups = filters.groupFilters;
  const selectedTags = filters.tagFilters;

  const matchWith = useCallback(
    (
      row: {
        systemId: number;
        talkgroupId: number;
        group: string;
        tag: string;
      },
      opts: {
        systemIds?: number[];
        talkgroupIds?: number[];
        groups?: string[];
        tags?: string[];
      },
    ) => {
      if (opts.systemIds && opts.systemIds.length > 0) {
        if (!opts.systemIds.includes(row.systemId)) return false;
      }
      if (opts.talkgroupIds && opts.talkgroupIds.length > 0) {
        if (!opts.talkgroupIds.includes(row.talkgroupId)) return false;
      }
      if (opts.groups && opts.groups.length > 0) {
        if (!opts.groups.includes(row.group)) return false;
      }
      if (opts.tags && opts.tags.length > 0) {
        if (!opts.tags.includes(row.tag)) return false;
      }
      return true;
    },
    [],
  );

  const availableSystemIds = useMemo(() => {
    const out = new Set<number>();
    for (const row of filterRows) {
      if (
        matchWith(row, {
          talkgroupIds: selectedTalkgroupIds,
          groups: selectedGroups,
          tags: selectedTags,
        })
      ) {
        out.add(row.systemId);
      }
    }
    return out;
  }, [
    filterRows,
    matchWith,
    selectedTalkgroupIds,
    selectedGroups,
    selectedTags,
  ]);

  const availableTalkgroupIds = useMemo(() => {
    const out = new Set<number>();
    for (const row of filterRows) {
      if (
        matchWith(row, {
          systemIds: selectedSystemIds,
          groups: selectedGroups,
          tags: selectedTags,
        })
      ) {
        out.add(row.talkgroupId);
      }
    }
    return out;
  }, [filterRows, matchWith, selectedSystemIds, selectedGroups, selectedTags]);

  const availableGroups = useMemo(() => {
    const out = new Set<string>();
    for (const row of filterRows) {
      if (
        matchWith(row, {
          systemIds: selectedSystemIds,
          talkgroupIds: selectedTalkgroupIds,
          tags: selectedTags,
        })
      ) {
        if (row.group) out.add(row.group);
      }
    }
    return [...out].sort((a, b) => a.localeCompare(b));
  }, [
    filterRows,
    matchWith,
    selectedSystemIds,
    selectedTalkgroupIds,
    selectedTags,
  ]);

  const availableTags = useMemo(() => {
    const out = new Set<string>();
    for (const row of filterRows) {
      if (
        matchWith(row, {
          systemIds: selectedSystemIds,
          talkgroupIds: selectedTalkgroupIds,
          groups: selectedGroups,
        })
      ) {
        if (row.tag) out.add(row.tag);
      }
    }
    return [...out].sort((a, b) => a.localeCompare(b));
  }, [
    filterRows,
    matchWith,
    selectedSystemIds,
    selectedTalkgroupIds,
    selectedGroups,
  ]);

  const availableSystems = useMemo(
    () => systems.filter((sys) => availableSystemIds.has(sys.id)),
    [systems, availableSystemIds],
  );
  const availableTalkgroups = useMemo(
    () => allTalkgroups.filter((tg) => availableTalkgroupIds.has(tg.id)),
    [allTalkgroups, availableTalkgroupIds],
  );

  // Keep selected filters valid when options are narrowed by other selections.
  useEffect(() => {
    const cleaned = selectedSystemIds.filter((id) =>
      availableSystemIds.has(id),
    );
    if (cleaned.length !== selectedSystemIds.length) {
      dispatch(setSystemFilters(cleaned));
    }
  }, [dispatch, selectedSystemIds, availableSystemIds]);

  useEffect(() => {
    const cleaned = selectedTalkgroupIds.filter((id) =>
      availableTalkgroupIds.has(id),
    );
    if (cleaned.length !== selectedTalkgroupIds.length) {
      dispatch(setTalkgroupFilters(cleaned));
    }
  }, [dispatch, selectedTalkgroupIds, availableTalkgroupIds]);

  useEffect(() => {
    const cleaned = selectedGroups.filter((g) => availableGroups.includes(g));
    if (cleaned.length !== selectedGroups.length) {
      dispatch(setGroupFilters(cleaned));
    }
  }, [dispatch, selectedGroups, availableGroups]);

  useEffect(() => {
    const cleaned = selectedTags.filter((t) => availableTags.includes(t));
    if (cleaned.length !== selectedTags.length) {
      dispatch(setTagFilters(cleaned));
    }
  }, [dispatch, selectedTags, availableTags]);

  // Build query params
  const queryParams = useMemo(() => {
    const params: CallSearchParams = {
      systemIds: filters.systemIds,
      talkgroupIds: filters.talkgroupIds,
      groupFilters: filters.groupFilters,
      tagFilters: filters.tagFilters,
      transcript: filters.transcript,
      page: filters.page,
      limit: filters.limit,
      sort: filters.sort,
    };
    if (filters.dateFrom) {
      params.dateFrom = Math.floor(new Date(filters.dateFrom).getTime() / 1000);
    }
    if (filters.dateTo) {
      // End of day
      params.dateTo = Math.floor(
        new Date(filters.dateTo + "T23:59:59").getTime() / 1000,
      );
    }
    if (filters.bookmarkedOnly) {
      params.bookmarkedOnly = true;
    }
    return params;
  }, [filters]);

  const { data, isFetching, refetch } = useSearchCallsQuery(queryParams, {
    skip: !isOpen,
  });

  const calls = data?.calls ?? [];
  const total = data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / filters.limit));

  // eslint-disable-next-line react-hooks/incompatible-library
  const virtualizer = useVirtualizer({
    count: calls.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 62,
    overscan: 5,
  });

  // Count active filters
  const activeFilterCount = useMemo(() => {
    let count = 0;
    if (filters.systemIds.length > 0) count++;
    if (filters.talkgroupIds.length > 0) count++;
    if (filters.groupFilters.length > 0) count++;
    if (filters.tagFilters.length > 0) count++;
    if (filters.dateFrom) count++;
    if (filters.dateTo) count++;
    if (filters.bookmarkedOnly) count++;
    if (filters.transcript) count++;
    if (filters.sort !== "desc") count++;
    return count;
  }, [filters]);

  const handleRowClick = useCallback(
    async (call: CallSearchResult) => {
      try {
        const headers: HeadersInit = {};
        if (token) {
          headers.Authorization = `Bearer ${token}`;
        }

        const resp = await fetch(`/api/calls/${call.id}/audio`, { headers });
        if (!resp.ok) {
          console.error("failed to load call audio", call.id, resp.status);
          return;
        }

        const blob = await resp.blob();
        const audioUrl = URL.createObjectURL(blob);

        const playCall: Call = {
          id: call.id,
          audioName: call.audioName || `call-${call.id}`,
          audioType: call.audioType || blob.type || "audio/mpeg",
          dateTime: call.dateTime,
          systemId: call.systemId,
          system: call.systemId,
          talkgroupId: call.talkgroupId,
          talkgroup: call.talkgroupId,
          frequency: call.frequency,
          duration: call.duration,
          source: call.source,
          site: call.site,
          channel: call.channel,
          decoder: call.decoder,
          errorCount: call.errorCount,
          spikeCount: call.spikeCount,
          talkerAlias: call.talkerAlias,
          systemLabel: call.systemLabel,
          talkgroupLabel: call.talkgroupLabel,
          talkgroupName: call.talkgroupName,
          talkgroupTag: call.talkgroupTag,
          talkgroupGroup: call.talkgroupGroup,
          transcript: call.transcript,
        };

        audioPlayer.playNow(playCall, audioUrl);
      } catch (err) {
        console.error("failed to play call", call.id, err);
      }
    },
    [token],
  );

  const handleDownload = useCallback(
    async (call: CallSearchResult) => {
      try {
        const headers: HeadersInit = {};
        if (token) {
          headers.Authorization = `Bearer ${token}`;
        }

        const resp = await fetch(`/api/calls/${call.id}/audio`, { headers });
        if (!resp.ok) {
          console.error("failed to download call audio", call.id, resp.status);
          return;
        }

        const blob = await resp.blob();
        const url = URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = call.audioName || `call-${call.id}.mp3`;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);
      } catch (err) {
        console.error("failed to download call", call.id, err);
      }
    },
    [token],
  );

  return (
    <>
      {/* Backdrop */}
      {isOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/50"
          onClick={onClose}
          aria-hidden
        />
      )}

      {/* Panel */}
      <div
        className={`fixed top-0 left-0 z-50 flex h-full w-full flex-col bg-base-100 transition-transform duration-300 ease-in-out sm:w-125 ${
          isOpen ? "translate-x-0" : "-translate-x-full"
        }`}
      >
        {/* Header */}
        <div className="flex items-center justify-between border-b border-base-300 px-4 py-3">
          <h2 className="text-lg font-semibold">Search Calls</h2>
          <div className="flex items-center gap-1">
            <button
              className="btn btn-ghost btn-sm btn-circle"
              onClick={() => refetch()}
              aria-label="Refresh"
              disabled={isFetching}
            >
              <RefreshCw
                size={16}
                className={isFetching ? "animate-spin" : ""}
              />
            </button>
            <button
              className="btn btn-ghost btn-sm btn-circle"
              onClick={onClose}
              aria-label="Close"
            >
              <X size={18} />
            </button>
          </div>
        </div>

        {/* Results list */}
        <div ref={parentRef} className="relative flex-1 overflow-y-auto">
          {isFetching && (
            <div className="absolute inset-0 z-10 flex items-center justify-center bg-base-100/60">
              <span className="loading loading-spinner loading-md" />
            </div>
          )}

          {calls.length === 0 && !isFetching && (
            <div className="flex h-32 items-center justify-center text-base-content/50">
              No results
            </div>
          )}

          <div
            className="relative w-full"
            style={{ height: virtualizer.getTotalSize() }}
          >
            {virtualizer.getVirtualItems().map((virtualRow) => {
              const call = calls[virtualRow.index];
              return (
                <div
                  key={call.id}
                  data-index={virtualRow.index}
                  ref={virtualizer.measureElement}
                  className="absolute left-0 flex w-full items-start gap-2 border-b border-base-300 px-3 py-2 hover:bg-base-200"
                  style={{ top: virtualRow.start }}
                >
                  {/* Call details */}
                  <div className="min-w-0 flex-1">
                    {/* Row 1: talkgroup name */}
                    <div className="text-xs font-medium truncate">
                      {call.talkgroupName || call.talkgroupLabel}
                    </div>
                    {/* Row 2: system */}
                    <div className="text-[11px] text-base-content/60 truncate">
                      {call.systemLabel}
                    </div>
                    {/* Row 3: freq, UID, TGID */}
                    <div className="flex items-center gap-2 text-[11px] text-base-content/40">
                      {call.frequency > 0 && (
                        <span>{(call.frequency / 1e6).toFixed(4)} MHz</span>
                      )}
                      {call.source > 0 && <span>UID: {call.source}</span>}
                      {call.talkerAlias && <span>{call.talkerAlias}</span>}
                      {call.talkgroupId > 0 && (
                        <span>TGID: {call.talkgroupId}</span>
                      )}
                      {call.errorCount != null && call.errorCount > 0 && (
                        <span>E:{call.errorCount}</span>
                      )}
                      {call.spikeCount != null && call.spikeCount > 0 && (
                        <span>S:{call.spikeCount}</span>
                      )}
                    </div>
                  </div>
                  {/* Date/time + action buttons */}
                  <div className="flex shrink-0 flex-col items-end gap-0.5">
                    <span className="text-[11px] text-base-content/60">
                      {formatDate(call.dateTime)} {formatTime(call.dateTime)}
                    </span>
                    {call.duration > 0 && (
                      <span className="text-[11px] text-base-content/40">
                        {call.duration}s
                      </span>
                    )}
                    <div className="flex items-center gap-0.5">
                      <button
                        onClick={() => void handleRowClick(call)}
                        className="btn btn-ghost btn-xs btn-square"
                        aria-label="Play call"
                      >
                        <Play className="w-3 h-3" />
                      </button>
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          void handleDownload(call);
                        }}
                        className="btn btn-ghost btn-xs btn-square"
                        aria-label="Download call"
                      >
                        <Download className="w-3 h-3" />
                      </button>
                      {isAuthenticated && (
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            void toggleBookmark(call.id);
                          }}
                          className={`btn btn-ghost btn-xs btn-square ${
                            bookmarkedCallIds.includes(call.id)
                              ? "text-warning"
                              : ""
                          }`}
                          aria-label="Toggle bookmark"
                        >
                          <Star
                            className={`w-3 h-3 ${
                              bookmarkedCallIds.includes(call.id)
                                ? "fill-current"
                                : ""
                            }`}
                          />
                        </button>
                      )}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        {/* Paginator */}
        <div className="border-t border-base-300 px-4 py-2">
          <div className="flex items-center justify-between">
            <div className="join">
              <button
                className="join-item btn btn-sm"
                disabled={filters.page <= 1}
                onClick={() => dispatch(setPage(filters.page - 1))}
              >
                <ChevronLeft size={14} />
                Prev
              </button>
              <button className="join-item btn btn-sm btn-disabled pointer-events-none">
                Page {filters.page} of {totalPages}
              </button>
              <button
                className="join-item btn btn-sm"
                disabled={filters.page >= totalPages}
                onClick={() => dispatch(setPage(filters.page + 1))}
              >
                Next
                <ChevronRight size={14} />
              </button>
            </div>
          </div>
        </div>

        {/* Filters section */}
        <div className="border-t border-base-300">
          <div className="collapse collapse-arrow">
            <input type="checkbox" />
            <div className="collapse-title text-sm font-medium">
              Filters
              {activeFilterCount > 0 && (
                <span className="badge badge-primary badge-sm ml-2">
                  {activeFilterCount}
                </span>
              )}
            </div>
            <div className="collapse-content space-y-3">
              {/* Transcript */}
              <label className="flex flex-col w-full">
                <span className="text-xs">Transcript</span>
                <input
                  type="text"
                  className="input input-sm w-full"
                  placeholder="Search transcripts…"
                  value={filters.transcript ?? ""}
                  onChange={(e) =>
                    dispatch(setTranscript(e.target.value || undefined))
                  }
                />
              </label>

              {/* System */}
              <div className="flex flex-col w-full gap-1">
                <div className="flex items-center justify-between">
                  <span className="text-xs">System</span>
                  <button
                    className="btn btn-ghost btn-xs"
                    onClick={() => dispatch(setSystemFilters([]))}
                    disabled={filters.systemIds.length === 0}
                  >
                    Clear
                  </button>
                </div>
                <div className="max-h-28 overflow-y-auto rounded border border-base-300 p-2 space-y-1">
                  {availableSystems.map((sys) => (
                    <label
                      key={sys.id}
                      className="label cursor-pointer justify-start gap-2 py-0.5"
                    >
                      <input
                        type="checkbox"
                        className="checkbox checkbox-xs"
                        checked={filters.systemIds.includes(sys.id)}
                        onChange={() => dispatch(toggleSystemFilter(sys.id))}
                      />
                      <span className="label-text text-xs">{sys.label}</span>
                    </label>
                  ))}
                </div>
              </div>

              {/* Talkgroup */}
              <div className="flex flex-col w-full gap-1">
                <div className="flex items-center justify-between">
                  <span className="text-xs">Talkgroup</span>
                  <button
                    className="btn btn-ghost btn-xs"
                    onClick={() => dispatch(setTalkgroupFilters([]))}
                    disabled={filters.talkgroupIds.length === 0}
                  >
                    Clear
                  </button>
                </div>
                <div className="max-h-36 overflow-y-auto rounded border border-base-300 p-2 space-y-1">
                  {availableTalkgroups.map((tg) => (
                    <label
                      key={tg.id}
                      className="label cursor-pointer justify-start gap-2 py-0.5"
                    >
                      <input
                        type="checkbox"
                        className="checkbox checkbox-xs"
                        checked={filters.talkgroupIds.includes(tg.id)}
                        onChange={() => dispatch(toggleTalkgroupFilter(tg.id))}
                      />
                      <span className="label-text text-xs truncate">
                        {tg.label} — {tg.name}
                      </span>
                    </label>
                  ))}
                </div>
              </div>

              {/* Group */}
              <div className="flex flex-col w-full gap-1">
                <div className="flex items-center justify-between">
                  <span className="text-xs">Group</span>
                  <button
                    className="btn btn-ghost btn-xs"
                    onClick={() => dispatch(setGroupFilters([]))}
                    disabled={filters.groupFilters.length === 0}
                  >
                    Clear
                  </button>
                </div>
                <div className="max-h-28 overflow-y-auto rounded border border-base-300 p-2 space-y-1">
                  {availableGroups.map((g) => (
                    <label
                      key={g}
                      className="label cursor-pointer justify-start gap-2 py-0.5"
                    >
                      <input
                        type="checkbox"
                        className="checkbox checkbox-xs"
                        checked={filters.groupFilters.includes(g)}
                        onChange={() => dispatch(toggleGroupFilter(g))}
                      />
                      <span className="label-text text-xs">{g}</span>
                    </label>
                  ))}
                </div>
              </div>

              {/* Tag */}
              <div className="flex flex-col w-full gap-1">
                <div className="flex items-center justify-between">
                  <span className="text-xs">Tag</span>
                  <button
                    className="btn btn-ghost btn-xs"
                    onClick={() => dispatch(setTagFilters([]))}
                    disabled={filters.tagFilters.length === 0}
                  >
                    Clear
                  </button>
                </div>
                <div className="max-h-28 overflow-y-auto rounded border border-base-300 p-2 space-y-1">
                  {availableTags.map((t) => (
                    <label
                      key={t}
                      className="label cursor-pointer justify-start gap-2 py-0.5"
                    >
                      <input
                        type="checkbox"
                        className="checkbox checkbox-xs"
                        checked={filters.tagFilters.includes(t)}
                        onChange={() => dispatch(toggleTagFilter(t))}
                      />
                      <span className="label-text text-xs">{t}</span>
                    </label>
                  ))}
                </div>
              </div>

              {/* Date from */}
              <label className="flex flex-col w-full">
                <span className="text-xs">Date from</span>
                <input
                  type="date"
                  className="input input-sm w-full"
                  value={filters.dateFrom ?? ""}
                  onChange={(e) =>
                    dispatch(setDateFrom(e.target.value || undefined))
                  }
                />
              </label>

              {/* Date to */}
              <label className="flex flex-col w-full">
                <span className="text-xs">Date to</span>
                <input
                  type="date"
                  className="input input-sm w-full"
                  value={filters.dateTo ?? ""}
                  onChange={(e) =>
                    dispatch(setDateTo(e.target.value || undefined))
                  }
                />
              </label>

              {/* Sort */}
              <label className="flex flex-col w-full">
                <span className="text-xs">Sort</span>
                <select
                  className="select select-sm w-full"
                  value={filters.sort}
                  onChange={(e) =>
                    dispatch(setSort(e.target.value as "asc" | "desc"))
                  }
                >
                  <option value="desc">Newest first</option>
                  <option value="asc">Oldest first</option>
                </select>
              </label>

              {/* Bookmarked only */}
              <label className="flex cursor-pointer items-center gap-2">
                <input
                  type="checkbox"
                  className="toggle toggle-sm"
                  checked={filters.bookmarkedOnly}
                  onChange={(e) =>
                    dispatch(setBookmarkedOnly(e.target.checked))
                  }
                />
                <span className="text-sm">Bookmarked only</span>
              </label>

              {/* Reset */}
              <button
                className="btn btn-ghost btn-sm"
                onClick={() => dispatch(resetFilters())}
              >
                <RotateCcw size={14} />
                Reset filters
              </button>
            </div>
          </div>
        </div>
      </div>
    </>
  );
}
