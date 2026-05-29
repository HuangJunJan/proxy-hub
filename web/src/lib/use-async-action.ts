import { useCallback, useState } from "react";

export function useAsyncAction<Args extends unknown[]>(action: (...args: Args) => Promise<void>) {
  const [loading, setLoading] = useState(false);

  const run = useCallback(
    async (...args: Args) => {
      if (loading) {
        return;
      }
      setLoading(true);
      try {
        await action(...args);
      } finally {
        setLoading(false);
      }
    },
    [action, loading],
  );

  return { loading, run };
}
