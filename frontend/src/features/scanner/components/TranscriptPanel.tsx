import type { Call, TranscriptionSegment } from "../types";

const SPEAKER_COLORS = [
  "bg-primary/10",
  "bg-secondary/10",
  "bg-accent/10",
  "bg-info/10",
  "bg-success/10",
  "bg-warning/10",
];

function formatTime(seconds: number): string {
  const m = Math.floor(seconds / 60);
  const s = Math.floor(seconds % 60);
  return `${m}:${s.toString().padStart(2, "0")}`;
}

function speakerLabel(speaker: string): string {
  const match = speaker.match(/SPEAKER_(\d+)/);
  if (match) {
    return `Speaker ${parseInt(match[1], 10) + 1}`;
  }
  return speaker;
}

interface TranscriptPanelProps {
  call: Call | null;
}

export function TranscriptPanel({ call }: TranscriptPanelProps) {
  if (!call?.transcript) return null;

  const segments = call.transcriptSegments;
  const hasDiarization =
    segments && segments.length > 0 && segments.some((s) => s.speaker);

  return (
    <details open className="border-t border-base-content/20 pt-2 mt-2">
      <summary className="text-xs opacity-60 cursor-pointer select-none mb-1">
        Transcript
      </summary>
      {hasDiarization ? (
        <DiarizedTranscript segments={segments} />
      ) : (
        <p className="text-xs italic opacity-70 whitespace-pre-wrap px-1">
          {call.transcript}
        </p>
      )}
    </details>
  );
}

function DiarizedTranscript({
  segments,
}: {
  segments: TranscriptionSegment[];
}) {
  const speakerMap = new Map<string, number>();
  let nextIndex = 0;

  for (const seg of segments) {
    if (seg.speaker && !speakerMap.has(seg.speaker)) {
      speakerMap.set(seg.speaker, nextIndex++);
    }
  }

  return (
    <div className="space-y-0.5 px-1">
      {segments.map((seg, i) => {
        const colorIndex = seg.speaker
          ? (speakerMap.get(seg.speaker) ?? 0) % SPEAKER_COLORS.length
          : 0;
        const color = SPEAKER_COLORS[colorIndex];
        const title = `${formatTime(seg.start)} – ${formatTime(seg.end)}`;

        return (
          <p
            key={i}
            className={`text-xs px-1.5 py-0.5 rounded ${color}`}
            title={title}
          >
            <span className="font-semibold opacity-70">
              [{speakerLabel(seg.speaker ?? "Unknown")}]
            </span>{" "}
            {seg.text}
          </p>
        );
      })}
    </div>
  );
}
