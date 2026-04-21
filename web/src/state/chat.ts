export type ChatMessage = {
  id: string;
  role: 'user' | 'assistant' | 'system' | 'tool';
  content: string;
  timestamp: number;
  toolCalls?: ToolCallSnapshot[];
  truncated?: true;
};

export type ToolCallSnapshot = {
  id: string;
  name: string;
  input: unknown;
  result?: string;
  state: 'running' | 'done' | 'error';
};

export type SessionSummary = {
  id: string;
  title: string;
  updatedAt: number;
};

export type ChatState = {
  activeSessionId: string | null;
  sessions: SessionSummary[];
  messagesBySession: Record<string, ChatMessage[]>;
  streaming: {
    sessionId: string | null;
    assistantDraft: string;
    toolCalls: ToolCallSnapshot[];
    status: 'idle' | 'running' | 'cancelling' | 'error';
    error: string | null;
  };
  composer: {
    text: string;
    selectedModel: string;
  };
};

export const initialChatState: ChatState = {
  activeSessionId: null,
  sessions: [],
  messagesBySession: {},
  streaming: {
    sessionId: null,
    assistantDraft: '',
    toolCalls: [],
    status: 'idle',
    error: null,
  },
  composer: { text: '', selectedModel: '' },
};

export type ChatAction =
  | { type: 'chat/session/select'; sessionId: string }
  | { type: 'chat/session/created'; id: string; title: string }
  | { type: 'chat/session/listLoaded'; sessions: SessionSummary[] }
  | { type: 'chat/messages/loaded'; sessionId: string; messages: ChatMessage[] }
  | { type: 'chat/stream/start'; sessionId: string; userText: string }
  | { type: 'chat/stream/rollbackUserMessage'; sessionId: string }
  | { type: 'chat/stream/token'; delta: string }
  | { type: 'chat/stream/toolCall'; call: ToolCallSnapshot }
  | { type: 'chat/stream/toolResult'; id: string; result: string }
  | { type: 'chat/stream/complete'; text: string; messageId: string }
  | { type: 'chat/stream/cancelled' }
  | { type: 'chat/stream/error'; message: string }
  | { type: 'chat/composer/setText'; text: string }
  | { type: 'chat/composer/setModel'; model: string };

export function chatReducer(state: ChatState, action: ChatAction): ChatState {
  switch (action.type) {
    case 'chat/session/select':
      return { ...state, activeSessionId: action.sessionId };

    case 'chat/session/created':
      return {
        ...state,
        sessions: [
          { id: action.id, title: action.title, updatedAt: Date.now() },
          ...state.sessions,
        ],
        activeSessionId: action.id,
      };

    case 'chat/session/listLoaded':
      return { ...state, sessions: action.sessions };

    case 'chat/messages/loaded':
      return {
        ...state,
        messagesBySession: {
          ...state.messagesBySession,
          [action.sessionId]: action.messages,
        },
      };

    case 'chat/stream/start': {
      const existing = state.messagesBySession[action.sessionId] ?? [];
      const userMsg: ChatMessage = {
        id: `draft-user-${Date.now()}`,
        role: 'user',
        content: action.userText,
        timestamp: Date.now(),
      };
      return {
        ...state,
        messagesBySession: {
          ...state.messagesBySession,
          [action.sessionId]: [...existing, userMsg],
        },
        streaming: {
          sessionId: action.sessionId,
          assistantDraft: '',
          toolCalls: [],
          status: 'running',
          error: null,
        },
      };
    }

    case 'chat/stream/rollbackUserMessage': {
      const existing = state.messagesBySession[action.sessionId] ?? [];
      const trimmed = existing.slice();
      while (trimmed.length > 0 && trimmed.at(-1)?.id.startsWith('draft-')) {
        trimmed.pop();
      }
      return {
        ...state,
        messagesBySession: {
          ...state.messagesBySession,
          [action.sessionId]: trimmed,
        },
        streaming: { ...initialChatState.streaming },
      };
    }

    case 'chat/stream/token':
      return {
        ...state,
        streaming: {
          ...state.streaming,
          assistantDraft: state.streaming.assistantDraft + action.delta,
        },
      };

    case 'chat/stream/toolCall':
      return {
        ...state,
        streaming: {
          ...state.streaming,
          toolCalls: [...state.streaming.toolCalls, action.call],
        },
      };

    case 'chat/stream/toolResult':
      return {
        ...state,
        streaming: {
          ...state.streaming,
          toolCalls: state.streaming.toolCalls.map((c) =>
            c.id === action.id ? { ...c, result: action.result, state: 'done' } : c,
          ),
        },
      };

    case 'chat/stream/complete': {
      const sid = state.streaming.sessionId;
      if (!sid) return state;
      const existing = state.messagesBySession[sid] ?? [];
      const assistantMsg: ChatMessage = {
        id: action.messageId,
        role: 'assistant',
        content: action.text,
        timestamp: Date.now(),
        toolCalls: state.streaming.toolCalls,
      };
      return {
        ...state,
        messagesBySession: {
          ...state.messagesBySession,
          [sid]: [...existing, assistantMsg],
        },
        streaming: { ...initialChatState.streaming },
      };
    }

    case 'chat/stream/cancelled': {
      const sid = state.streaming.sessionId;
      if (!sid) return state;
      const existing = state.messagesBySession[sid] ?? [];
      const assistantMsg: ChatMessage = {
        id: `draft-assistant-cancelled-${Date.now()}`,
        role: 'assistant',
        content: state.streaming.assistantDraft,
        timestamp: Date.now(),
        toolCalls: state.streaming.toolCalls,
        truncated: true,
      };
      return {
        ...state,
        messagesBySession: {
          ...state.messagesBySession,
          [sid]: [...existing, assistantMsg],
        },
        streaming: { ...initialChatState.streaming },
      };
    }

    case 'chat/stream/error': {
      return {
        ...state,
        streaming: {
          ...state.streaming,
          status: 'error',
          error: action.message,
        },
      };
    }

    case 'chat/composer/setText':
      return { ...state, composer: { ...state.composer, text: action.text } };

    case 'chat/composer/setModel':
      return { ...state, composer: { ...state.composer, selectedModel: action.model } };

    default:
      return state;
  }
}
