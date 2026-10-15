"use client";

import { useState, useEffect } from "react";
import { SovereignApiClient, TelemetryUpdate } from "./api";

export interface UseTelemetryOptions {
  client: SovereignApiClient;
  initialTelemetry?: TelemetryUpdate | null;
  eventSourceFactory?: (url: string) => EventSource;
}

export function useTelemetry({
  client,
  initialTelemetry = null,
  eventSourceFactory,
}: UseTelemetryOptions) {
  const [telemetry, setTelemetry] = useState<TelemetryUpdate | null>(initialTelemetry);
  const [isConnected, setIsConnected] = useState(false);
  const [hasError, setHasError] = useState(false);

  useEffect(() => {
    let isMounted = true;
    let es: EventSource | null = null;

    try {
      const url = client.getEventsUrl();

      if (eventSourceFactory) {
        es = eventSourceFactory(url);
      } else if (typeof window !== "undefined" && typeof window.EventSource !== "undefined") {
        es = new window.EventSource(url);
      }

      if (es) {
        es.onopen = () => {
          if (isMounted) {
            setIsConnected(true);
            setHasError(false);
          }
        };

        es.addEventListener("telemetry.update", (event: MessageEvent) => {
          if (!isMounted) return;
          try {
            const data = JSON.parse(event.data) as TelemetryUpdate;
            setTelemetry(data);
            setIsConnected(true);
            setHasError(false);
          } catch {
            // Ignore malformed payloads
          }
        });

        es.onerror = () => {
          if (isMounted) {
            setIsConnected(false);
            setHasError(true);
          }
        };
      }
    } catch {
      if (isMounted) {
        setHasError(true);
      }
    }

    return () => {
      isMounted = false;
      if (es) {
        es.close();
      }
    };
  }, [client, eventSourceFactory]);

  return {
    telemetry,
    isConnected,
    hasError,
    setTelemetry,
  };
}
