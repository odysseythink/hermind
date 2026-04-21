import { useReducer, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import ChatSidebar from './ChatSidebar';
import ConversationHeader from './ConversationHeader';
import MessageList from './MessageList';
import ComposerBar from './ComposerBar';
import Toast from './Toast';
import styles from './ChatWorkspace.module.css';
import { useSessionList } from '../../hooks/useSessionList';
import { useChatStream } from '../../hooks/useChatStream';
import { chatReducer, initialChatState } from '../../state/chat';
import { apiFetch, ApiError } from '../../api/client';
import {
  MessageSubmitResponseSchema,
  MessagesResponseSchema,
} from '../../api/schemas';

type Props = {
  sessionId: string | null;
  onChangeSession: (id: string) => void;
  providerConfigured?: boolean;
};

const MODEL_OPTIONS = ['', 'claude-opus-4-7', 'claude-sonnet-4-6', 'gpt-4'];

export default function ChatWorkspace({ sessionId, onChangeSession, providerConfigured = true }: Props) {
  const { t } = useTranslation('ui');
  const [state, dispatch] = useReducer(chatReducer, initialChatState);
  const { sessions, newSession, insertSession, renameSession } = useSessionList();
  const [toast, setToast] = useState<string | null>(null);

  // Subscribe to SSE for the active session.
  useChatStream(sessionId, dispatch, insertSession);

  // Load message history when sessionId changes.
  useEffect(() => {
    if (!sessionId) return;
    const ctrl = new AbortController();
    apiFetch(`/api/sessions/${encodeURIComponent(sessionId)}/messages`, {
      schema: MessagesResponseSchema,
      signal: ctrl.signal,
    })
      .then((r) => {
        dispatch({
          type: 'chat/messages/loaded',
          sessionId,
          messages: r.messages.map((m) => ({
            id: m.id,
            role: m.role,
            content: m.content,
            timestamp: m.timestamp ?? Date.now(),
          })),
        });
      })
      .catch((err) => {
        if (ctrl.signal.aborted) return;
        // 404 = new session, no history yet — silent.
        if (err instanceof ApiError && err.status === 404) return;
        console.warn('load messages failed', err);
      });
    return () => ctrl.abort();
  }, [sessionId]);

  async function handleSend() {
    if (!sessionId) return;
    const text = state.composer.text.trim();
    if (!text) return;
    dispatch({ type: 'chat/composer/setText', text: '' });
    dispatch({ type: 'chat/stream/start', sessionId, userText: text });
    try {
      await apiFetch(`/api/sessions/${encodeURIComponent(sessionId)}/messages`, {
        method: 'POST',
        body: { text, model: state.composer.selectedModel || undefined },
        schema: MessageSubmitResponseSchema,
      });
    } catch (err) {
      dispatch({ type: 'chat/stream/rollbackUserMessage', sessionId });
      if (err instanceof ApiError) {
        if (err.status === 409) setToast(t('chat.errorBusy'));
        else if (err.status === 503) setToast(t('chat.errorNoProvider'));
        else setToast(t('chat.errorSendFailed', { msg: err.message }));
      } else {
        setToast(t('chat.errorSendFailed', { msg: err instanceof Error ? err.message : '' }));
      }
    }
  }

  async function handleRename(id: string, title: string): Promise<void> {
    try {
      await apiFetch(`/api/sessions/${encodeURIComponent(id)}`, {
        method: 'PATCH',
        body: { title },
      });
      renameSession(id, title);
    } catch (err) {
      setToast(
        t('chat.renameFailed', { msg: err instanceof Error ? err.message : '' }),
      );
      throw err; // let SessionItem reset its local draft
    }
  }

  async function handleStop() {
    if (!sessionId) return;
    try {
      await apiFetch(`/api/sessions/${encodeURIComponent(sessionId)}/cancel`, {
        method: 'POST',
      });
    } catch (err) {
      console.warn('cancel failed', err);
    }
  }

  const activeTitle =
    sessions.find((s) => s.id === sessionId)?.title ?? t('chat.newConversation');

  return (
    <div className={styles.workspace}>
      <ChatSidebar
        sessions={sessions}
        activeId={sessionId}
        onSelect={onChangeSession}
        onNew={() => {
          const id = newSession();
          onChangeSession(id);
        }}
        onRename={handleRename}
      />
      <main className={styles.main}>
        <ConversationHeader
          title={activeTitle}
          model={state.composer.selectedModel}
          modelOptions={MODEL_OPTIONS}
          onModelChange={(m) => dispatch({ type: 'chat/composer/setModel', model: m })}
        />
        <MessageList
          messages={state.messagesBySession[sessionId ?? ''] ?? []}
          streamingDraft={state.streaming.assistantDraft}
          streamingToolCalls={state.streaming.toolCalls}
          streamingSessionId={state.streaming.sessionId}
          activeSessionId={sessionId}
        />
        {state.streaming.status === 'error' && state.streaming.error && (
          <div role="alert" className={styles.errorBanner}>
            {state.streaming.error}
          </div>
        )}
        <ComposerBar
          text={state.composer.text}
          onChangeText={(txt) => dispatch({ type: 'chat/composer/setText', text: txt })}
          onSend={handleSend}
          onStop={handleStop}
          disabled={!providerConfigured}
          streaming={state.streaming.status === 'running'}
          onSlashCommand={(cmd) => {
            switch (cmd) {
              case 'new': {
                const id = newSession();
                onChangeSession(id);
                dispatch({ type: 'chat/composer/setText', text: '' });
                break;
              }
              case 'settings':
                window.location.hash = '#/settings/models';
                dispatch({ type: 'chat/composer/setText', text: '' });
                break;
              case 'model':
              case 'clear':
                dispatch({ type: 'chat/composer/setText', text: '' });
                break;
            }
          }}
        />
      </main>
      {toast && <Toast message={toast} onDismiss={() => setToast(null)} />}
    </div>
  );
}
