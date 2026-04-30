/// <reference lib="dom" />
type Handler = (ev: MessageEvent) => void;

export class FakeEventSource {
  static instances: FakeEventSource[] = [];
  readonly url: string;
  readyState = 0;
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;
  onmessage: Handler | null = null;
  private listeners: Record<string, Set<Handler | (() => void)>> = {};

  constructor(url: string) {
    this.url = url;
    FakeEventSource.instances.push(this);
    queueMicrotask(() => {
      this.readyState = 1;
      this.onopen?.();
      this.listeners['open']?.forEach((h) => (h as () => void)());
    });
  }

  addEventListener(event: string, h: Handler | (() => void)) {
    if (!this.listeners[event]) this.listeners[event] = new Set();
    this.listeners[event].add(h);
    if (event === 'message') this.onmessage = h as Handler;
    if (event === 'open') this.onopen = h as () => void;
    if (event === 'error') this.onerror = h as () => void;
  }

  removeEventListener(event: string, h: Handler | (() => void)) {
    this.listeners[event]?.delete(h);
  }

  dispatchMessage(data: unknown) {
    const ev = new MessageEvent('message', { data: JSON.stringify(data) });
    this.onmessage?.(ev);
    this.listeners['message']?.forEach((h) => (h as Handler)(ev));
  }

  close() {
    this.readyState = 2;
  }

  static install() {
     
    (globalThis as any).EventSource = FakeEventSource;
  }

  static reset() {
    FakeEventSource.instances = [];
  }
}
