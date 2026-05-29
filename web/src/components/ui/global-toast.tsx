import { useEffect, useState } from "react";
import { subscribeErrorMessage } from "../../lib/error-events";
import { Toast } from "./toast";

const TOAST_TIMEOUT_MS = 4000;

export function GlobalToast() {
  const [message, setMessage] = useState("");

  useEffect(() => {
    return subscribeErrorMessage((nextMessage) => {
      setMessage(nextMessage);
    });
  }, []);

  useEffect(() => {
    if (!message) {
      return;
    }
    const timeout = window.setTimeout(() => {
      setMessage("");
    }, TOAST_TIMEOUT_MS);
    return () => {
      window.clearTimeout(timeout);
    };
  }, [message]);

  if (!message) {
    return null;
  }

  return (
    <div className="global-toast-region" role="status" aria-live="polite">
      <Toast variant="destructive">{message}</Toast>
    </div>
  );
}
