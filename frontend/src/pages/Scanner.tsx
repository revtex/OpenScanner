import { useCallback, useEffect, useState, useMemo } from "react";
import { useNavigate } from "react-router-dom";
import { useGetSetupStatusQuery } from "@/app/api";
import { useAppDispatch, useAppSelector } from "@/app/store";
import { setSetupStatus, selectToken } from "@/app/slices/authSlice";
import { expireAvoids, setPaused } from "@/app/slices/scannerSlice";
import { useScanner } from "@/hooks/useScanner";
import { useKeyboardShortcuts } from "@/hooks/useKeyboardShortcuts";
import { LEDPanel } from "@/components/scanner/LEDPanel";
import { DisplayPanel } from "@/components/scanner/DisplayPanel";
import { ControlToolbar } from "@/components/scanner/ControlToolbar";
import SelectTGPanel from "@/components/scanner/SelectTGPanel";
import SearchPanel from "@/components/scanner/SearchPanel";
import BookmarksPanel from "@/components/scanner/BookmarksPanel";

export default function Scanner() {
  const navigate = useNavigate();
  const dispatch = useAppDispatch();
  const { data: setupStatus } = useGetSetupStatusQuery();
  const token = useAppSelector(selectToken);

  const scanner = useScanner();

  const [selectTGOpen, setSelectTGOpen] = useState(false);
  const [searchOpen, setSearchOpen] = useState(false);
  const [bookmarksOpen, setBookmarksOpen] = useState(false);

  const handleToggleSelectTG = useCallback(() => {
    setSelectTGOpen((prev) => !prev);
    setSearchOpen(false);
  }, []);

  const handleToggleSearch = useCallback(() => {
    setSearchOpen((prev) => !prev);
    setSelectTGOpen(false);
  }, []);

  const handleCloseAllPanels = useCallback(() => {
    setSelectTGOpen(false);
    setSearchOpen(false);
  }, []);

  const kbCallbacks = useMemo(
    () => ({
      onSkip: scanner.skip,
      onReplay: scanner.replay,
      onSetVolume: scanner.setVolume,
      onToggleSelectTG: handleToggleSelectTG,
      onToggleSearch: handleToggleSearch,
      onToggleShortcutsModal: () => {
        /* handled inside ControlToolbar */
      },
      onCloseAllPanels: handleCloseAllPanels,
      volume: scanner.volume,
    }),
    [
      scanner.skip,
      scanner.replay,
      scanner.setVolume,
      scanner.volume,
      handleToggleSelectTG,
      handleToggleSearch,
      handleCloseAllPanels,
    ],
  );

  useKeyboardShortcuts(kbCallbacks);

  useEffect(() => {
    if (setupStatus) {
      dispatch(setSetupStatus(setupStatus));
      if (setupStatus.needsSetup) {
        navigate("/setup", { replace: true });
        return;
      }
      // If not public access and not authenticated, redirect to login
      if (!setupStatus.publicAccess && !token) {
        navigate("/login", { replace: true });
      }
    }
  }, [setupStatus, navigate, dispatch, token]);

  // Expire timed avoid entries every 10 seconds
  useEffect(() => {
    const id = setInterval(() => dispatch(expireAvoids()), 10_000);
    return () => clearInterval(id);
  }, [dispatch]);

  // Pause applies to the current playback moment only; clear stale paused UI
  // state when leaving the scanner route.
  useEffect(
    () => () => {
      dispatch(setPaused(false));
    },
    [dispatch],
  );

  return (
    <div className="max-w-2xl mx-auto p-6">
      <LEDPanel />
      <DisplayPanel
        currentCall={scanner.currentCall}
        history={scanner.history}
        listenerCount={scanner.listenerCount}
        queueCount={scanner.pendingCount}
        avoidList={scanner.avoidList}
        time12hFormat={scanner.config?.time12hFormat ?? false}
        showListenersCount={scanner.config?.showListenersCount ?? false}
        shareableLinks={scanner.config?.shareableLinks ?? false}
        isAuthenticated={!!token}
      />
      <ControlToolbar
        isPlaying={scanner.isPlaying}
        isPaused={scanner.isPaused}
        isLive={scanner.isLive}
        volume={scanner.volume}
        heldSystem={scanner.heldSystem}
        heldTG={scanner.heldTG}
        currentCallTgId={scanner.currentCall?.talkgroup}
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
        onToggleSelectTG={handleToggleSelectTG}
        onToggleSearch={handleToggleSearch}
        onToggleBookmarks={
          token ? () => setBookmarksOpen((prev) => !prev) : undefined
        }
        keypadBeeps={scanner.config?.keypadBeeps}
      />
      <SelectTGPanel
        isOpen={selectTGOpen}
        onClose={() => setSelectTGOpen(false)}
      />
      <SearchPanel isOpen={searchOpen} onClose={() => setSearchOpen(false)} />
      <BookmarksPanel
        isOpen={bookmarksOpen}
        onClose={() => setBookmarksOpen(false)}
      />
    </div>
  );
}
