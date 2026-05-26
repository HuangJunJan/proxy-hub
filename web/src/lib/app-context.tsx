import { createContext, useContext } from "react";
import type { Language, ThemeMode } from "./types";

export interface AppContextValue {
  language: Language;
  setLanguage: (value: Language) => void;
  theme: ThemeMode;
  setTheme: (value: ThemeMode) => void;
  username: string;
  setAuthenticated: (username: string) => void;
  setLoggedOut: () => void;
  t: (key: string) => string;
}

const AppContext = createContext<AppContextValue | null>(null);

export const AppContextProvider = AppContext.Provider;

export function useAppContext() {
  const value = useContext(AppContext);
  if (!value) {
    throw new Error("useAppContext must be used within AppContextProvider");
  }
  return value;
}
