import { useReducer, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import ChatSidebar from './ChatSidebar';
import ConversationHeader from './ConversationHeader';
import MessageList from './MessageList';
import ComposerBar from './ComposerBar';
import SessionSettingsDrawer from './SessionSettingsDrawer';
import Toast from './Toast';
import styles from './ChatWorkspace.module.css';
import { useSessionList } from '../../hooks/useSessionList';
import { useChatStream } from '../../hooks/useChatStream';
import { chatReducer, initialChatState } from '../../state/chat';
import { apiFetch, ApiError } from '../../api/client';
import {
  MessageSubmitResponseSchema,
  MessagesResponseSchema,
  SessionSummarySchema,
  type SessionPatch,
  type SessionSummary,
} from '../../api/schemas';


type Props = {
  sessionId: string | null;
  onChangeSession: (id: string) => void;
  providerConfigured?: boolean;
  modelOptions: string[];
  onEnsureModelsLoaded?: () => Promise<void>;
};

export default function ChatWorkspace({ sessionId, onChangeSession, providerConfigured = true, modelOptions, onEnsureModelsLoaded }: Props) {
  const { t } = useTranslation('ui');
  const [state, dispatch] = useReducer(chatReducer, initialChatState);
  const { sessions, newSession, insertSession, patchSession } = useSessionList();
  const [toast, setToast] = useState<string | null>(null);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [drawerSession, setDrawerSession] = useState<SessionSummary | null>(null);

  useChatStream(sessionId, dispatch, insertSession, (id, patch) => patchSession(id, patch));

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
            id: String(m.id),
            role: m.role,
            content: m.content,
            timestamp: m.timestamp ?? Date.now(),
          })),
        });
      })
      .catch((err) => {
        if (ctrl.signal.aborted) return;
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
        body: { text },
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
      // Local state refresh flows through the session_updated SSE event,
      // which useChatStream pipes to patchSession. No optimistic write here.
    } catch (err) {
      setToast(t('chat.renameFailed', { msg: err instanceof Error ? err.message : '' }));
      throw err;
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

  async function handleOpenSettings() {
    if (!sessionId) return;
    // Fire-and-forget: populate the model dropdown from all configured
    // providers. The drawer opens immediately with whatever's already cached;
    // newly-fetched models appear as they arrive.
    if (onEnsureModelsLoaded) void onEnsureModelsLoaded();
    // Fetch authoritative session state before mounting the drawer so the
    // user always sees the current stored model + system_prompt, even if
    // the cached sessions list is stale or missing this row entirely.
    try {
      const fresh = await apiFetch(`/api/sessions/${encodeURIComponent(sessionId)}`, {
        schema: SessionSummarySchema,
      });
      setDrawerSession(fresh);
      setSettingsOpen(true);
      patchSession(fresh.id, {
        title: fresh.title,
        model: fresh.model,
        system_prompt: fresh.system_prompt,
      });
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        setToast(t('chat.settings.loadFailed', { msg: 'session not found' }));
        return;
      }
      // Fall back to the cached row so the user can at least see stale data
      // and attempt a save; flag the degraded state via toast.
      const cached = sessions.find((s) => s.id === sessionId) ?? null;
      if (cached) {
        setDrawerSession(cached);
        setSettingsOpen(true);
      }
      setToast(
        t('chat.settings.loadFailed', {
          msg: err instanceof Error ? err.message : 'network error',
        }),
      );
    }
  }

  async function handleSettingsSave(patch: SessionPatch) {
    if (!sessionId) return;
    try {
      await apiFetch(`/api/sessions/${encodeURIComponent(sessionId)}`, {
        method: 'PATCH',
        body: patch,
      });
      // Local state refresh flows through the session_updated SSE event.
    } catch (err) {
      if (err instanceof ApiError && err.status === 400) {
        setToast(t('chat.settings.saveTooLong'));
      } else {
        setToast(t('chat.settings.saveFailed', { msg: err instanceof Error ? err.message : '' }));
      }
      throw err;
    }
  }

  const activeSession = sessions.find((s) => s.id === sessionId);
  const activeTitle = activeSession?.title ?? t('chat.newConversation');

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
          onOpenSettings={handleOpenSettings}
          settingsDisabled={!sessionId}
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
                if (sessionId) void handleOpenSettings();
                dispatch({ type: 'chat/composer/setText', text: '' });
                break;
              // /model used to trigger a header dropdown; open the drawer instead.
              case 'model':
              case 'clear':
                dispatch({ type: 'chat/composer/setText', text: '' });
                break;
            }
          }}
        />
        {settingsOpen && drawerSession && (
          <SessionSettingsDrawer
            open={settingsOpen}
            session={drawerSession}
            modelOptions={modelOptions}
            onClose={() => {
              setSettingsOpen(false);
              setDrawerSession(null);
            }}
            onSave={handleSettingsSave}
          />
        )}
      </main>
      {toast && <Toast message={toast} onDismiss={() => setToast(null)} />}
    </div>
  );
}
