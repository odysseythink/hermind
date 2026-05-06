import { requestUrl } from "obsidian";

export interface ObsidianContext {
	vault_path: string;
	current_note?: string;
	selected_text?: string;
	cursor_line?: number;
}

export class HermindAPI {
	constructor(private baseUrl: string) {}

	async sendMessage(message: string, ctx?: ObsidianContext): Promise<void> {
		const body: Record<string, unknown> = { user_message: message };
		if (ctx) {
			body.obsidian_context = ctx;
		}
		await requestUrl({
			url: `${this.baseUrl}/api/conversation/messages`,
			method: "POST",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify(body),
		});
	}

	async cancel(): Promise<void> {
		await requestUrl({
			url: `${this.baseUrl}/api/conversation/cancel`,
			method: "POST",
		});
	}
}
