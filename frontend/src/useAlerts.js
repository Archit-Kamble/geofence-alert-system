import { useEffect, useRef, useState } from "react";
import { WS_URL } from "./api.js";

// useAlerts keeps one WebSocket open to the backend and calls onAlert for
// every alert that arrives. If the connection drops, it reconnects by itself,
// waiting a little longer after each failed try (1s, 2s, 4s ... up to 15s).
//
// It returns the connection status: "connecting", "open" or "closed".
export function useAlerts(onAlert) {
  const [status, setStatus] = useState("connecting");

  // The newest onAlert is kept in a ref. The effect below can then run only
  // once, and still call the latest function without reconnecting on every render.
  const onAlertRef = useRef(onAlert);
  useEffect(() => {
    onAlertRef.current = onAlert;
  }, [onAlert]);

  useEffect(() => {
    let socket = null;
    let retryTimer = null;
    let retryDelay = 1000;
    let stopped = false; // true after the component is removed

    function connect() {
      setStatus("connecting");
      socket = new WebSocket(WS_URL);

      socket.onopen = () => {
        retryDelay = 1000;
        setStatus("open");
      };

      socket.onmessage = (message) => {
        try {
          onAlertRef.current(JSON.parse(message.data));
        } catch {
          // ignore a message that is not valid JSON
        }
      };

      socket.onclose = () => {
        if (stopped) return;
        setStatus("closed");
        retryTimer = setTimeout(connect, retryDelay);
        retryDelay = Math.min(retryDelay * 2, 15000);
      };

      // An error is always followed by onclose, which handles the retry.
      socket.onerror = () => socket.close();
    }

    connect();

    // Cleanup: runs when the component is removed from the page.
    return () => {
      stopped = true;
      clearTimeout(retryTimer);
      if (socket) socket.close();
    };
  }, []);

  return status;
}
