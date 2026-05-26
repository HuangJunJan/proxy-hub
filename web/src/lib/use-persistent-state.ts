import { useEffect, useState } from "react";

export function usePersistentState<T extends string>(key: string, fallback: T) {
  const [value, setValue] = useState<T>(() => (localStorage.getItem(key) as T | null) ?? fallback);
  useEffect(() => {
    localStorage.setItem(key, value);
  }, [key, value]);
  return [value, setValue] as const;
}
