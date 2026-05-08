import { useEffect, useState, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import styles from './SkillsSection.module.css';
import Switch from '../../fields/Switch';
import { apiFetch, ApiError } from '../../../api/client';
import { SkillsResponseSchema, type ConfigSection as ConfigSectionT } from '../../../api/schemas';
import { useDescriptorT } from '../../../i18n/useDescriptorT';

export interface SkillsSectionProps {
  section: ConfigSectionT;
  value: Record<string, unknown>;
  originalValue: Record<string, unknown>;
  onField: (field: string, value: unknown) => void;
  config?: Record<string, unknown>;
}

type FetchState =
  | { status: 'loading' }
  | { status: 'ok'; skills: { name: string; description: string; enabled: boolean }[] }
  | { status: 'error'; message: string };

type SkillRow = {
  name: string;
  description: string;
  missing: boolean;
  enabled: boolean;
};

function asBool(v: unknown): boolean {
  if (typeof v === 'boolean') return v;
  if (typeof v === 'string') return v === 'true';
  return false;
}

function asString(v: unknown): string {
  if (v === undefined || v === null) return '';
  return typeof v === 'string' ? v : String(v);
}

// parseIntField coerces an <input type="number"> string back to a number
// for dispatch, since the backend YAML loader rejects string-encoded ints.
// Empty / non-finite input falls back to 0.
function parseIntField(raw: string): number {
  if (raw === '') return 0;
  const n = Number(raw);
  return Number.isFinite(n) ? n : 0;
}

export default function SkillsSection(props: SkillsSectionProps) {
  const { t } = useTranslation('ui');
  const dt = useDescriptorT();
  const [fetchState, setFetchState] = useState<FetchState>({ status: 'loading' });

  const load = useCallback((signal?: AbortSignal) => {
    setFetchState({ status: 'loading' });
    apiFetch('/api/skills', { schema: SkillsResponseSchema, signal })
      .then(resp => {
        const normalized = resp.skills.map(s => ({
          name: s.name,
          description: s.description || '',
          enabled: s.enabled,
        }));
        setFetchState({ status: 'ok', skills: normalized });
      })
      .catch(err => {
        if (signal?.aborted) return;
        const msg = err instanceof ApiError ? `${err.status}` : err instanceof Error ? err.message : String(err);
        setFetchState({ status: 'error', message: msg });
      });
  }, []);

  useEffect(() => {
    const ac = new AbortController();
    load(ac.signal);
    return () => ac.abort();
  }, [load]);

  const disabled = (props.value.disabled as string[] | undefined) ?? [];
  const disabledSet = new Set(disabled);

  const rows: SkillRow[] = [];
  if (fetchState.status === 'ok') {
    const seen = new Set<string>();
    for (const s of fetchState.skills) {
      rows.push({
        name: s.name,
        description: s.description,
        missing: false,
        enabled: !disabledSet.has(s.name),
      });
      seen.add(s.name);
    }
    for (const name of disabled) {
      if (!seen.has(name)) {
        rows.push({ name, description: '', missing: true, enabled: false });
      }
    }
    rows.sort((a, b) => a.name.localeCompare(b.name));
  }

  const total = fetchState.status === 'ok' ? rows.length : 0;
  const disabledCount = rows.filter(r => !r.enabled).length;

  function toggleSkill(name: string, nextEnabled: boolean) {
    const cur = (props.value.disabled as string[] | undefined) ?? [];
    const next = nextEnabled
      ? cur.filter(n => n !== name)
      : [...cur.filter(n => n !== name), name].sort();
    props.onField('disabled', next);
  }

  const sectionLabel = dt.sectionLabel(props.section.key, props.section.label);
  const sectionSummary = props.section.summary
    ? dt.sectionSummary(props.section.key, props.section.summary)
    : '';

  return (
    <section className={styles.section} aria-label={sectionLabel}>
      <header className={styles.header}>
        <h2>{sectionLabel}</h2>
        {sectionSummary && <p>{sectionSummary}</p>}
      </header>

      <div className={styles.panel}>
        <h3 className={styles.panelTitle}>{t('skills.globals')}</h3>

        <div className={styles.globalsRow}>
          <div>
            <label htmlFor="skills-auto-extract" className={styles.label}>
              {dt.fieldLabel(props.section.key, 'auto_extract', 'Auto-extract')}
            </label>
            <div className={styles.help}>
              {dt.fieldHelp(props.section.key, 'auto_extract', '')}
            </div>
          </div>
          <Switch
            checked={asBool(props.value.auto_extract)}
            onChange={(next) => props.onField('auto_extract', next)}
            ariaLabel={dt.fieldLabel(props.section.key, 'auto_extract', 'Auto-extract')}
          />
        </div>

        <div className={styles.globalsRow}>
          <div>
            <label htmlFor="skills-inject-count" className={styles.label}>
              {dt.fieldLabel(props.section.key, 'inject_count', 'Inject count')}
            </label>
            <div className={styles.help}>
              {dt.fieldHelp(props.section.key, 'inject_count', '')}
            </div>
          </div>
          <input
            id="skills-inject-count"
            type="number"
            className={styles.numberInput}
            value={asString(props.value.inject_count)}
            onChange={(e) => props.onField('inject_count', parseIntField(e.currentTarget.value))}
          />
        </div>

        <div className={styles.globalsRow}>
          <div>
            <label htmlFor="skills-half-life" className={styles.label}>
              {dt.fieldLabel(props.section.key, 'generation_half_life', 'Generation half-life')}
            </label>
            <div className={styles.help}>
              {dt.fieldHelp(props.section.key, 'generation_half_life', '')}
            </div>
          </div>
          <input
            id="skills-half-life"
            type="number"
            className={styles.numberInput}
            value={asString(props.value.generation_half_life)}
            onChange={(e) => props.onField('generation_half_life', parseIntField(e.currentTarget.value))}
          />
        </div>
      </div>

      <div className={styles.panel}>
        <h3 className={styles.panelTitle}>
          {t('skills.list')}
          <span className={styles.panelTitleMeta}>
            — {t('skills.listMeta', { count: total, disabledCount })}
          </span>
        </h3>

        {fetchState.status === 'loading' && (
          <div className={styles.statusRow}>{t('skills.loading')}</div>
        )}
        {fetchState.status === 'error' && (
          <div className={styles.errorRow}>
            <span>{t('skills.error', { msg: fetchState.message })}</span>
            <button type="button" className={styles.retryButton} onClick={() => load()}>
              {t('skills.errorRetry')}
            </button>
          </div>
        )}
        {fetchState.status === 'ok' && rows.length === 0 && (
          <div className={styles.statusRow}>{t('skills.empty')}</div>
        )}
        {fetchState.status === 'ok' && rows.map(row => (
          <div key={row.name} className={styles.skillRow}>
            <div>
              <div>
                <span className={styles.skillName}>{row.name}</span>
                {row.missing && (
                  <span className={styles.skillNameMissing}>{t('skills.rowMissing')}</span>
                )}
              </div>
              {row.description && <div className={styles.skillDescription}>{row.description}</div>}
            </div>
            <Switch
              checked={row.enabled}
              onChange={(next) => toggleSkill(row.name, next)}
              ariaLabel={t('skills.toggleAria', { name: row.name })}
            />
          </div>
        ))}
      </div>
    </section>
  );
}
