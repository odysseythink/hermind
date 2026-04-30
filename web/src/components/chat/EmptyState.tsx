import styles from './EmptyState.module.css';

interface Props {
  suggestions: string[];
  onSuggestionClick: (text: string) => void;
}

export default function EmptyState({ suggestions, onSuggestionClick }: Props) {
  return (
    <div className={styles.emptyState}>
      <div className={styles.greeting}>
        <h1 className={styles.title}>How can I help you today?</h1>
      </div>
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
    </div>
  );
}
