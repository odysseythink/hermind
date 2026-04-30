export type Attachment = {
  id: string;
  name: string;
  type: string;
  url: string;
  size: number;
};

export type Source = {
  id: string;
  title: string;
  text: string;
  metadata?: Record<string, unknown>;
};

export type MessageMetrics = {
  promptTokens?: number;
  completionTokens?: number;
  latencyMs?: number;
};

export type ChatMessage = {
  id: string;
  role: string;
  content: string;
  timestamp: number;
  chatId?: number;
  attachments?: Attachment[];
  sources?: Source[];
  feedbackScore?: number | null;
  metrics?: MessageMetrics;
  error?: boolean;
  pending?: boolean;
  animate?: boolean;
};

export type ToolCall = {
  id: string;
  name: string;
  input: unknown;
  state: 'running' | 'done' | 'error';
  result?: string;
};

export type StreamingState = {
  status: 'idle' | 'running' | 'error';
  assistantDraft: string;
  toolCalls: ToolCall[];
  error?: string;
};

export type ChatState = {
  messages: ChatMessage[];
  composer: {
    text: string;
    attachments: Attachment[];
  };
  streaming: StreamingState;
  suggestions: string[];
};

export const initialChatState: ChatState = {
  messages: [],
  composer: { text: '', attachments: [] },
  streaming: { status: 'idle', assistantDraft: '', toolCalls: [] },
  suggestions: [],
};

export type ChatAction =
  | { type: 'chat/history/loaded'; messages: ChatMessage[] }
  | { type: 'chat/composer/setText'; text: string }
  | { type: 'chat/composer/setAttachments'; attachments: Attachment[] }
  | { type: 'chat/stream/start'; userText: string }
  | { type: 'chat/stream/token'; delta: string }
  | { type: 'chat/stream/toolCall'; call: ToolCall }
  | { type: 'chat/stream/toolResult'; id: string; result: string }
  | { type: 'chat/stream/done'; assistantText: string }
  | { type: 'chat/stream/error'; message: string }
  | { type: 'chat/stream/rollbackUserMessage' }
  | { type: 'chat/message/edit'; id: string; content: string }
  | { type: 'chat/message/delete'; id: string }
  | { type: 'chat/message/regenerate'; id: string }
  | { type: 'chat/suggestions/loaded'; suggestions: string[] };

export function chatReducer(state: ChatState, action: ChatAction): ChatState {
  switch (action.type) {
    case 'chat/history/loaded':
      return { ...state, messages: action.messages };

    case 'chat/composer/setText':
      return { ...state, composer: { ...state.composer, text: action.text } };

    case 'chat/composer/setAttachments':
      return { ...state, composer: { ...state.composer, attachments: action.attachments } };

    case 'chat/stream/start':
      return {
        ...state,
        messages: [
          ...state.messages,
          {
            id: `user-${Date.now()}`,
            role: 'user',
            content: action.userText,
            timestamp: Date.now(),
          },
        ],
        streaming: { status: 'running', assistantDraft: '', toolCalls: [] },
      };

    case 'chat/stream/token':
      return {
        ...state,
        streaming: {
          ...state.streaming,
          assistantDraft: state.streaming.assistantDraft + action.delta,
        },
      };

    case 'chat/stream/toolCall': {
      const idx = state.streaming.toolCalls.findIndex((t) => t.id === action.call.id);
      const next = [...state.streaming.toolCalls];
      if (idx >= 0) {
        next[idx] = action.call;
      } else {
        next.push(action.call);
      }
      return { ...state, streaming: { ...state.streaming, toolCalls: next } };
    }

    case 'chat/stream/toolResult': {
      const idx = state.streaming.toolCalls.findIndex((t) => t.id === action.id);
      if (idx < 0) return state;
      const next = [...state.streaming.toolCalls];
      next[idx] = { ...next[idx], result: action.result, state: 'done' as const };
      return { ...state, streaming: { ...state.streaming, toolCalls: next } };
    }

    case 'chat/stream/done': {
      const assistantMsg: ChatMessage = {
        id: `asst-${Date.now()}`,
        role: 'assistant',
        content: state.streaming.assistantDraft || action.assistantText,
        timestamp: Date.now(),
      };
      return {
        ...state,
        messages: [...state.messages, assistantMsg],
        streaming: { status: 'idle', assistantDraft: '', toolCalls: [] },
      };
    }

    case 'chat/stream/error':
      return {
        ...state,
        streaming: { status: 'error', assistantDraft: '', toolCalls: [], error: action.message },
      };

    case 'chat/stream/rollbackUserMessage':
      return {
        ...state,
        messages: state.messages.slice(0, -1),
        streaming: { status: 'idle', assistantDraft: '', toolCalls: [] },
      };

    case 'chat/message/edit': {
      const msgs = state.messages.map((m) =>
        m.id === action.id ? { ...m, content: action.content, pending: false } : m
      );
      return { ...state, messages: msgs };
    }

    case 'chat/message/delete': {
      const filtered = state.messages.filter((m) => m.id !== action.id);
      return { ...state, messages: filtered };
    }

    case 'chat/message/regenerate': {
      const targetIndex = state.messages.findIndex((m) => m.id === action.id);
      if (targetIndex < 0) return state;
      return {
        ...state,
        messages: state.messages.slice(0, targetIndex),
        streaming: { status: 'running', assistantDraft: '', toolCalls: [] },
      };
    }

    case 'chat/suggestions/loaded':
      return { ...state, suggestions: action.suggestions };

    default:
      return state;
  }
}
