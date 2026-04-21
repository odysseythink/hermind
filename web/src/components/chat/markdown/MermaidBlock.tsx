import { useEffect, useRef } from 'react';

type MermaidModule = { initialize(cfg: Record<string, unknown>): void; render(id: string, chart: string): Promise<{ svg: string }> };

let mermaidPromise: Promise<MermaidModule> | null = null;
function loadMermaid(): Promise<MermaidModule> {
  if (!mermaidPromise) {
    mermaidPromise = import('mermaid').then((m) => {
      const mm = (m.default ?? m) as unknown as MermaidModule;
      mm.initialize({ startOnLoad: false, theme: 'default' });
      return mm;
    });
  }
  return mermaidPromise;
}

type Props = { chart: string };

export default function MermaidBlock({ chart }: Props) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    let cancelled = false;
    loadMermaid().then((m) => {
      if (cancelled || !ref.current) return;
      const id = `m-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`;
      m.render(id, chart).then(({ svg }) => {
        if (!cancelled && ref.current) ref.current.innerHTML = svg;
      }).catch(() => {});
    });
    return () => { cancelled = true; };
  }, [chart]);
  return <div ref={ref} data-mermaid-container />;
}
