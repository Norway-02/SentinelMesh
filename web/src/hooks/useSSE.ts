import { useEffect, useState, useRef, useCallback } from 'react';
import { SSEEvent } from '../api/types';

const API_BASE = import.meta.env.VITE_API_BASE_URL || 'http://127.0.0.1:8787';

export interface UseSSEReturn {
  events: SSEEvent[];
  isConnected: boolean;
  lastEventTime: string | null;
  error: string | null;
  clearEvents: () => void;
}

export function useSSE(maxEvents = 100): UseSSEReturn {
  const [events, setEvents] = useState<SSEEvent[]>([]);
  const [isConnected, setIsConnected] = useState(false);
  const [lastEventTime, setLastEventTime] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const seenIdsRef = useRef<Set<string>>(new Set());
  const retryCountRef = useRef(0);
  const eventSourceRef = useRef<EventSource | null>(null);

  const clearEvents = useCallback(() => {
    setEvents([]);
    seenIdsRef.current.clear();
  }, []);

  useEffect(() => {
    let isMounted = true;
    let timerId: any = null;

    function connect() {
      if (!isMounted) return;

      const url = `${API_BASE}/v1/events/stream`;
      const es = new EventSource(url);
      eventSourceRef.current = es;

      es.onopen = () => {
        if (!isMounted) return;
        setIsConnected(true);
        setError(null);
        retryCountRef.current = 0;
      };

      es.onmessage = (event) => {
        if (!isMounted) return;
        try {
          const parsed: SSEEvent = JSON.parse(event.data);
          
          // Deduplicate by ID
          if (parsed.id && seenIdsRef.current.has(parsed.id)) {
            return;
          }
          if (parsed.id) {
            seenIdsRef.current.add(parsed.id);
            if (seenIdsRef.current.size > 500) {
              const items = Array.from(seenIdsRef.current);
              seenIdsRef.current = new Set(items.slice(items.length - 250));
            }
          }

          const now = new Date().toLocaleTimeString();
          setLastEventTime(now);

          setEvents((prev) => [parsed, ...prev.slice(0, maxEvents - 1)]);
        } catch {
          // ignore heartbeat / unparseable
        }
      };

      es.onerror = () => {
        if (!isMounted) return;
        setIsConnected(false);
        es.close();

        // Exponential backoff reconnect
        retryCountRef.current += 1;
        const delay = Math.min(1000 * Math.pow(2, retryCountRef.current), 15000);
        setError(`Disconnected. Reconnecting in ${(delay / 1000).toFixed(0)}s...`);

        timerId = setTimeout(() => {
          connect();
        }, delay);
      };
    }

    connect();

    return () => {
      isMounted = false;
      if (timerId) clearTimeout(timerId);
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
      }
    };
  }, [maxEvents]);

  return {
    events,
    isConnected,
    lastEventTime,
    error,
    clearEvents,
  };
}
