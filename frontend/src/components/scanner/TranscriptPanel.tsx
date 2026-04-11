import type { Call } from "@/types";

interface TranscriptPanelProps {
  call: Call | null;
}

export function TranscriptPanel({ call }: TranscriptPanelProps) {
  if (!call?.transcript) return null;

  return (
    <details open className="border-t border-base-content/20 pt-2 mt-2">
      <summary className="text-xs opacity-60 cursor-pointer select-none mb-1">
        Transcript
      </summary>
      <p className="text-xs italic text-neutral-content/80 whitespace-pre-wrap px-1">
        {call.transcript}
      </p>
    </details>
  );
}
