import { useEffect, useReducer, useState, useTransition } from 'react';
import { useTranslation } from 'react-i18next';
import ConversationHeader from './ConversationHeader';
import ChatHistory from './ChatHistory';
import PromptInput from './PromptInput';
import Toast from './Toast';
import ModelPicker from './ModelPicker';
import EmptyState from './EmptyState';
import styles from './ChatWorkspace.module.css';
import { useChatStream } from '../../hooks/useChatStream';
import { chatReducer, initialChatState } from '../../state/chat';
import { apiFetch, apiPut, apiDelete, ApiError } from '../../api/client';
import {
  ConversationHistoryResponseSchema,
  SuggestionsResponseSchema,
  MetaResponseSchema,
} from '../../api/schemas';

type Props = {
  instanceRoot: string;
  providerConfigured?: boolean;
};

export default function ChatWorkspace({
  instanceRoot,
  providerConfigured = true,
}: Props) {
  const { t } = useTranslation('ui');
  const [state, dispatch] = useReducer(chatReducer, initialChatState);
  const [toast, setToast] = useState<string | null>(null);
  const [currentModel, setCurrentModel] = useState('');
  const [, startTransition] = useTransition();

  useChatStream(dispatch);

  useEffect(() => {
    const ctrl = new AbortController();
    apiFetch('/api/conversation', {
      schema: ConversationHistoryResponseSchema,
      signal: ctrl.signal,
    })
      .then((r) => {
        const messages = r.messages.map((m) => ({
          id: String(m.id),
          chatId: m.id,
          role: m.role,
          content: m.content,
          timestamp: m.timestamp,
        }));
        startTransition(() => {
          dispatch({ type: 'chat/history/loaded', messages });
        });
      })
      .catch(() => {
        /* empty history is fine */
      });
    return () => ctrl.abort();
  }, [startTransition]);

  useEffect(() => {
    const ctrl = new AbortController();
    apiFetch('/api/suggestions', {
      schema: SuggestionsResponseSchema,
      signal: ctrl.signal,
    })
      .then((r) => {
        startTransition(() => {
          dispatch({ type: 'chat/suggestions/loaded', suggestions: r.suggestions });
        });
      })
      .catch(() => {
        /* missing suggestions is fine */
      });
    return () => ctrl.abort();
  }, [startTransition]);

  useEffect(() => {
    const ctrl = new AbortController();
    apiFetch('/api/status', { schema: MetaResponseSchema, signal: ctrl.signal })
      .then((r) => setCurrentModel(r.current_model))
      .catch(() => {});
    return () => ctrl.abort();
  }, []);

  async function handleSend(overrideText?: string) {
    const text = (overrideText ?? state.composer.text).trim();
    if (!text) return;
    dispatch({ type: 'chat/composer/setText', text: '' });
    dispatch({ type: 'chat/stream/start', userText: text });
    try {
      await apiFetch('/api/conversation/messages', {
        method: 'POST',
        body: { user_message: text },
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

  async function handleEdit(id: string, content: string) {
    const msg = state.messages.find((m) => m.id === id);
    if (!msg || msg.chatId === undefined) return;
    dispatch({ type: 'chat/message/edit', id, content });
    try {
      await apiPut(`/api/conversation/messages/${msg.chatId}`, { content });
    } catch (err) {
      setToast(t('chat.errorEditFailed', { msg: err instanceof Error ? err.message : '' }));
    }
  }

  async function handleDelete(id: string) {
    const msg = state.messages.find((m) => m.id === id);
    if (!msg || msg.chatId === undefined) return;
    dispatch({ type: 'chat/message/delete', id });
    try {
      await apiDelete(`/api/conversation/messages/${msg.chatId}`);
    } catch (err) {
      setToast(t('chat.errorDeleteFailed', { msg: err instanceof Error ? err.message : '' }));
    }
  }

  async function handleRegenerate(id: string) {
    const targetIndex = state.messages.findIndex((m) => m.id === id);
    if (targetIndex < 0) return;
    const msg = state.messages[targetIndex];
    if (!msg || msg.chatId === undefined) return;

    let precedingUserContent = '';
    for (let i = targetIndex - 1; i >= 0; i--) {
      if (state.messages[i].role === 'user') {
        precedingUserContent = state.messages[i].content;
        break;
      }
    }

    dispatch({ type: 'chat/message/regenerate', id });

    try {
      await apiDelete(`/api/conversation/messages/${msg.chatId}`);
    } catch (err) {
      console.warn('regenerate delete failed', err);
    }

    try {
      await apiFetch('/api/conversation/messages', {
        method: 'POST',
        body: { user_message: precedingUserContent },
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

  const handleSuggestionClick = (text: string) => {
    dispatch({ type: 'chat/composer/setText', text });
    handleSend(text);
  };

  const isEmpty =
    state.messages.length === 0 &&
    !state.streaming.assistantDraft &&
    state.streaming.toolCalls.length === 0;

  return (
    <div className={`${styles.workspace} ${isEmpty ? styles.emptyMode : styles.chatMode}`}>
      <div className={styles.modelPicker}>
        <ModelPicker modelName={currentModel} />
      </div>

      {isEmpty ? (
        <>
          <EmptyState
            suggestions={state.suggestions}
            onSuggestionClick={handleSuggestionClick}
          />
          <div className={styles.promptWrapper}>
            <PromptInput
              text={state.composer.text}
              onTextChange={(txt) => dispatch({ type: 'chat/composer/setText', text: txt })}
              onSubmit={handleSend}
              disabled={!providerConfigured || state.streaming.status === 'running'}
              suggestions={state.suggestions}
            />
          </div>
        </>
      ) : (
        <>
          <ConversationHeader
            instanceRoot={instanceRoot}
            onStop={handleStop}
            streaming={state.streaming.status === 'running'}
          />
          <ChatHistory
            messages={state.messages}
            streamingDraft={state.streaming.assistantDraft}
            streamingToolCalls={state.streaming.toolCalls}
            onEdit={handleEdit}
            onDelete={handleDelete}
            onRegenerate={handleRegenerate}
          />
          {state.streaming.status === 'error' && state.streaming.error && (
            <div role="alert" className={styles.errorBanner}>
              {state.streaming.error}
            </div>
          )}
          <PromptInput
            text={state.composer.text}
            onTextChange={(txt) => dispatch({ type: 'chat/composer/setText', text: txt })}
            onSubmit={handleSend}
            disabled={!providerConfigured || state.streaming.status === 'running'}
            suggestions={state.suggestions}
          />
        </>
      )}
      {toast && <Toast message={toast} onDismiss={() => setToast(null)} />}
    </div>
  );
}
