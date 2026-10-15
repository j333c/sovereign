"use client";

import { useState, useEffect, useRef } from "react";
import {
  SovereignApiClient,
  TelemetryUpdate,
  ServiceHealthChangedEvent,
  ServiceErrorEvent,
} from "./api";

export interface UseTelemetryOptions {
  client: SovereignApiClient;
  initialTelemetry?: TelemetryUpdate | null;
  eventSourceFactory?: (url: string) => EventSource;
  onServiceHealthChanged?: (event: ServiceHealthChangedEvent) => void;
  onServiceStatusChanged?: (event: ServiceHealthChangedEvent | any) => void;
  onServiceError?: (event: ServiceErrorEvent) => void;
}

export function useTelemetry({
  client,
  initialTelemetry = null,
  eventSourceFactory,
  onServiceHealthChanged,
  onServiceStatusChanged,
  onServiceError,
}: UseTelemetryOptions) {
  const [telemetry, setTelemetry] = useState<TelemetryUpdate | null>(initialTelemetry);
  const [isConnected, setIsConnected] = useState(false);
  const [hasError, setHasError] = useState(false);

  const onHealthChangedRef = useRef(onServiceHealthChanged);
  onHealthChangedRef.current = onServiceHealthChanged;

  const onStatusChangedRef = useRef(onServiceStatusChanged);
  onStatusChangedRef.current = onServiceStatusChanged;

  const onErrorRef = useRef(onServiceError);
  onErrorRef.current = onServiceError;

  useEffect(() => {
    let isMounted = true;
    let es: EventSource | null = null;

    try {
      const url = client.getEventsUrl();

      if (eventSourceFactory) {
        es = eventSourceFactory(url);
      } else if (typeof window !== "undefined" && typeof window.EventSource !== "undefined") {
        es = new window.EventSource(url, { withCredentials: true });
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

        es.addEventListener("service.health_changed", (event: MessageEvent) => {
          if (!isMounted) return;
          try {
            const data = JSON.parse(event.data) as ServiceHealthChangedEvent;
            onHealthChangedRef.current?.(data);
          } catch {
            // Ignore malformed payloads
          }
        });

        es.addEventListener("service.status_changed", (event: MessageEvent) => {
          if (!isMounted) return;
          try {
            const data = JSON.parse(event.data) as ServiceHealthChangedEvent;
            onStatusChangedRef.current?.(data);
          } catch {
            // Ignore malformed payloads
          }
        });

        es.addEventListener("service.error", (event: MessageEvent) => {
          if (!isMounted) return;
          try {
            const data = JSON.parse(event.data) as ServiceErrorEvent;
            onErrorRef.current?.(data);
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
