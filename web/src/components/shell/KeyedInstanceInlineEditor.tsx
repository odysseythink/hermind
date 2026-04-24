import ConfigSection from '../ConfigSection';
import type { ConfigSection as ConfigSectionT } from '../../api/schemas';

export interface KeyedInstanceInlineEditorProps {
  section: ConfigSectionT;
  instanceKey: string;
  value: Record<string, unknown>;
  originalValue: Record<string, unknown>;
  dirty: boolean;
  config: Record<string, unknown>;
  onField: (instanceKey: string, field: string, value: unknown) => void;
  onDelete: () => void;
}

export default function KeyedInstanceInlineEditor(props: KeyedInstanceInlineEditorProps) {
  function onDeleteClick() {
    if (window.confirm(`Delete "${props.instanceKey}"? This cannot be undone.`)) {
      props.onDelete();
    }
  }

  return (
    <div>
      <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '0.5rem', alignItems: 'center' }}>
        <strong>{props.instanceKey}</strong>
        {props.dirty && <span title="Unsaved changes">●</span>}
        <div style={{ marginLeft: 'auto' }}>
          <button type="button" onClick={onDeleteClick} aria-label="Delete">Delete</button>
        </div>
      </div>
      <ConfigSection
        section={props.section}
        value={props.value}
        originalValue={props.originalValue}
        onFieldChange={(field, v) => props.onField(props.instanceKey, field, v)}
        config={props.config}
        hideSectionMeta
      />
    </div>
  );
}
