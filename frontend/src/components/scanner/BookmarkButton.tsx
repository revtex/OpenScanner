import { Star } from "lucide-react";

interface BookmarkButtonProps {
  isBookmarked: boolean;
  onToggle: () => void;
}

export function BookmarkButton({
  isBookmarked,
  onToggle,
}: BookmarkButtonProps) {
  return (
    <button
      className={`btn btn-circle btn-ghost btn-xs ${isBookmarked ? "text-warning" : "opacity-50"}`}
      onClick={onToggle}
      aria-label={isBookmarked ? "Remove bookmark" : "Add bookmark"}
    >
      <Star className="w-4 h-4" fill={isBookmarked ? "currentColor" : "none"} />
    </button>
  );
}
