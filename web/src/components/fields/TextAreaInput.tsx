import styles from './TextAreaInput.module.css';

type Props = {
  value: string;
  onChange: (next: string) => void;
  placeholder?: string;
  disabled?: boolean;
  rows?: number;
  'aria-label'?: string;
};

export default function TextAreaInput({
  value, onChange, placeholder, disabled, rows = 6, ...rest
}: Props) {
  return (
    <textarea
      className={styles.textarea}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      placeholder={placeholder}
      disabled={disabled}
      rows={rows}
      aria-label={rest['aria-label']}
    />
  );
}
