// sanitizeDownloadFilename strips path separators, control characters, and
// leading dots from a server-supplied filename before assigning it to
// <a download>. The browser treats `download` as a filename (not HTML), but
// a hostile server could still smuggle path traversal or confusing unicode,
// and static analysers flag any server string reaching `appendChild`.
export function sanitizeDownloadFilename(
  name: string | undefined | null,
  fallback: string,
): string {
  if (!name) return fallback;
  // Take only the basename — strip anything that looks like a path.
  const base = name.replace(/^.*[\\/]/, "");
  // Strip control characters and quote/angle-bracket injection attempts.
  const cleaned = base
    // eslint-disable-next-line no-control-regex
    .replace(/[\x00-\x1f\x7f"<>|]/g, "")
    .replace(/^\.+/, "")
    .trim();
  if (!cleaned) return fallback;
  // Cap length to avoid filesystem issues on some platforms.
  return cleaned.length > 200 ? cleaned.slice(0, 200) : cleaned;
}
