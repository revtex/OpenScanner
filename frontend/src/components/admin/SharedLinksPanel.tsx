import { Trash2, ExternalLink } from "lucide-react";
import {
  useGetSharedLinksQuery,
  useDeleteSharedLinkMutation,
} from "@/hooks/useAdminWsOps";

function formatDate(unix: number): string {
  return new Date(unix * 1000).toLocaleString();
}

function formatDuration(secs: number): string {
  if (!secs) return "-";
  const minutes = Math.floor(secs / 60);
  const seconds = secs % 60;
  return `${minutes}:${seconds.toString().padStart(2, "0")}`;
}

export default function SharedLinksPanel() {
  const { data: links, isLoading, isError } = useGetSharedLinksQuery();
  const [deleteLink] = useDeleteSharedLinkMutation();

  const handleDelete = (id: number) => {
    if (
      confirm(
        "Remove this shared link? The call will no longer be accessible via its share URL and will become eligible for pruning.",
      )
    ) {
      void deleteLink(id);
    }
  };

  if (isLoading) {
    return (
      <div className="flex justify-center py-12">
        <span className="loading loading-spinner loading-lg" />
      </div>
    );
  }

  if (isError) {
    return (
      <div className="alert alert-error">Failed to load shared links.</div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-bold">Shared Links</h2>
        <span className="badge badge-neutral">{links?.length ?? 0} total</span>
      </div>

      {!links?.length ? (
        <div className="text-base-content/60 py-8 text-center">
          No calls have been shared yet.
        </div>
      ) : (
        <>
          {/* Desktop table */}
          <div className="hidden md:block overflow-x-auto rounded-xl border border-base-300 bg-base-200/40">
            <table className="table table-zebra w-full">
              <thead>
                <tr>
                  <th>System</th>
                  <th>Talkgroup</th>
                  <th>Call Date</th>
                  <th>Duration</th>
                  <th>Shared By</th>
                  <th>Shared At</th>
                  <th>Expires</th>
                  <th className="w-20">Actions</th>
                </tr>
              </thead>
              <tbody>
                {links.map((link) => (
                  <tr key={link.id}>
                    <td>{link.systemLabel || "-"}</td>
                    <td>
                      <div>{link.talkgroupLabel || "-"}</div>
                      {link.talkgroupName && (
                        <span className="text-xs text-base-content/60">
                          {link.talkgroupName}
                        </span>
                      )}
                    </td>
                    <td className="whitespace-nowrap text-sm">
                      {formatDate(link.dateTime)}
                    </td>
                    <td className="whitespace-nowrap font-medium">
                      {formatDuration(link.duration)}
                    </td>
                    <td className="truncate">{link.sharedBy || "-"}</td>
                    <td className="whitespace-nowrap text-sm">
                      {formatDate(link.createdAt)}
                    </td>
                    <td className="whitespace-nowrap text-sm">
                      {link.expiresAt ? (
                        formatDate(link.expiresAt)
                      ) : (
                        <span className="text-base-content/40">Never</span>
                      )}
                    </td>
                    <td>
                      <div className="flex gap-1">
                        <a
                          href={`/call/${link.token}`}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="btn btn-ghost btn-xs btn-square"
                          title="Open shared link"
                        >
                          <ExternalLink className="w-4 h-4" />
                        </a>
                        <button
                          className="btn btn-ghost btn-xs btn-square text-error"
                          onClick={() => handleDelete(link.id)}
                          title="Revoke shared link"
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Mobile card list */}
          <div className="md:hidden space-y-3">
            {links.map((link) => (
              <div
                key={link.id}
                className="card bg-base-200 card-body p-3 gap-2"
              >
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0">
                    <div className="font-medium text-sm truncate">
                      {link.systemLabel || "-"}
                    </div>
                    <div className="text-xs text-base-content/60 truncate">
                      {link.talkgroupLabel || "-"}
                      {link.talkgroupName && ` — ${link.talkgroupName}`}
                    </div>
                  </div>
                  <div className="flex gap-1 shrink-0">
                    <a
                      href={`/call/${link.token}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="btn btn-ghost btn-xs btn-square"
                    >
                      <ExternalLink className="w-4 h-4" />
                    </a>
                    <button
                      className="btn btn-ghost btn-xs btn-square text-error"
                      onClick={() => handleDelete(link.id)}
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </div>
                </div>
                <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-base-content/60">
                  <span>{formatDate(link.dateTime)}</span>
                  <span className="font-medium text-base-content">
                    {formatDuration(link.duration)}
                  </span>
                  <span>by {link.sharedBy || "-"}</span>
                  <span>
                    Expires:{" "}
                    {link.expiresAt ? formatDate(link.expiresAt) : "Never"}
                  </span>
                </div>
              </div>
            ))}
          </div>
        </>
      )}
    </div>
  );
}
