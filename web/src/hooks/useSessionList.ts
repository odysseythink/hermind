import { useCallback, useEffect, useRef, useState } from 'react';
import { apiFetch } from '../api/client';
import { SessionsListResponseSchema, type SessionSummary } from '../api/schemas';

const POLL_INTERVAL_MS = 10_000;

export function useSessionList() {
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [error, setError] = useState<string | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  const refetch = useCallback(() => {
    abortRef.current?.abort();
    const ctrl = new AbortController();
    abortRef.current = ctrl;
    apiFetch('/api/sessions?limit=50', {
      schema: SessionsListResponseSchema,
      signal: ctrl.signal,
    })
      .then((r) => {
        // Merge: server-authoritative for known ids; preserve local-only rows
        // (e.g. just-inserted via SSE before the server indexes them).
        setSessions((prev) => {
          const byId = new Map<string, SessionSummary>();
          for (const s of r.sessions) byId.set(s.id, s);
          for (const s of prev) if (!byId.has(s.id)) byId.set(s.id, s);
          return [...byId.values()].sort(
            (a, b) => (b.started_at ?? 0) - (a.started_at ?? 0),
          );
        });
      })
      .catch((e) => {
        if (ctrl.signal.aborted) return;
        setError(e instanceof Error ? e.message : 'load failed');
      });
  }, []);

  // Initial load + polling + focus-refetch
  useEffect(() => {
    refetch();
    const timer = setInterval(refetch, POLL_INTERVAL_MS);
    const onFocus = () => refetch();
    window.addEventListener('focus', onFocus);
    return () => {
      clearInterval(timer);
      window.removeEventListener('focus', onFocus);
      abortRef.current?.abort();
    };
  }, [refetch]);

  const newSession = useCallback(() => crypto.randomUUID(), []);

  const insertSession = useCallback((session: SessionSummary) => {
    setSessions((prev) =>
      prev.some((s) => s.id === session.id) ? prev : [session, ...prev],
    );
  }, []);

  const patchSession = useCallback(
    (id: string, patch: Partial<Pick<SessionSummary, 'title' | 'model' | 'system_prompt'>>) => {
      setSessions((prev) =>
        prev.map((s) => (s.id === id ? { ...s, ...patch } : s)),
      );
    },
    [],
  );

  return { sessions, error, newSession, insertSession, patchSession, refetch };
}
