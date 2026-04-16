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
        <div className="overflow-x-auto rounded-xl border border-base-300 bg-base-200/40">
          <table className="table table-zebra table-fixed w-full">
            <thead>
              <tr>
                <th className="w-[30%]">System</th>
                <th className="w-[24%]">Talkgroup</th>
                <th className="w-44">Call Date</th>
                <th className="w-24">Duration</th>
                <th className="w-32">Shared By</th>
                <th className="w-44">Shared At</th>
                <th className="w-20">Actions</th>
              </tr>
            </thead>
            <tbody>
              {links.map((link) => (
                <tr key={link.id}>
                  <td className="align-top">
                    <div
                      className="leading-snug whitespace-normal wrap-break-word"
                      title={link.systemLabel || "-"}
                    >
                      {link.systemLabel || "-"}
                    </div>
                  </td>
                  <td className="align-top">
                    <div
                      className="leading-snug whitespace-normal wrap-break-word"
                      title={
                        link.talkgroupName
                          ? `${link.talkgroupLabel || "-"} (${link.talkgroupName})`
                          : link.talkgroupLabel || "-"
                      }
                    >
                      <div>{link.talkgroupLabel || "-"}</div>
                      {link.talkgroupName && (
                        <span className="mt-0.5 block text-xs text-base-content/60">
                          {link.talkgroupName}
                        </span>
                      )}
                    </div>
                  </td>
                  <td className="whitespace-nowrap text-sm">
                    {formatDate(link.dateTime)}
                  </td>
                  <td className="whitespace-nowrap font-medium">
                    {formatDuration(link.duration)}
                  </td>
                  <td>
                    <div className="truncate" title={link.sharedBy || "-"}>
                      {link.sharedBy || "-"}
                    </div>
                  </td>
                  <td className="whitespace-nowrap text-sm">
                    {formatDate(link.createdAt)}
                  </td>
                  <td className="align-top">
                    <div className="flex gap-1 whitespace-nowrap">
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
