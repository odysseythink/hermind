import ModelSelector from './ModelSelector';
import styles from './ConversationHeader.module.css';

type Props = {
  title: string;
  model: string;
  modelOptions: string[];
  onModelChange: (v: string) => void;
};

export default function ConversationHeader({ title, model, modelOptions, onModelChange }: Props) {
  return (
    <header className={styles.header}>
      <h2 className={styles.title}>{title}</h2>
      <ModelSelector value={model} options={modelOptions} onChange={onModelChange} />
    </header>
  );
}
