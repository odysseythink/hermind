import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { apiFetch } from '../../api/client';
import { ToolsResponseSchema, SkillsResponseSchema } from '../../api/schemas';
import styles from './ToolsMenu.module.css';

interface Props {
  visible: boolean;
  onClose: () => void;
  onSelect: (text: string, sendNow?: boolean) => void;
  suggestions: string[];
}

export default function ToolsMenu({ visible, onClose, onSelect, suggestions }: Props) {
  const { t } = useTranslation('ui');
  const menuRef = useRef<HTMLDivElement>(null);
  const [tools, setTools] = useState<{ name: string; description?: string }[]>([]);
  const [skills, setSkills] = useState<{ name: string; description?: string; enabled: boolean }[]>([]);

  useEffect(() => {
    if (!visible) return;
    Promise.all([
      apiFetch('/api/tools', { schema: ToolsResponseSchema }).catch(() => ({ tools: [] })),
      apiFetch('/api/skills', { schema: SkillsResponseSchema }).catch(() => ({ skills: [] })),
    ]).then(([toolsRes, skillsRes]) => {
      setTools(toolsRes.tools);
      setSkills(skillsRes.skills.filter((s) => s.enabled));
    });
  }, [visible]);

  useEffect(() => {
    if (!visible) return;
    const handleClick = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        onClose();
      }
    };
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [visible, onClose]);

  if (!visible) return null;

  return (
    <div className={styles.menu} ref={menuRef}>
      <div className={styles.section}>
        <div className={styles.sectionHeader}>MCP</div>
        {tools.length === 0 ? (
          <div className={styles.empty}>{t('chat.noTools')}</div>
        ) : (
          tools.map((tool) => (
            <button
              key={tool.name}
              className={styles.item}
              onClick={() => {
                onSelect(tool.name);
                onClose();
              }}
            >
              <span className={styles.itemName}>{tool.name}</span>
              {tool.description && <span className={styles.itemDesc}>{tool.description}</span>}
            </button>
          ))
        )}
      </div>

      {suggestions.length > 0 && (
        <div className={styles.section}>
          <div className={styles.sectionHeader}>Commands</div>
          {suggestions.map((s) => (
            <button
              key={s}
              className={styles.item}
              onClick={() => {
                onSelect(s, true);
                onClose();
              }}
            >
              <span className={styles.itemName}>{s}</span>
            </button>
          ))}
        </div>
      )}

      {skills.length > 0 && (
        <div className={styles.section}>
          <div className={styles.sectionHeader}>Skills</div>
          {skills.map((s) => (
            <button
              key={s.name}
              className={styles.item}
              onClick={() => {
                onSelect(`/${s.name}`);
                onClose();
              }}
            >
              <span className={styles.itemName}>{s.name}</span>
              {s.description && <span className={styles.itemDesc}>{s.description}</span>}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
