import styles from './ModelSelector.module.css';

type Props = {
  value: string;
  options: string[];
  onChange: (v: string) => void;
};

export default function ModelSelector({ value, options, onChange }: Props) {
  return (
    <select className={styles.select} value={value} onChange={(e) => onChange(e.target.value)}>
      {options.map((o) => (
        <option key={o || '(default)'} value={o}>
          {o || '(default)'}
        </option>
      ))}
    </select>
  );
}
