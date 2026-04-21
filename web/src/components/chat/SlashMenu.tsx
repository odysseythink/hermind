import { useEffect, useState } from 'react';

export type SlashCommand = {
  id: string;
  label: string;
  run: () => void;
};

type Props = { commands: SlashCommand[]; onClose: () => void };

export default function SlashMenu({ commands, onClose }: Props) {
  const [idx, setIdx] = useState(0);
  useEffect(() => {
    const on = (e: KeyboardEvent) => {
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        setIdx((i) => Math.min(i + 1, commands.length - 1));
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        setIdx((i) => Math.max(i - 1, 0));
      } else if (e.key === 'Enter') {
        e.preventDefault();
        commands[idx]?.run();
        onClose();
      } else if (e.key === 'Escape') {
        e.preventDefault();
        onClose();
      }
    };
    window.addEventListener('keydown', on, true);
    return () => window.removeEventListener('keydown', on, true);
  }, [commands, idx, onClose]);

  return (
    <ul
      style={{
        position: 'absolute',
        bottom: '100%',
        left: 0,
        listStyle: 'none',
        padding: 0,
        margin: 0,
        background: 'var(--bg-topbar, #161b22)',
        border: '1px solid var(--border, #30363d)',
        borderRadius: 4,
        minWidth: '12rem',
      }}
    >
      {commands.map((c, i) => (
        <li
          key={c.id}
          style={{
            background: i === idx ? 'var(--accent, #1f6feb)' : 'transparent',
            color: i === idx ? '#fff' : 'inherit',
            padding: '0.25rem 0.5rem',
            cursor: 'pointer',
          }}
          onMouseDown={(e) => {
            e.preventDefault();
            c.run();
            onClose();
          }}
        >
          /{c.label}
        </li>
      ))}
    </ul>
  );
}
