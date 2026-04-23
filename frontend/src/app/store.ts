import { configureStore } from "@reduxjs/toolkit";
import { useDispatch, useSelector } from "react-redux";
import { api } from "@/app/api";
import { scannerSlice } from "@/app/slices/scannerSlice";
import { authSlice } from "@/app/slices/authSlice";
import { callsSlice } from "@/app/slices/callsSlice";

export const store = configureStore({
  reducer: {
    [api.reducerPath]: api.reducer,
    scanner: scannerSlice.reducer,
    auth: authSlice.reducer,
    calls: callsSlice.reducer,
  },
  middleware: (getDefaultMiddleware) =>
    getDefaultMiddleware().concat(api.middleware),
});

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;

export const useAppDispatch = useDispatch.withTypes<AppDispatch>();
export const useAppSelector = useSelector.withTypes<RootState>();
