import { SSEEvent } from "./chat/types";

export class HermindSSE {
	private eventSource: EventSource | null = null;
	private reconnectAttempts = 0;
	private maxReconnects = 3;

	constructor(
		private baseUrl: string,
		private onEvent: (event: SSEEvent) => void,
		private onError: (msg: string) => void
	) {}

	connect(): void {
		this.disconnect();
		this.eventSource = new EventSource(`${this.baseUrl}/api/sse`);

		this.eventSource.onmessage = (evt) => {
			try {
				const parsed = JSON.parse(evt.data) as SSEEvent;
				this.onEvent(parsed);
				if (parsed.type === "done" || parsed.type === "error") {
					this.reconnectAttempts = 0;
				}
			} catch {
				// ignore malformed events
			}
		};

		this.eventSource.onerror = () => {
			this.onError("SSE connection lost");
			this.tryReconnect();
		};
	}

	disconnect(): void {
		if (this.eventSource) {
			this.eventSource.close();
			this.eventSource = null;
		}
	}

	private tryReconnect(): void {
		if (this.reconnectAttempts >= this.maxReconnects) {
			this.onError("SSE reconnection failed after 3 attempts");
			return;
		}
		this.reconnectAttempts++;
		setTimeout(() => this.connect(), 2000);
	}
}
