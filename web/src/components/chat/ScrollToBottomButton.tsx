import styles from './ScrollToBottomButton.module.css';

interface Props {
  onClick: () => void;
}

export default function ScrollToBottomButton({ onClick }: Props) {
  return (
    <button className={styles.button} onClick={onClick} aria-label="Scroll to bottom">
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
        <path d="M6 9l6 6 6-6" />
      </svg>
    </button>
  );
}
