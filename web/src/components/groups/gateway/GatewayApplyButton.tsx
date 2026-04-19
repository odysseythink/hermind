import styles from './GatewayApplyButton.module.css';

export interface GatewayApplyButtonProps {
  dirty: boolean;
  busy: boolean;
  onApply: () => void;
}

export default function GatewayApplyButton({ dirty, busy, onApply }: GatewayApplyButtonProps) {
  const title = dirty
    ? 'Save first, then apply'
    : busy
      ? 'Busy…'
      : 'Restart gateway with current config';
  return (
    <button
      type="button"
      className={styles.btn}
      onClick={onApply}
      disabled={dirty || busy}
      title={title}
    >
      Apply
    </button>
  );
}
