import styles from './StreamingCursor.module.css';

export default function StreamingCursor() {
  return <span aria-hidden="true" className={styles.cursor} />;
}
