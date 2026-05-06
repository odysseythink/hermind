import { ItemView, WorkspaceLeaf } from "obsidian";
import { HermindSettings } from "../settings";
import { HermindAPI } from "../api";
import { HermindSSE } from "../sse";
import { extractContext } from "../context";
import { ChatUI } from "./ChatUI";
import { ChatMessage, SSEEvent } from "./types";

export const VIEW_TYPE_HERMIND = "hermind-chat-view";

export class ChatView extends ItemView {
	private ui: ChatUI;
	private api: HermindAPI;
	private sse: HermindSSE;
	private messages: ChatMessage[] = [];
	private currentAssistantMessage = "";
	private currentToolCalls: Record<string, { name: string; input: Record<string, unknown>; result?: string }> = {};

	constructor(leaf: WorkspaceLeaf, private settings: HermindSettings) {
		super(leaf);
		this.api = new HermindAPI(settings.hermindUrl);
		this.sse = new HermindSSE(
			settings.hermindUrl,
			(evt) => this.handleSSE(evt),
			(msg) => this.ui?.showError(msg)
		);
	}

	getViewType(): string {
		return VIEW_TYPE_HERMIND;
	}

	getDisplayText(): string {
		return "Hermind";
	}

	async onOpen(): Promise<void> {
		this.ui = new ChatUI(this.containerEl, {
			onSend: (text) => this.sendMessage(text),
			onSave: () => this.saveConversation(),
			showToolCalls: this.settings.showToolCalls,
		});
		this.sse.connect();
	}

	async onClose(): Promise<void> {
		this.sse.disconnect();
	}

	sendSelection(selection: string): void {
		this.sendMessage(selection);
	}

	private async sendMessage(text: string): Promise<void> {
		const userMsg: ChatMessage = { id: crypto.randomUUID(), role: "user", content: text };
		this.messages.push(userMsg);
		this.ui.addMessage(userMsg);

		this.currentAssistantMessage = "";
		this.currentToolCalls = {};
		this.ui.startAssistantMessage();

		try {
			const ctx = this.settings.autoAttachContext ? extractContext(this.app) : undefined;
			if (ctx) {
				this.ui.setContextIndicator(ctx.current_note, ctx.selected_text);
			} else {
				this.ui.setContextIndicator();
			}
			await this.api.sendMessage(text, ctx);
		} catch (err) {
			this.ui.showError(`Failed to send: ${err}`);
		}
	}

	private handleSSE(evt: SSEEvent): void {
		switch (evt.type) {
			case "message_chunk":
				this.currentAssistantMessage += (evt.data.text as string) || "";
				this.ui.updateAssistantMessage(this.currentAssistantMessage);
				break;
			case "tool_call": {
				const id = evt.data.id as string;
				this.currentToolCalls[id] = {
					name: evt.data.name as string,
					input: evt.data.input as Record<string, unknown>,
				};
				this.ui.addToolCall(id, this.currentToolCalls[id]);
				break;
			}
			case "tool_result": {
				const id = evt.data.id as string;
				if (this.currentToolCalls[id]) {
					this.currentToolCalls[id].result = evt.data.result as string;
					this.ui.updateToolCallResult(id, this.currentToolCalls[id].result ?? "");
				}
				break;
			}
			case "done": {
				const assistantMsg: ChatMessage = {
					id: crypto.randomUUID(),
					role: "assistant",
					content: this.currentAssistantMessage,
					toolCalls: Object.entries(this.currentToolCalls).map(([id, tc]) => ({
						id,
						name: tc.name,
						input: tc.input,
						result: tc.result,
					})),
				};
				this.messages.push(assistantMsg);
				this.ui.finalizeAssistantMessage();
				if (this.settings.autoSave) {
					this.saveConversation().catch(() => { /* ignore auto-save errors */ });
				}
				break;
			}
			case "error":
				this.ui.showError(evt.data.message as string);
				break;
		}
	}

	async saveConversation(): Promise<void> {
		if (this.messages.length === 0) return;
		const folder = this.settings.saveFolder || "Hermind Conversations";
		await this.app.vault.createFolder(folder).catch(() => { /* may exist */ });
		const date = new Date().toISOString().replace(/[:T]/g, "-").slice(0, 19);
		const firstUserMsg = this.messages.find((m) => m.role === "user")?.content.slice(0, 30) || "conversation";
		const safeName = firstUserMsg.replace(/[^a-z0-9\u4e00-\u9fa5]/gi, " ").trim().replace(/\s+/g, "-");
		const fileName = `${folder}/${date}-${safeName}.md`;

		const lines: string[] = [
			"---",
			`title: "Hermind Conversation"`,
			`date: ${new Date().toISOString()}`,
			`tags: [hermind, conversation]`,
			`message_count: ${this.messages.length}`,
			"---",
			"",
		];

		for (const msg of this.messages) {
			lines.push(`## ${msg.role === "user" ? "User" : "Assistant"}`);
			lines.push("");
			lines.push(msg.content);
			lines.push("");
		}

		await this.app.vault.create(fileName, lines.join("\n"));
	}
}
