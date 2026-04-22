import { useEffect, useRef } from 'react';
import type { ChatAction } from '../state/chat';
import {
  SessionSummarySchema,
  SessionUpdatedPayloadSchema,
  type SessionSummary,
  type SessionUpdatedPayload,
} from '../api/schemas';

type Dispatch = (a: ChatAction) => void;

export function useChatStream(
  sessionId: string | null,
  dispatch: Dispatch,
  onSessionCreated?: (session: SessionSummary) => void,
  onSessionUpdated?: (id: string, patch: SessionUpdatedPayload) => void,
) {
  const tokenBufRef = useRef('');
  const rafPendingRef = useRef(false);
  const onSessionCreatedRef = useRef(onSessionCreated);
  const onSessionUpdatedRef = useRef(onSessionUpdated);
  onSessionCreatedRef.current = onSessionCreated;
  onSessionUpdatedRef.current = onSessionUpdated;

  useEffect(() => {
    if (!sessionId) return;
    const token = new URLSearchParams(window.location.search).get('t') ?? '';
    const es = new EventSource(
      `/api/sessions/${encodeURIComponent(sessionId)}/stream/sse?t=${encodeURIComponent(token)}`,
    );

    function flushTokens() {
      rafPendingRef.current = false;
      if (tokenBufRef.current) {
        dispatch({ type: 'chat/stream/token', delta: tokenBufRef.current });
        tokenBufRef.current = '';
      }
    }

    es.onmessage = (ev) => {
      let parsed: { type?: string; session_id?: string; data?: Record<string, unknown> };
      try {
        parsed = JSON.parse(ev.data);
      } catch {
        return;
      }
      if (parsed.session_id && parsed.session_id !== sessionId) return;
      switch (parsed.type) {
        case 'session_created': {
          const payload = SessionSummarySchema.parse(parsed.data);
          dispatch({ type: 'chat/session/created', session: payload });
          onSessionCreatedRef.current?.(payload);
          break;
        }
        case 'session_updated': {
          const payload = SessionUpdatedPayloadSchema.parse(parsed.data);
          onSessionUpdatedRef.current?.(parsed.session_id!, payload);
          break;
        }
        case 'token': {
          const d = parsed.data as { text?: string } | undefined;
          if (typeof d?.text === 'string') {
            tokenBufRef.current += d.text;
            if (!rafPendingRef.current) {
              rafPendingRef.current = true;
              requestAnimationFrame(flushTokens);
            }
          }
          break;
        }
        case 'tool_call': {
          const d = parsed.data as Record<string, unknown>;
          dispatch({
            type: 'chat/stream/toolCall',
            call: {
              id: String(d.id ?? d.tool_use_id ?? Date.now()),
              name: String(d.name ?? 'tool'),
              input: d.input ?? d,
              state: 'running',
            },
          });
          break;
        }
        case 'tool_result': {
          const d = parsed.data as { call?: { id?: string }; result?: string } | undefined;
          dispatch({
            type: 'chat/stream/toolResult',
            id: String(d?.call?.id ?? Date.now()),
            result: String(d?.result ?? ''),
          });
          break;
        }
        case 'message_complete': {
          flushTokens();
          const d = parsed.data as { assistant_text?: string; message_id?: string } | undefined;
          dispatch({
            type: 'chat/stream/complete',
            text: String(d?.assistant_text ?? ''),
            messageId: String(d?.message_id ?? `complete-${Date.now()}`),
          });
          break;
        }
        case 'status': {
          const d = parsed.data as { state?: string; error?: string } | undefined;
          if (d?.state === 'cancelled') {
            flushTokens();
            dispatch({ type: 'chat/stream/cancelled' });
          } else if (d?.state === 'error') {
            dispatch({ type: 'chat/stream/error', message: String(d.error ?? 'error') });
          }
          break;
        }
      }
    };

    return () => {
      es.close();
    };
  }, [sessionId, dispatch]);
}
