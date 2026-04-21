import { useState } from 'react';
import type { ToolCallSnapshot } from '../../state/chat';

type Props = { call: ToolCallSnapshot };

export default function ToolCallCard({ call }: Props) {
  const [open, setOpen] = useState(false);
  return (
    <div
      style={{
        border: '1px solid var(--border, #30363d)',
        borderRadius: 4,
        padding: '0.25rem 0.5rem',
        margin: '0.25rem 0',
        fontSize: '0.9em',
      }}
    >
      <button type="button" onClick={() => setOpen((o) => !o)} style={{ background: 'transparent', border: 'none', cursor: 'pointer' }}>
        {open ? '▼' : '▶'} tool: {call.name} ({call.state})
      </button>
      {open && (
        <div style={{ marginTop: '0.25rem' }}>
          <div>
            <strong>input:</strong>{' '}
            <code>{JSON.stringify(call.input)}</code>
          </div>
          {call.result && (
            <div>
              <strong>result:</strong>
              <pre style={{ whiteSpace: 'pre-wrap', margin: 0 }}>{call.result}</pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
