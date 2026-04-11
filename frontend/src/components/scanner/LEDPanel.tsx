import { Sun, Moon } from 'lucide-react';
import { useTheme } from '@/hooks/useTheme';
import { useAppSelector } from '@/app/store';

export function LEDPanel() {
  const { isDark, toggle } = useTheme();
  const config = useAppSelector((s) => s.scanner.config);
  const isLive = useAppSelector((s) => s.scanner.isLive);
  const isPaused = useAppSelector((s) => s.scanner.isPaused);
  const currentCall = useAppSelector((s) => s.scanner.currentCall);
  const isPlaying = !!currentCall;

  const branding = config?.branding ?? 'OPENSCANNER';

  // LED color logic
  let ledColor = '#555';
  if (currentCall?.talkgroupLedColor) {
    ledColor = currentCall.talkgroupLedColor;
  } else if (isLive && isPlaying) {
    ledColor = '#00e676';
  } else if (isPaused || !isLive) {
    ledColor = '#ff9100';
  }

  const shouldBlink = isPaused;

  return (
    <div className="flex items-center justify-between h-6 mb-6">
      <span
        className="text-sm font-bold tracking-widest uppercase opacity-70"
        style={{ letterSpacing: '2px' }}
      >
        {branding}
      </span>
      <div className="flex items-center gap-3">
        <button
          className="btn btn-ghost btn-xs btn-circle"
          onClick={toggle}
          aria-label="Toggle theme"
        >
          {isDark ? <Sun className="w-4 h-4" /> : <Moon className="w-4 h-4" />}
        </button>
        <div
          className={`rounded-sm ${shouldBlink ? 'animate-pulse' : ''}`}
          style={{
            width: '24px',
            height: '12px',
            backgroundColor: ledColor,
            boxShadow: `0 0 8px ${ledColor}, 0 0 16px ${ledColor}`,
            animationTimingFunction: shouldBlink ? 'step-end' : undefined,
          }}
        />
      </div>
    </div>
  );
}
