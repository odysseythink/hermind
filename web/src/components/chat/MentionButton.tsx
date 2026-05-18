import styles from './MentionButton.module.css';

interface Props {
  onClick: () => void;
  disabled?: boolean;
}

export default function MentionButton({ onClick, disabled }: Props) {
  return (
    <button
      type="button"
      className={styles.btn}
      onClick={onClick}
      disabled={disabled}
      aria-label="Mention"
    >
      @
    </button>
  );
}
