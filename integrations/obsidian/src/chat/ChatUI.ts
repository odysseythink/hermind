import { ChatMessage, ToolCallEvent } from "./types";

interface ChatUIOptions {
	onSend: (text: string) => void;
	onSave: () => void;
	showToolCalls: boolean;
}

export class ChatUI {
	private container: HTMLElement;
	private messagesEl: HTMLElement;
	private inputEl: HTMLTextAreaElement;
	private currentAssistantEl: HTMLElement | null = null;
	private errorEl: HTMLElement;
	private opts: ChatUIOptions;

	constructor(parent: HTMLElement, opts: ChatUIOptions) {
		this.opts = opts;
		this.container = parent.createDiv({ cls: "hermind-chat-container" });
		this.container.style.display = "flex";
		this.container.style.flexDirection = "column";
		this.container.style.height = "100%";

		this.errorEl = this.container.createDiv({ cls: "hermind-error" });
		this.errorEl.style.display = "none";
		this.errorEl.style.color = "var(--text-error)";
		this.errorEl.style.padding = "8px";
		this.errorEl.style.fontSize = "12px";

		this.messagesEl = this.container.createDiv({ cls: "hermind-messages" });
		this.messagesEl.style.flex = "1";
		this.messagesEl.style.overflowY = "auto";
		this.messagesEl.style.padding = "8px";

		const inputContainer = this.container.createDiv({ cls: "hermind-input-container" });
		inputContainer.style.padding = "8px";
		inputContainer.style.borderTop = "1px solid var(--background-modifier-border)";
		inputContainer.style.display = "flex";
		inputContainer.style.gap = "8px";

		this.inputEl = inputContainer.createEl("textarea");
		this.inputEl.style.flex = "1";
		this.inputEl.style.resize = "none";
		this.inputEl.style.height = "60px";
		this.inputEl.placeholder = "Ask hermind...";
		this.inputEl.addEventListener("keydown", (e) => {
			if (e.key === "Enter" && !e.shiftKey) {
				e.preventDefault();
				this.submit();
			}
		});

		const sendBtn = inputContainer.createEl("button", { text: "Send" });
		sendBtn.onclick = () => this.submit();

		const saveBtn = inputContainer.createEl("button", { text: "Save" });
		saveBtn.onclick = () => this.opts.onSave();
	}

	addMessage(msg: ChatMessage): void {
		const el = this.messagesEl.createDiv({ cls: `hermind-message hermind-message-${msg.role}` });
		el.style.marginBottom = "12px";
		el.style.padding = "8px";
		el.style.borderRadius = "6px";
		el.style.backgroundColor = msg.role === "user" ? "var(--background-modifier-form-field)" : "var(--background-primary-alt)";
		el.createEl("div", { text: msg.content });

		if (msg.toolCalls && this.opts.showToolCalls) {
			for (const tc of msg.toolCalls) {
				this.renderToolCall(el, tc);
			}
		}
	}

	startAssistantMessage(): void {
		this.currentAssistantEl = this.messagesEl.createDiv({ cls: "hermind-message hermind-message-assistant" });
		this.currentAssistantEl.style.marginBottom = "12px";
		this.currentAssistantEl.style.padding = "8px";
		this.currentAssistantEl.style.borderRadius = "6px";
		this.currentAssistantEl.style.backgroundColor = "var(--background-primary-alt)";
	}

	updateAssistantMessage(text: string): void {
		if (!this.currentAssistantEl) return;
		this.currentAssistantEl.empty();
		this.currentAssistantEl.createEl("div", { text });
	}

	finalizeAssistantMessage(): void {
		this.currentAssistantEl = null;
	}

	addToolCall(id: string, tc: { name: string; input: Record<string, unknown> }): void {
		if (!this.currentAssistantEl || !this.opts.showToolCalls) return;
		const el = this.currentAssistantEl.createDiv({ cls: "hermind-tool-call" });
		el.style.fontSize = "11px";
		el.style.color = "var(--text-muted)";
		el.createEl("div", { text: `🔧 ${tc.name}` });
	}

	updateToolCallResult(id: string, result: string): void {
		// No-op for now
	}

	showError(msg: string): void {
		this.errorEl.style.display = "block";
		this.errorEl.setText(msg);
		setTimeout(() => {
			this.errorEl.style.display = "none";
		}, 5000);
	}

	private submit(): void {
		const text = this.inputEl.value.trim();
		if (!text) return;
		this.inputEl.value = "";
		this.opts.onSend(text);
	}
}
