import styles from './ModelPicker.module.css';

interface Props {
  modelName: string;
}

export default function ModelPicker({ modelName }: Props) {
  return (
    <div className={styles.picker}>
      <span className={styles.modelName}>{modelName || '—'}</span>
    </div>
  );
}
