import styles from './Footer.module.css';
import type { Flash } from '../state';

export interface FooterProps {
  flash: Flash | null;
}

export default function Footer({ flash }: FooterProps) {
  if (!flash) {
    return <footer className={styles.footer} />;
  }
  const cls = flash.kind === 'err' ? styles.flashErr : styles.flashOk;
  return (
    <footer className={styles.footer}>
      <span className={cls}>{flash.msg}</span>
    </footer>
  );
}
