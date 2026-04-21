import { useEffect, useState } from 'react';
import { codeToHtml } from 'shiki';
import MermaidBlock from './MermaidBlock';

type Props = {
  language: string;
  code: string;
};

export default function CodeBlock({ language, code }: Props) {
  const [html, setHtml] = useState<string | null>(null);

  useEffect(() => {
    if (language === 'mermaid') return;
    let cancelled = false;
    codeToHtml(code, { lang: language || 'text', theme: 'github-dark' })
      .then((h) => { if (!cancelled) setHtml(h); })
      .catch(() => { if (!cancelled) setHtml(null); });
    return () => { cancelled = true; };
  }, [language, code]);

  if (language === 'mermaid') {
    return <MermaidBlock chart={code} />;
  }
  if (html) return <div dangerouslySetInnerHTML={{ __html: html }} />;
  return (
    <pre>
      <code>{code}</code>
    </pre>
  );
}
