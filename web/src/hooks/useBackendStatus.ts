import { useState, useEffect } from 'react';
import { api } from '../api/client';

export interface BackendStatus {
  isOnline: boolean;
  isReady: boolean;
  lastChecked: string | null;
  error: string | null;
  checkStatus: () => Promise<void>;
}

export function useBackendStatus(pollIntervalMs = 10000): BackendStatus {
  const [isOnline, setIsOnline] = useState(false);
  const [isReady, setIsReady] = useState(false);
  const [lastChecked, setLastChecked] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const checkStatus = async () => {
    try {
      await api.getHealthz();
      setIsOnline(true);
      
      try {
        await api.getReadyz();
        setIsReady(true);
      } catch {
        setIsReady(false);
      }
      
      setError(null);
    } catch (err: any) {
      setIsOnline(false);
      setIsReady(false);
      setError(err.message || 'Backend unavailable');
    } finally {
      setLastChecked(new Date().toLocaleTimeString());
    }
  };

  useEffect(() => {
    checkStatus();
    const interval = setInterval(checkStatus, pollIntervalMs);
    return () => clearInterval(interval);
  }, [pollIntervalMs]);

  return {
    isOnline,
    isReady,
    lastChecked,
    error,
    checkStatus,
  };
}
