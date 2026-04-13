import {
  useGetActivityStatsQuery,
  useGetActivityChartQuery,
  useGetTopTalkgroupsQuery,
} from "@/app/slices/activitySlice";
import type { ChartBucket } from "@/app/slices/activitySlice";

const POLL_INTERVAL = 30_000;

function formatUptime(totalSeconds: number): string {
  const days = Math.floor(totalSeconds / 86400);
  const hours = Math.floor((totalSeconds % 86400) / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const parts: string[] = [];
  if (days > 0) parts.push(`${days}d`);
  if (hours > 0 || days > 0) parts.push(`${hours}h`);
  parts.push(`${minutes}m`);
  return parts.join(" ");
}

function formatHourLabel(unixHour: number): string {
  const date = new Date(unixHour * 1000);
  return date.toLocaleTimeString(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}

function Sparkline({ buckets }: { buckets: ChartBucket[] }) {
  const maxCount = Math.max(...buckets.map((b) => b.count), 1);

  return (
    <div className="flex items-end gap-[2px] h-32">
      {buckets.map((bucket, i) => {
        const heightPct = (bucket.count / maxCount) * 100;
        return (
          <div
            key={bucket.hour}
            className="flex-1 flex flex-col items-center justify-end h-full"
          >
            <div
              className="w-full bg-primary rounded-t-sm min-h-[2px] tooltip tooltip-top"
              data-tip={`${bucket.count} calls`}
              style={{ height: `${Math.max(heightPct, 2)}%` }}
            />
            {i % 4 === 0 && (
              <span className="text-[9px] text-base-content/50 mt-1 leading-none">
                {formatHourLabel(bucket.hour)}
              </span>
            )}
          </div>
        );
      })}
    </div>
  );
}

export default function ActivityPanel() {
  const { data: stats, isLoading: statsLoading } = useGetActivityStatsQuery(
    undefined,
    { pollingInterval: POLL_INTERVAL },
  );
  const { data: chart, isLoading: chartLoading } = useGetActivityChartQuery(
    undefined,
    { pollingInterval: POLL_INTERVAL },
  );
  const { data: topTG, isLoading: topLoading } = useGetTopTalkgroupsQuery(
    undefined,
    { pollingInterval: POLL_INTERVAL },
  );

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Activity</h1>

      {/* Stat cards */}
      {statsLoading ? (
        <div className="flex justify-center p-8">
          <span className="loading loading-spinner loading-md" />
        </div>
      ) : stats ? (
        <div className="stats stats-vertical sm:stats-horizontal shadow w-full bg-base-200">
          <div className="stat">
            <div className="stat-title">Calls Today</div>
            <div className="stat-value text-primary">
              {stats.callsToday.toLocaleString()}
            </div>
          </div>
          <div className="stat">
            <div className="stat-title">This Week</div>
            <div className="stat-value">
              {stats.callsThisWeek.toLocaleString()}
            </div>
          </div>
          <div className="stat">
            <div className="stat-title">Total Calls</div>
            <div className="stat-value">
              {stats.callsTotal.toLocaleString()}
            </div>
          </div>
          <div className="stat">
            <div className="stat-title">Listeners</div>
            <div className="stat-value text-secondary">
              {stats.activeListeners}
            </div>
          </div>
          <div className="stat">
            <div className="stat-title">Uptime</div>
            <div className="stat-value text-sm">
              {formatUptime(stats.uptime)}
            </div>
          </div>
        </div>
      ) : null}

      {/* Calls per hour chart */}
      <div className="card bg-base-200 shadow">
        <div className="card-body">
          <h2 className="card-title text-base">Calls / Hour (Last 24h)</h2>
          {chartLoading ? (
            <div className="flex justify-center p-4">
              <span className="loading loading-spinner loading-sm" />
            </div>
          ) : chart && chart.buckets.length > 0 ? (
            <Sparkline buckets={chart.buckets} />
          ) : (
            <p className="text-sm text-base-content/50">No data available</p>
          )}
        </div>
      </div>

      {/* Top talkgroups */}
      <div className="card bg-base-200 shadow">
        <div className="card-body">
          <h2 className="card-title text-base">
            Top 10 Busiest Talkgroups (Last 24h)
          </h2>
          {topLoading ? (
            <div className="flex justify-center p-4">
              <span className="loading loading-spinner loading-sm" />
            </div>
          ) : topTG && topTG.talkgroups.length > 0 ? (
            <div className="overflow-x-auto">
              <table className="table table-zebra table-sm">
                <thead>
                  <tr>
                    <th>#</th>
                    <th>Talkgroup</th>
                    <th>System</th>
                    <th className="text-right">Calls</th>
                  </tr>
                </thead>
                <tbody>
                  {topTG.talkgroups.map((tg, i) => (
                    <tr key={tg.talkgroupId}>
                      <td className="text-base-content/50">{i + 1}</td>
                      <td>{tg.talkgroupLabel}</td>
                      <td className="text-base-content/70">{tg.systemLabel}</td>
                      <td className="text-right font-mono">
                        {tg.callCount.toLocaleString()}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <p className="text-sm text-base-content/50">No data available</p>
          )}
        </div>
      </div>
    </div>
  );
}
