import { useState } from 'react';
import styles from './TestConnection.module.css';
import { apiFetch, ApiError } from '../api/client';
import { PlatformTestResponseSchema } from '../api/schemas';

export interface TestConnectionProps {
  instanceKey: string;
  dirty: boolean;
}

type Result =
  | { kind: 'idle' }
  | { kind: 'busy' }
  | { kind: 'ok' }
  | { kind: 'err'; msg: string }
  | { kind: 'warn'; msg: string };

export default function TestConnection({ instanceKey, dirty }: TestConnectionProps) {
  const [result, setResult] = useState<Result>({ kind: 'idle' });

  async function runProbe() {
    setResult({ kind: 'busy' });
    try {
      const res = await apiFetch(
        `/api/platforms/${encodeURIComponent(instanceKey)}/test`,
        { method: 'POST', schema: PlatformTestResponseSchema },
      );
      if (res.ok) {
        setResult({ kind: 'ok' });
      } else {
        setResult({ kind: 'err', msg: res.error ?? 'probe failed' });
      }
    } catch (e) {
      if (e instanceof ApiError && e.status === 501) {
        setResult({
          kind: 'warn',
          msg: 'no probe for this platform type',
        });
        return;
      }
      setResult({ kind: 'err', msg: toMsg(e) });
    }
  }

  return (
    <div className={styles.wrap}>
      <button
        type="button"
        className={styles.btn}
        onClick={runProbe}
        disabled={result.kind === 'busy' || dirty}
        title={dirty ? 'Save changes first — probe uses on-disk config' : undefined}
      >
        {result.kind === 'busy' ? 'Testing…' : 'Test connection'}
      </button>
      {renderResult(result)}
    </div>
  );
}

function renderResult(r: Result) {
  if (r.kind === 'idle' || r.kind === 'busy') return null;
  if (r.kind === 'ok') {
    return (
      <span className={`${styles.result} ${styles.resultOk}`}>
        <span className={`${styles.dot} ${styles.dotOk}`} />
        connected
      </span>
    );
  }
  if (r.kind === 'warn') {
    return (
      <span className={`${styles.result} ${styles.resultWarn}`}>
        <span className={`${styles.dot} ${styles.dotWarn}`} />
        {r.msg}
      </span>
    );
  }
  return (
    <span className={`${styles.result} ${styles.resultErr}`}>
      <span className={`${styles.dot} ${styles.dotErr}`} />
      {r.msg}
    </span>
  );
}

function toMsg(e: unknown): string {
  if (e instanceof ApiError) {
    if (typeof e.body === 'object' && e.body !== null && 'error' in e.body) {
      const m = (e.body as { error?: unknown }).error;
      if (typeof m === 'string') return m;
    }
    return `HTTP ${e.status}`;
  }
  return e instanceof Error ? e.message : String(e);
}
