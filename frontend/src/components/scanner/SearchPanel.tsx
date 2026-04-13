import { useCallback, useMemo, useRef } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import {
  X,
  Play,
  Download,
  Star,
  ChevronLeft,
  ChevronRight,
  RotateCcw,
} from "lucide-react";
import { useAppSelector, useAppDispatch } from "@/app/store";
import {
  useSearchCallsQuery,
  type CallSearchResult,
  setSystemFilter,
  setTalkgroupFilter,
  setGroupFilter,
  setTagFilter,
  setDateFrom,
  setDateTo,
  setSort,
  setPage,
  setBookmarkedOnly,
  setDownloadMode,
  setTranscript,
  resetFilters,
} from "@/app/slices/callsSlice";
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

function formatDuration(secs: number): string {
  if (!secs) return "";
  if (secs < 60) return `${secs}s`;
  const m = Math.floor(secs / 60);
  const s = secs % 60;
  return s > 0 ? `${m}m${s}s` : `${m}m`;
}

export default function SearchPanel({ isOpen, onClose }: SearchPanelProps) {
  const dispatch = useAppDispatch();
  const filters = useAppSelector((s) => s.calls);
  const config = useAppSelector((s) => s.scanner.config);
  const token = useAppSelector((s) => s.auth.token);
  const parentRef = useRef<HTMLDivElement>(null);

  const systems = useMemo(() => config?.systems ?? [], [config]);

  // Derive talkgroups filtered by selected system
  const talkgroups = useMemo(() => {
    if (filters.systemId) {
      const sys = systems.find((s) => s.id === filters.systemId);
      return sys?.talkgroups ?? [];
    }
    return systems.flatMap((s) => s.talkgroups ?? []);
  }, [systems, filters.systemId]);

  // Derive unique groups
  const groups = useMemo(() => {
    const set = new Set<string>();
    for (const tg of talkgroups) {
      if (tg.group) set.add(tg.group);
    }
    return [...set].sort();
  }, [talkgroups]);

  // Derive unique tags
  const tags = useMemo(() => {
    const set = new Set<string>();
    for (const tg of talkgroups) {
      if (tg.tag) set.add(tg.tag);
    }
    return [...set].sort();
  }, [talkgroups]);

  // Build query params
  const queryParams = useMemo(() => {
    const params: Record<string, number | string | boolean | undefined> = {
      systemId: filters.systemId,
      talkgroupId: filters.talkgroupId,
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

  const { data, isFetching } = useSearchCallsQuery(queryParams, {
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
    if (filters.systemId) count++;
    if (filters.talkgroupId) count++;
    if (filters.groupFilter) count++;
    if (filters.tagFilter) count++;
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
        if (filters.downloadMode) {
          const a = document.createElement("a");
          a.href = audioUrl;
          a.download = call.audioName || `call-${call.id}.mp3`;
          document.body.appendChild(a);
          a.click();
          document.body.removeChild(a);
          URL.revokeObjectURL(audioUrl);
          return;
        }

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
          systemLabel: call.systemLabel,
          talkgroupLabel: call.talkgroupLabel,
          talkgroupName: call.talkgroupName,
          talkgroupTag: call.talkgroupTag,
          talkgroupGroup: call.talkgroupGroup,
          transcript: call.transcript,
        };

        audioPlayer.play(playCall, audioUrl);
      } catch (err) {
        console.error("failed to play call", call.id, err);
      }
    },
    [filters.downloadMode, token],
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
        className={`fixed top-0 left-0 z-50 flex h-full w-full flex-col bg-base-100 transition-transform duration-300 ease-in-out sm:w-[500px] ${
          isOpen ? "translate-x-0" : "-translate-x-full"
        }`}
      >
        {/* Header */}
        <div className="flex items-center justify-between border-b border-base-300 px-4 py-3">
          <h2 className="text-lg font-semibold">Search Calls</h2>
          <button
            className="btn btn-ghost btn-sm btn-circle"
            onClick={onClose}
            aria-label="Close"
          >
            <X size={18} />
          </button>
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
                  className="absolute left-0 flex w-full cursor-pointer items-start gap-2 border-b border-base-300 px-3 py-1.5 hover:bg-base-200"
                  style={{ top: virtualRow.start }}
                  onClick={() => void handleRowClick(call)}
                >
                  {/* Play/Download icon */}
                  <span className="mt-1 shrink-0 text-primary">
                    {filters.downloadMode ? (
                      <Download size={14} />
                    ) : (
                      <Play size={14} />
                    )}
                  </span>

                  {/* Call details — two rows */}
                  <div className="min-w-0 flex-1">
                    {/* Row 1: talkgroup name, bookmark */}
                    <div className="flex items-center gap-1">
                      <span className="truncate text-xs font-medium">
                        {call.talkgroupName || call.talkgroupLabel}
                      </span>
                      {call.bookmarked && (
                        <Star
                          size={11}
                          className="shrink-0 fill-warning text-warning"
                        />
                      )}
                    </div>
                    {/* Row 2: system, tag, date/time */}
                    <div className="flex items-center gap-1 text-[11px] text-base-content/60">
                      <span className="truncate">{call.systemLabel}</span>
                      {call.talkgroupTag && (
                        <>
                          <span>·</span>
                          <span className="shrink-0">{call.talkgroupTag}</span>
                        </>
                      )}
                      <span className="ml-auto shrink-0">
                        {formatDate(call.dateTime)} {formatTime(call.dateTime)}
                      </span>
                    </div>
                    {/* Row 3: duration, frequency, source */}
                    <div className="flex items-center gap-2 text-[11px] text-base-content/40">
                      {call.duration > 0 && (
                        <span>{formatDuration(call.duration)}</span>
                      )}
                      {call.frequency > 0 && (
                        <span>{(call.frequency / 1e6).toFixed(4)} MHz</span>
                      )}
                      {call.source > 0 && <span>UID: {call.source}</span>}
                      {call.talkgroupId > 0 && (
                        <span>TGID: {call.talkgroupId}</span>
                      )}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        {/* Paginator + Download mode */}
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

          <label className="mt-2 flex cursor-pointer items-center gap-2">
            <input
              type="checkbox"
              className="toggle toggle-primary toggle-sm"
              checked={filters.downloadMode}
              onChange={(e) => dispatch(setDownloadMode(e.target.checked))}
            />
            <span className="text-sm">Download mode</span>
          </label>
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
              <label className="flex flex-col w-full">
                <span className="text-xs">System</span>
                <select
                  className="select select-sm w-full"
                  value={filters.systemId ?? ""}
                  onChange={(e) =>
                    dispatch(
                      setSystemFilter(
                        e.target.value ? Number(e.target.value) : undefined,
                      ),
                    )
                  }
                >
                  <option value="">All Systems</option>
                  {systems.map((sys) => (
                    <option key={sys.id} value={sys.id}>
                      {sys.label}
                    </option>
                  ))}
                </select>
              </label>

              {/* Talkgroup */}
              <label className="flex flex-col w-full">
                <span className="text-xs">Talkgroup</span>
                <select
                  className="select select-sm w-full"
                  value={filters.talkgroupId ?? ""}
                  onChange={(e) =>
                    dispatch(
                      setTalkgroupFilter(
                        e.target.value ? Number(e.target.value) : undefined,
                      ),
                    )
                  }
                >
                  <option value="">All Talkgroups</option>
                  {talkgroups.map((tg) => (
                    <option key={tg.id} value={tg.id}>
                      {tg.label} — {tg.name}
                    </option>
                  ))}
                </select>
              </label>

              {/* Group */}
              <label className="flex flex-col w-full">
                <span className="text-xs">Group</span>
                <select
                  className="select select-sm w-full"
                  value={filters.groupFilter ?? ""}
                  onChange={(e) =>
                    dispatch(setGroupFilter(e.target.value || undefined))
                  }
                >
                  <option value="">All Groups</option>
                  {groups.map((g) => (
                    <option key={g} value={g}>
                      {g}
                    </option>
                  ))}
                </select>
              </label>

              {/* Tag */}
              <label className="flex flex-col w-full">
                <span className="text-xs">Tag</span>
                <select
                  className="select select-sm w-full"
                  value={filters.tagFilter ?? ""}
                  onChange={(e) =>
                    dispatch(setTagFilter(e.target.value || undefined))
                  }
                >
                  <option value="">All Tags</option>
                  {tags.map((t) => (
                    <option key={t} value={t}>
                      {t}
                    </option>
                  ))}
                </select>
              </label>

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
