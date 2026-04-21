import { useEffect, useState } from 'react';
import styles from './SlashMenu.module.css';

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
    <ul className={styles.menu}>
      {commands.map((c, i) => (
        <li
          key={c.id}
          className={`${styles.item} ${i === idx ? styles.active : ''}`}
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
