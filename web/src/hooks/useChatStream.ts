import { useEffect, useRef } from 'react';
import type { ChatAction } from '../state/chat';

type Dispatch = (a: ChatAction) => void;

export function useChatStream(dispatch: Dispatch) {
  const tokenBufRef = useRef('');
  const rafPendingRef = useRef(false);

  useEffect(() => {
    const es = new EventSource('/api/sse');

    function flushTokens() {
      rafPendingRef.current = false;
      if (tokenBufRef.current) {
        dispatch({ type: 'chat/stream/token', delta: tokenBufRef.current });
        tokenBufRef.current = '';
      }
    }

    es.onmessage = (ev) => {
      let parsed: { type?: string; data?: Record<string, unknown> };
      try {
        parsed = JSON.parse(ev.data);
      } catch {
        return;
      }
      switch (parsed.type) {
        case 'message_chunk': {
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
          const d = (parsed.data ?? {}) as Record<string, unknown>;
          dispatch({
            type: 'chat/stream/toolCall',
            call: {
              id: String(d.id ?? Date.now()),
              name: String(d.name ?? 'tool'),
              input: d.input ?? null,
              state: 'running',
            },
          });
          break;
        }
        case 'tool_result': {
          const d = (parsed.data ?? {}) as Record<string, unknown>;
          dispatch({
            type: 'chat/stream/toolResult',
            id: String(d.id ?? ''),
            result: String(d.result ?? ''),
          });
          break;
        }
        case 'done': {
          flushTokens();
          dispatch({ type: 'chat/stream/done', assistantText: '' });
          break;
        }
        case 'error': {
          const d = parsed.data as { message?: string } | undefined;
          dispatch({ type: 'chat/stream/error', message: d?.message ?? 'stream error' });
          break;
        }
      }
    };

    es.onerror = () => {
      // Connection broke — surface as error once; EventSource will try
      // to reconnect on its own.
      dispatch({ type: 'chat/stream/error', message: 'SSE disconnected' });
    };

    return () => {
      es.close();
    };
  }, [dispatch]);
}
