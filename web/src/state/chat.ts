export type ChatMessage = {
  id: string;
  role: string;
  content: string;
  timestamp: number;
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
  composer: { text: string };
  streaming: StreamingState;
};

export const initialChatState: ChatState = {
  messages: [],
  composer: { text: '' },
  streaming: { status: 'idle', assistantDraft: '', toolCalls: [] },
};

export type ChatAction =
  | { type: 'chat/history/loaded'; messages: ChatMessage[] }
  | { type: 'chat/composer/setText'; text: string }
  | { type: 'chat/stream/start'; userText: string }
  | { type: 'chat/stream/token'; delta: string }
  | { type: 'chat/stream/toolCall'; call: ToolCall }
  | { type: 'chat/stream/toolResult'; id: string; result: string }
  | { type: 'chat/stream/done'; assistantText: string }
  | { type: 'chat/stream/error'; message: string }
  | { type: 'chat/stream/rollbackUserMessage' };

export function chatReducer(state: ChatState, action: ChatAction): ChatState {
  switch (action.type) {
    case 'chat/history/loaded':
      return { ...state, messages: action.messages };
    case 'chat/composer/setText':
      return { ...state, composer: { text: action.text } };
    case 'chat/stream/start':
      return {
        ...state,
        messages: [
          ...state.messages,
          {
            id: `user-${Date.now()}`,
            role: 'user',
            content: JSON.stringify({ text: action.userText }),
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
            c.id === action.id ? { ...c, state: 'done', result: action.result } : c,
          ),
        },
      };
    case 'chat/stream/done':
      return {
        ...state,
        messages: [
          ...state.messages,
          {
            id: `asst-${Date.now()}`,
            role: 'assistant',
            content: action.assistantText,
            timestamp: Date.now(),
          },
        ],
        streaming: { status: 'idle', assistantDraft: '', toolCalls: [] },
      };
    case 'chat/stream/error':
      return {
        ...state,
        streaming: { ...state.streaming, status: 'error', error: action.message },
      };
    case 'chat/stream/rollbackUserMessage':
      return {
        ...state,
        messages: state.messages.slice(0, -1),
        streaming: { status: 'idle', assistantDraft: '', toolCalls: [] },
      };
  }
}
