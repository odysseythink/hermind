export interface ChatMessage {
	id: string;
	role: "user" | "assistant";
	content: string;
	toolCalls?: ToolCallEvent[];
}

export interface ToolCallEvent {
	id: string;
	name: string;
	input: Record<string, unknown>;
	result?: string;
}

export interface SSEEvent {
	type: "message_chunk" | "tool_call" | "tool_result" | "done" | "error";
	data: Record<string, unknown>;
}
