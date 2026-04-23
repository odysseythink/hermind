import { useEffect, useReducer, useState } from 'react';
import { useTranslation } from 'react-i18next';
import ConversationHeader from './ConversationHeader';
import MessageList from './MessageList';
import ComposerBar from './ComposerBar';
import Toast from './Toast';
import styles from './ChatWorkspace.module.css';
import { useChatStream } from '../../hooks/useChatStream';
import { chatReducer, initialChatState } from '../../state/chat';
import { apiFetch, ApiError } from '../../api/client';
import { ConversationHistoryResponseSchema } from '../../api/schemas';

type Props = {
  instanceRoot: string;
  providerConfigured?: boolean;
  modelOptions: string[];
  currentModel: string;
};

export default function ChatWorkspace({
  instanceRoot,
  providerConfigured = true,
  modelOptions,
  currentModel,
}: Props) {
  const { t } = useTranslation('ui');
  const [state, dispatch] = useReducer(chatReducer, initialChatState);
  const [toast, setToast] = useState<string | null>(null);
  const [runtimeModel, setRuntimeModel] = useState<string>(currentModel);

  useChatStream(dispatch);

  // Track currentModel changes (e.g. settings save reloads meta).
  useEffect(() => {
    setRuntimeModel((prev) => (prev === '' && currentModel ? currentModel : prev));
  }, [currentModel]);

  useEffect(() => {
    const ctrl = new AbortController();
    apiFetch('/api/conversation', {
      schema: ConversationHistoryResponseSchema,
      signal: ctrl.signal,
    })
      .then((r) =>
        dispatch({
          type: 'chat/history/loaded',
          messages: r.messages.map((m) => ({
            id: String(m.id),
            role: m.role,
            content: m.content,
            timestamp: m.timestamp,
          })),
        }),
      )
      .catch(() => {
        /* empty history is fine */
      });
    return () => ctrl.abort();
  }, []);

  async function handleSend() {
    const text = state.composer.text.trim();
    if (!text) return;
    dispatch({ type: 'chat/composer/setText', text: '' });
    dispatch({ type: 'chat/stream/start', userText: text });
    try {
      await apiFetch('/api/conversation/messages', {
        method: 'POST',
        body: { user_message: text, model: runtimeModel },
      });
    } catch (err) {
      dispatch({ type: 'chat/stream/rollbackUserMessage' });
      if (err instanceof ApiError) {
        if (err.status === 409) setToast(t('chat.errorBusy'));
        else if (err.status === 503) setToast(t('chat.errorNoProvider'));
        else setToast(t('chat.errorSendFailed', { msg: err.message }));
      } else {
        setToast(t('chat.errorSendFailed', { msg: err instanceof Error ? err.message : '' }));
      }
    }
  }

  async function handleStop() {
    try {
      await apiFetch('/api/conversation/cancel', { method: 'POST' });
    } catch (err) {
      console.warn('cancel failed', err);
    }
  }

  return (
    <div className={styles.workspace}>
      <ConversationHeader
        instanceRoot={instanceRoot}
        modelOptions={modelOptions}
        selectedModel={runtimeModel}
        onSelectModel={setRuntimeModel}
        onStop={handleStop}
        streaming={state.streaming.status === 'running'}
      />
      <MessageList
        messages={state.messages}
        streamingDraft={state.streaming.assistantDraft}
        streamingToolCalls={state.streaming.toolCalls}
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
          if (cmd === 'clear') dispatch({ type: 'chat/composer/setText', text: '' });
        }}
      />
      {toast && <Toast message={toast} onDismiss={() => setToast(null)} />}
    </div>
  );
}
