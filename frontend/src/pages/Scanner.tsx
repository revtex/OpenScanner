import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useGetSetupStatusQuery } from '@/app/api';
import { useAppDispatch } from '@/app/store';
import { setSetupStatus } from '@/app/slices/authSlice';
import { useScanner } from '@/hooks/useScanner';
import { LEDPanel } from '@/components/scanner/LEDPanel';
import { DisplayPanel } from '@/components/scanner/DisplayPanel';
import { ControlToolbar } from '@/components/scanner/ControlToolbar';

export default function Scanner() {
  const navigate = useNavigate();
  const dispatch = useAppDispatch();
  const { data: setupStatus } = useGetSetupStatusQuery();

  const scanner = useScanner();

  useEffect(() => {
    if (setupStatus) {
      dispatch(setSetupStatus(setupStatus));
      if (setupStatus.needsSetup) {
        navigate('/setup', { replace: true });
      }
    }
  }, [setupStatus, navigate, dispatch]);

  return (
    <div className="max-w-2xl mx-auto p-6">
      <LEDPanel />
      <DisplayPanel
        currentCall={scanner.currentCall}
        history={scanner.history}
        listenerCount={scanner.listenerCount}
        queueCount={scanner.callQueue.length}
        avoidList={scanner.avoidList}
      />
      <ControlToolbar
        isPlaying={scanner.isPlaying}
        isPaused={scanner.isPaused}
        isLive={scanner.isLive}
        volume={scanner.volume}
        heldSystem={scanner.heldSystem}
        heldTG={scanner.heldTG}
        avoidList={scanner.avoidList}
        currentCallTgId={scanner.currentCall?.talkgroupId}
        currentCallSystemId={scanner.currentCall?.system}
        onTogglePause={scanner.togglePause}
        onToggleLive={scanner.toggleLive}
        onSkip={scanner.skip}
        onReplay={scanner.replay}
        onDownload={scanner.download}
        onSetVolume={scanner.setVolume}
        onHoldSystem={scanner.holdSystem}
        onHoldTG={scanner.holdTG}
        onAddAvoid={scanner.addAvoid}
        onClearAvoids={scanner.clearAvoids}
      />
    </div>
  );
}
