import { configureStore } from "@reduxjs/toolkit";
import { useDispatch, useSelector } from "react-redux";
import { api } from "@/app/api";
import { scannerSlice } from "@/app/slices/scanner/scannerSlice";
import { authSlice } from "@/features/auth";
import { callsSlice } from "@/app/slices/scanner/callsSlice";
import { audioListenerMiddleware } from "@/app/audioListenerMiddleware";

export const store = configureStore({
  reducer: {
    [api.reducerPath]: api.reducer,
    scanner: scannerSlice.reducer,
    auth: authSlice.reducer,
    calls: callsSlice.reducer,
  },
  middleware: (getDefaultMiddleware) =>
    getDefaultMiddleware()
      .prepend(audioListenerMiddleware.middleware)
      .concat(api.middleware),
});

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;

export const useAppDispatch = useDispatch.withTypes<AppDispatch>();
export const useAppSelector = useSelector.withTypes<RootState>();
