import {
  createContext,
  useContext,
  useRef,
  useState,
  useCallback,
} from "react";
import { useNavigate } from "react-router-dom";

interface NavigationGuardContextValue {
  /** Register a guard — return true to block navigation. */
  setGuard: (guard: (() => boolean) | null) => void;
  /** Call before navigating — returns true if navigation is allowed. */
  requestNavigation: (path: string) => boolean;
}

const NavigationGuardContext = createContext<NavigationGuardContextValue>({
  setGuard: () => {},
  requestNavigation: () => true,
});

// eslint-disable-next-line react-refresh/only-export-components
export function useNavigationGuard() {
  return useContext(NavigationGuardContext);
}

export function NavigationGuardProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const guardRef = useRef<(() => boolean) | null>(null);
  const [pendingPath, setPendingPath] = useState<string | null>(null);
  const navigate = useNavigate();

  const setGuard = useCallback((guard: (() => boolean) | null) => {
    guardRef.current = guard;
  }, []);

  const requestNavigation = useCallback((path: string) => {
    if (guardRef.current?.()) {
      setPendingPath(path);
      return false;
    }
    return true;
  }, []);

  const handleStay = useCallback(() => {
    setPendingPath(null);
  }, []);

  const handleLeave = useCallback(() => {
    const path = pendingPath;
    setPendingPath(null);
    guardRef.current = null;
    if (path) navigate(path);
  }, [pendingPath, navigate]);

  return (
    <NavigationGuardContext.Provider value={{ setGuard, requestNavigation }}>
      {children}

      {pendingPath && (
        <dialog className="modal modal-open z-100">
          <div className="modal-box">
            <h3 className="font-bold text-lg">Unsaved Changes</h3>
            <p className="py-4">
              You have unsaved changes. Are you sure you want to leave?
            </p>
            <div className="modal-action">
              <button className="btn" onClick={handleStay}>
                Stay
              </button>
              <button className="btn btn-warning" onClick={handleLeave}>
                Leave
              </button>
            </div>
          </div>
          <div className="modal-backdrop" onClick={handleStay} />
        </dialog>
      )}
    </NavigationGuardContext.Provider>
  );
}
