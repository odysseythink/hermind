import { useCallback, useEffect, useState } from 'react';
import { apiFetch } from '../api/client';
import { SessionsListResponseSchema, type SessionSummary } from '../api/schemas';

export function useSessionList() {
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const ctrl = new AbortController();
    apiFetch('/api/sessions?limit=50', {
      schema: SessionsListResponseSchema,
      signal: ctrl.signal,
    })
      .then((r) => setSessions(r.sessions))
      .catch((e) => {
        if (ctrl.signal.aborted) return;
        setError(e instanceof Error ? e.message : 'load failed');
      });
    return () => ctrl.abort();
  }, []);

  const newSession = useCallback(() => {
    const id = crypto.randomUUID();
    setSessions((prev) => [{ id, title: 'New conversation', updated_at: Date.now() }, ...prev]);
    return id;
  }, []);

  return { sessions, error, newSession };
}
