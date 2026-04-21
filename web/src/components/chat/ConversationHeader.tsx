import ModelSelector from './ModelSelector';

type Props = {
  title: string;
  model: string;
  modelOptions: string[];
  onModelChange: (v: string) => void;
};

export default function ConversationHeader({ title, model, modelOptions, onModelChange }: Props) {
  return (
    <header
      style={{
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
        padding: '0.5rem 1rem',
        borderBottom: '1px solid var(--border, #30363d)',
      }}
    >
      <h2 style={{ margin: 0, fontSize: '1rem' }}>{title}</h2>
      <ModelSelector value={model} options={modelOptions} onChange={onModelChange} />
    </header>
  );
}
