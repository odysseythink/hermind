import type { ReactNode } from 'react';
import type { Config } from '../api/schemas';
import type { GroupId } from './groups';
import { useTranslation } from 'react-i18next';

type AnyRec = Record<string, unknown>;

function asRecord(v: unknown): AnyRec {
  return v && typeof v === 'object' ? (v as AnyRec) : {};
}

function countKeys(v: unknown): number {
  return Object.keys(asRecord(v)).length;
}

function SummaryRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="summary-row">
      <span className="summary-label">{label}</span>
      <span className="summary-value">{value}</span>
    </div>
  );
}

export function SummaryFor({ group, cfg }: { group: GroupId; cfg: Config }) {
  const { t } = useTranslation('ui');
  const c = cfg as unknown as AnyRec;
  switch (group) {
    case 'gateway': return null;
    case 'models': {
      const model = typeof c.model === 'string' ? c.model : t('summary.unset');
      const providers = countKeys(c.providers);
      const fallbacks = Array.isArray(c.fallback_providers) ? c.fallback_providers.length : 0;
      return (
        <>
          <SummaryRow label={t('summary.defaultModel')} value={model} />
          <SummaryRow label={t('summary.providers')} value={t('summary.configuredCount', { count: providers })} />
          <SummaryRow label={t('summary.fallbacks')} value={t('summary.configuredCount', { count: fallbacks })} />
        </>
      );
    }
    case 'memory': {
      const m = asRecord(c.memory);
      const enabled = m.enabled === true;
      const backend = Object.keys(m).find(k => k !== 'enabled') ?? t('summary.none');
      return (
        <>
          <SummaryRow label={t('summary.backend')} value={backend} />
          <SummaryRow label={t('summary.enabled')} value={enabled ? t('summary.yes') : t('summary.no')} />
        </>
      );
    }
    case 'skills': {
      const s = asRecord(c.skills);
      const disabled = Array.isArray(s.disabled) ? s.disabled.length : 0;
      const overrides = countKeys(s.platform_disabled);
      return (
        <>
          <SummaryRow label={t('summary.globallyDisabled')} value={String(disabled)} />
          <SummaryRow label={t('summary.platformOverrides')} value={String(overrides)} />
        </>
      );
    }
    case 'runtime': {
      const agent = asRecord(c.agent);
      const storage = asRecord(c.storage);
      const hasPrompt = typeof agent.prompt === 'string' && agent.prompt.length > 0;
      const storageKind = typeof storage.kind === 'string' ? storage.kind : t('summary.default');
      return (
        <>
          <SummaryRow label={t('summary.agentPrompt')} value={hasPrompt ? t('summary.custom') : t('summary.default')} />
          <SummaryRow label={t('summary.storage')} value={storageKind} />
        </>
      );
    }
    case 'advanced': {
      const mcp = asRecord(c.mcp);
      const cron = asRecord(c.cron);
      const mcpServers = countKeys(mcp.servers);
      const cronJobs = Array.isArray(cron.jobs) ? cron.jobs.length : 0;
      return (
        <>
          <SummaryRow label={t('summary.mcpServers')} value={String(mcpServers)} />
          <SummaryRow label={t('summary.cronJobs')} value={String(cronJobs)} />
        </>
      );
    }
    case 'observability': {
      const log = asRecord(c.logging);
      const met = asRecord(c.metrics);
      const trc = asRecord(c.tracing);
      const level = typeof log.level === 'string' ? log.level : t('summary.default');
      return (
        <>
          <SummaryRow label={t('summary.logLevel')} value={level} />
          <SummaryRow label={t('summary.metrics')} value={met.enabled === true ? t('summary.on') : t('summary.off')} />
          <SummaryRow label={t('summary.tracing')} value={trc.enabled === true ? t('summary.on') : t('summary.off')} />
        </>
      );
    }
  }
  return null;
}

export function summaryFor(group: GroupId, cfg: Config): ReactNode {
  if (group === 'gateway') return null;
  return <SummaryFor group={group} cfg={cfg} />;
}
