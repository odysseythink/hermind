import ConfigSection from '../ConfigSection';
import type { ConfigSection as ConfigSectionT } from '../../api/schemas';

export interface ListElementInlineEditorProps {
  section: ConfigSectionT;
  index: number;
  length: number;
  value: Record<string, unknown>;
  originalValue: Record<string, unknown>;
  dirty: boolean;
  config: Record<string, unknown>;
  onField: (index: number, field: string, value: unknown) => void;
  onDelete: () => void;
  onMoveUp: () => void;
  onMoveDown: () => void;
}

export default function ListElementInlineEditor(props: ListElementInlineEditorProps) {
  const atTop = props.index === 0;
  const atBottom = props.index === props.length - 1;

  function onDeleteClick() {
    if (window.confirm(`Delete #${props.index + 1}? This cannot be undone.`)) {
      props.onDelete();
    }
  }

  return (
    <div>
      <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '0.5rem', alignItems: 'center' }}>
        <span>#{props.index + 1} of {props.length}</span>
        {props.dirty && <span title="Unsaved changes">●</span>}
        <div style={{ marginLeft: 'auto', display: 'flex', gap: '0.25rem' }}>
          <button type="button" onClick={props.onMoveUp} disabled={atTop} aria-label="Move up">↑</button>
          <button type="button" onClick={props.onMoveDown} disabled={atBottom} aria-label="Move down">↓</button>
          <button type="button" onClick={onDeleteClick} aria-label="Delete">Delete</button>
        </div>
      </div>
      <ConfigSection
        section={props.section}
        value={props.value}
        originalValue={props.originalValue}
        onFieldChange={(field, v) => props.onField(props.index, field, v)}
        config={props.config}
      />
    </div>
  );
}
