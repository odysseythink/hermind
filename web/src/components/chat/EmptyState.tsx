import { useTranslation } from 'react-i18next';
import styles from './EmptyState.module.css';

interface Props {
  suggestions: string[];
  onSuggestionClick: (text: string) => void;
}

export default function EmptyState({ suggestions, onSuggestionClick }: Props) {
  const { t } = useTranslation('ui');
  return (
    <>
      <h1 className={styles.greeting}>{t('chat.greeting')}</h1>
      {suggestions.length > 0 && (
        <div className={styles.suggestions}>
          {suggestions.map((text) => (
            <button
              key={text}
              className={styles.suggestionBtn}
              onClick={() => onSuggestionClick(text)}
            >
              {text}
            </button>
          ))}
        </div>
      )}
    </>
  );
}
