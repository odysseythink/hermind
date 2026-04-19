import styles from './GatewayPanel.module.css';
import type { PlatformInstance, SchemaDescriptor } from '../../../api/schemas';
import Editor from '../../Editor';
import GatewayApplyButton from './GatewayApplyButton';

export interface GatewayPanelProps {
  selectedKey: string | null;
  instance: PlatformInstance | null;
  originalInstance: PlatformInstance | null;
  descriptor: SchemaDescriptor | null;
  dirty: boolean;
  busy: boolean;
  onField: (field: string, value: string) => void;
  onToggleEnabled: (enabled: boolean) => void;
  onDelete: () => void;
  onApply: () => void;
}

export default function GatewayPanel({
  selectedKey,
  instance,
  originalInstance,
  descriptor,
  dirty,
  busy,
  onField,
  onToggleEnabled,
  onDelete,
  onApply,
}: GatewayPanelProps) {
  return (
    <section className={styles.panel} aria-label="Gateway configuration">
      <div className={styles.header}>
        <div className={styles.crumbs}>
          <strong>Gateway</strong>
          {selectedKey && <> · {selectedKey}</>}
        </div>
        <GatewayApplyButton dirty={dirty} busy={busy} onApply={onApply} />
      </div>
      <div className={styles.body}>
        <Editor
          selectedKey={selectedKey}
          instance={instance}
          originalInstance={originalInstance}
          descriptor={descriptor}
          onField={onField}
          onToggleEnabled={onToggleEnabled}
          onDelete={onDelete}
        />
      </div>
    </section>
  );
}
