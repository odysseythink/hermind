import { useTranslation } from 'react-i18next';
import { setLanguage } from '../../i18n';
import styles from './LanguageToggle.module.css';

export default function LanguageToggle() {
  const { i18n, t } = useTranslation('ui');
  const isZh = i18n.language.toLowerCase().startsWith('zh');

  return (
    <div className={styles.toggle} role="group" aria-label={t('language.switcher')}>
      <button
        type="button"
        className={isZh ? styles.active : ''}
        onClick={() => setLanguage('zh-CN')}
        aria-pressed={isZh}
      >
        中
      </button>
      <button
        type="button"
        className={!isZh ? styles.active : ''}
        onClick={() => setLanguage('en')}
        aria-pressed={!isZh}
      >
        EN
      </button>
    </div>
  );
}
