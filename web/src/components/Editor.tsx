import styles from './Editor.module.css';
import type { PlatformInstance, SchemaDescriptor } from '../api/schemas';

export interface EditorProps {
  selectedKey: string | null;
  instance: PlatformInstance | null;
  descriptor: SchemaDescriptor | null;
}

export default function Editor({ selectedKey, instance, descriptor }: EditorProps) {
  if (!selectedKey || !instance) {
    return (
      <div className={styles.wrapper}>
        <div className={styles.emptyCard}>
          <h2 className={styles.emptyTitle}>No instance selected</h2>
          <p className={styles.emptyBody}>
            Pick an instance from the sidebar, or click <em>+ New instance</em>
            to create one. Field editors land in Stage 4.
          </p>
        </div>
      </div>
    );
  }
  if (!descriptor) {
    return (
      <div className={styles.wrapper}>
        <div className={styles.emptyCard}>
          <h2 className={styles.emptyTitle}>Unknown platform type</h2>
          <p className={styles.emptyBody}>
            {selectedKey} is configured as type <code>{instance.type}</code>,
            which has no registered descriptor. Update the YAML directly or
            delete this instance.
          </p>
        </div>
      </div>
    );
  }
  return (
    <div className={styles.wrapper}>
      <section className={styles.panel}>
        <header className={styles.panelHeader}>
          <h2 className={styles.title}>{selectedKey}</h2>
          <span className={styles.typeTag}>{descriptor.display_name}</span>
        </header>
        {descriptor.summary && (
          <p className={styles.summary}>{descriptor.summary}</p>
        )}
        <div className={styles.stagePlaceholder}>
          Field editors land in Stage 4 —
          this descriptor has {descriptor.fields.length} field
          {descriptor.fields.length === 1 ? '' : 's'} to render.
        </div>
      </section>
    </div>
  );
}
