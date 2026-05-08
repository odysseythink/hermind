import { useTranslation } from 'react-i18next';
import { useTheme, type Theme } from '../../hooks/useTheme';
import styles from './ThemeToggle.module.css';

const OPTIONS: { key: Theme; label: string }[] = [
  { key: 'light', label: '☀' },
  { key: 'auto', label: 'A' },
  { key: 'dark', label: '☾' },
];

export default function ThemeToggle() {
  const { t } = useTranslation('ui');
  const { theme, setTheme } = useTheme();

  return (
    <div className={styles.toggle} role="group" aria-label={t('theme.switcher')}>
      {OPTIONS.map((opt) => (
        <button
          key={opt.key}
          type="button"
          className={theme === opt.key ? styles.active : ''}
          onClick={() => setTheme(opt.key)}
          aria-pressed={theme === opt.key}
          title={t(`theme.${opt.key}`)}
        >
          {opt.label}
        </button>
      ))}
    </div>
  );
}
