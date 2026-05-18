import styles from './ToolsButton.module.css';

interface Props {
  onClick: () => void;
  active?: boolean;
  disabled?: boolean;
}

export default function ToolsButton({ onClick, active, disabled }: Props) {
  return (
    <button
      type="button"
      className={`${styles.btn} ${active ? styles.active : ''}`}
      onClick={onClick}
      disabled={disabled}
      aria-label="Tools"
    >
      工具
    </button>
  );
}
