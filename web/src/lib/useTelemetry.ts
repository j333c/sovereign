"use client";

import { useState, useEffect, useRef } from "react";
import {
  SovereignApiClient,
  TelemetryUpdate,
  ServiceHealthChangedEvent,
  ServiceErrorEvent,
  DMSMailboxStatusEvent,
  DMSDocumentIngestedEvent,
  DMSDeadlineDetectedEvent,
} from "./api";

export interface UseTelemetryOptions {
  client: SovereignApiClient;
  initialTelemetry?: TelemetryUpdate | null;
  eventSourceFactory?: (url: string) => EventSource;
  onServiceHealthChanged?: (event: ServiceHealthChangedEvent) => void;
  onServiceStatusChanged?: (event: ServiceHealthChangedEvent | any) => void;
  onServiceError?: (event: ServiceErrorEvent) => void;
  onDMSMailboxStatus?: (event: DMSMailboxStatusEvent) => void;
  onDMSDocumentIngested?: (event: DMSDocumentIngestedEvent) => void;
  onDMSDeadlineDetected?: (event: DMSDeadlineDetectedEvent) => void;
}

export interface TelemetryListenerCallbacks {
  onTelemetryUpdate?: (data: TelemetryUpdate) => void;
  onServiceHealthChanged?: (event: ServiceHealthChangedEvent) => void;
  onServiceStatusChanged?: (event: ServiceHealthChangedEvent) => void;
  onServiceError?: (event: ServiceErrorEvent) => void;
  onDMSMailboxStatus?: (event: DMSMailboxStatusEvent) => void;
  onDMSDocumentIngested?: (event: DMSDocumentIngestedEvent) => void;
  onDMSDeadlineDetected?: (event: DMSDeadlineDetectedEvent) => void;
}

export function attachTelemetryListeners(
  es: { addEventListener: (event: string, cb: (event: any) => void) => void },
  callbacks: TelemetryListenerCallbacks
) {
  es.addEventListener("telemetry.update", (event: { data: string }) => {
    try {
      const data = JSON.parse(event.data) as TelemetryUpdate;
      callbacks.onTelemetryUpdate?.(data);
    } catch {
      // Ignore malformed payloads
    }
  });

  es.addEventListener("service.health_changed", (event: { data: string }) => {
    try {
      const data = JSON.parse(event.data) as ServiceHealthChangedEvent;
      callbacks.onServiceHealthChanged?.(data);
    } catch {
      // Ignore malformed payloads
    }
  });

  es.addEventListener("service.status_changed", (event: { data: string }) => {
    try {
      const data = JSON.parse(event.data) as ServiceHealthChangedEvent;
      callbacks.onServiceStatusChanged?.(data);
    } catch {
      // Ignore malformed payloads
    }
  });

  es.addEventListener("service.error", (event: { data: string }) => {
    try {
      const data = JSON.parse(event.data) as ServiceErrorEvent;
      callbacks.onServiceError?.(data);
    } catch {
      // Ignore malformed payloads
    }
  });

  es.addEventListener("dms.mailbox_status", (event: { data: string }) => {
    try {
      const data = JSON.parse(event.data) as DMSMailboxStatusEvent;
      callbacks.onDMSMailboxStatus?.(data);
    } catch {
      // Ignore malformed payloads
    }
  });

  es.addEventListener("dms.document_ingested", (event: { data: string }) => {
    try {
      const data = JSON.parse(event.data) as DMSDocumentIngestedEvent;
      callbacks.onDMSDocumentIngested?.(data);
    } catch {
      // Ignore malformed payloads
    }
  });

  es.addEventListener("dms.deadline_detected", (event: { data: string }) => {
    try {
      const data = JSON.parse(event.data) as DMSDeadlineDetectedEvent;
      callbacks.onDMSDeadlineDetected?.(data);
    } catch {
      // Ignore malformed payloads
    }
  });
}

export function useTelemetry({
  client,
  initialTelemetry = null,
  eventSourceFactory,
  onServiceHealthChanged,
  onServiceStatusChanged,
  onServiceError,
  onDMSMailboxStatus,
  onDMSDocumentIngested,
  onDMSDeadlineDetected,
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

  const onDMSMailboxStatusRef = useRef(onDMSMailboxStatus);
  onDMSMailboxStatusRef.current = onDMSMailboxStatus;

  const onDMSDocumentIngestedRef = useRef(onDMSDocumentIngested);
  onDMSDocumentIngestedRef.current = onDMSDocumentIngested;

  const onDMSDeadlineDetectedRef = useRef(onDMSDeadlineDetected);
  onDMSDeadlineDetectedRef.current = onDMSDeadlineDetected;

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

        attachTelemetryListeners(es, {
          onTelemetryUpdate: (data) => {
            if (!isMounted) return;
            setTelemetry(data);
            setIsConnected(true);
            setHasError(false);
          },
          onServiceHealthChanged: (data) => {
            if (!isMounted) return;
            onHealthChangedRef.current?.(data);
          },
          onServiceStatusChanged: (data) => {
            if (!isMounted) return;
            onStatusChangedRef.current?.(data);
          },
          onServiceError: (data) => {
            if (!isMounted) return;
            onErrorRef.current?.(data);
          },
          onDMSMailboxStatus: (data) => {
            if (!isMounted) return;
            onDMSMailboxStatusRef.current?.(data);
          },
          onDMSDocumentIngested: (data) => {
            if (!isMounted) return;
            onDMSDocumentIngestedRef.current?.(data);
          },
          onDMSDeadlineDetected: (data) => {
            if (!isMounted) return;
            onDMSDeadlineDetectedRef.current?.(data);
          },
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
