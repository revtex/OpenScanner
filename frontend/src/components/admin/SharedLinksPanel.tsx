import { Trash2, ExternalLink } from "lucide-react";
import {
  useGetSharedLinksQuery,
  useDeleteSharedLinkMutation,
} from "@/app/slices/shareSlice";

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
        <div className="overflow-x-auto">
          <table className="table table-zebra w-full">
            <thead>
              <tr>
                <th>System</th>
                <th>Talkgroup</th>
                <th>Call Date</th>
                <th>Duration</th>
                <th>Shared By</th>
                <th>Shared At</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {links.map((link) => (
                <tr key={link.id}>
                  <td>{link.systemLabel || "-"}</td>
                  <td>
                    {link.talkgroupLabel || "-"}
                    {link.talkgroupName && (
                      <span className="text-base-content/60 ml-1 text-xs">
                        ({link.talkgroupName})
                      </span>
                    )}
                  </td>
                  <td>{formatDate(link.dateTime)}</td>
                  <td>{formatDuration(link.duration)}</td>
                  <td>{link.sharedBy}</td>
                  <td>{formatDate(link.createdAt)}</td>
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
                        title="Delete shared link"
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
      )}
    </div>
  );
}
