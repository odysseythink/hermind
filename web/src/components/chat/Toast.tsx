import styles from './Toast.module.css';

type Props = { message: string; onDismiss: () => void };

export default function Toast({ message, onDismiss }: Props) {
  return (
    <div role="alert" className={styles.toast}>
      <span>{message}</span>
      <button
        type="button"
        className={styles.dismiss}
        onClick={onDismiss}
        aria-label="dismiss"
      >
        ×
      </button>
    </div>
  );
}
