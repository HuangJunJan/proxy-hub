type ErrorListener = (message: string) => void;

const listeners = new Set<ErrorListener>();

export function subscribeErrorMessage(listener: ErrorListener) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function emitErrorMessage(message: string) {
  const normalized = message.trim();
  if (!normalized) {
    return;
  }
  listeners.forEach((listener) => listener(normalized));
}
