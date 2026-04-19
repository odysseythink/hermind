import type { ReactNode } from 'react';
import type { Config } from '../api/schemas';
import type { GroupId } from './groups';

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

function modelsSummary(cfg: Config): ReactNode {
  const c = cfg as unknown as AnyRec;
  const model = typeof c.model === 'string' ? c.model : '(unset)';
  const providers = countKeys(c.providers);
  const fallbacks = Array.isArray(c.fallback_providers) ? c.fallback_providers.length : 0;
  return (
    <>
      <SummaryRow label="Default model" value={model} />
      <SummaryRow label="Providers" value={`${providers} configured`} />
      <SummaryRow label="Fallbacks" value={`${fallbacks} configured`} />
    </>
  );
}

function memorySummary(cfg: Config): ReactNode {
  const m = asRecord((cfg as unknown as AnyRec).memory);
  const enabled = m.enabled === true;
  const backend = Object.keys(m).find(k => k !== 'enabled') ?? '(none)';
  return (
    <>
      <SummaryRow label="Backend" value={backend} />
      <SummaryRow label="Enabled" value={enabled ? 'yes' : 'no'} />
    </>
  );
}

function skillsSummary(cfg: Config): ReactNode {
  const s = asRecord((cfg as unknown as AnyRec).skills);
  const disabled = Array.isArray(s.disabled) ? s.disabled.length : 0;
  const overrides = countKeys(s.platform_disabled);
  return (
    <>
      <SummaryRow label="Globally disabled" value={String(disabled)} />
      <SummaryRow label="Platform overrides" value={String(overrides)} />
    </>
  );
}

function runtimeSummary(cfg: Config): ReactNode {
  const c = cfg as unknown as AnyRec;
  const agent = asRecord(c.agent);
  const storage = asRecord(c.storage);
  const hasPrompt = typeof agent.prompt === 'string' && agent.prompt.length > 0;
  const storageKind = typeof storage.kind === 'string' ? storage.kind : '(default)';
  return (
    <>
      <SummaryRow label="Agent prompt" value={hasPrompt ? 'custom' : 'default'} />
      <SummaryRow label="Storage" value={storageKind} />
    </>
  );
}

function advancedSummary(cfg: Config): ReactNode {
  const c = cfg as unknown as AnyRec;
  const mcp = asRecord(c.mcp);
  const cron = asRecord(c.cron);
  const mcpServers = countKeys(mcp.servers);
  const cronJobs = Array.isArray(cron.jobs) ? cron.jobs.length : 0;
  return (
    <>
      <SummaryRow label="MCP servers" value={String(mcpServers)} />
      <SummaryRow label="Cron jobs" value={String(cronJobs)} />
    </>
  );
}

function observabilitySummary(cfg: Config): ReactNode {
  const c = cfg as unknown as AnyRec;
  const log = asRecord(c.logging);
  const met = asRecord(c.metrics);
  const trc = asRecord(c.tracing);
  const level = typeof log.level === 'string' ? log.level : '(default)';
  return (
    <>
      <SummaryRow label="Log level" value={level} />
      <SummaryRow label="Metrics" value={met.enabled === true ? 'on' : 'off'} />
      <SummaryRow label="Tracing" value={trc.enabled === true ? 'on' : 'off'} />
    </>
  );
}

const FNS: Record<Exclude<GroupId, 'gateway'>, (cfg: Config) => ReactNode> = {
  models: modelsSummary,
  memory: memorySummary,
  skills: skillsSummary,
  runtime: runtimeSummary,
  advanced: advancedSummary,
  observability: observabilitySummary,
};

export function summaryFor(group: GroupId, cfg: Config): ReactNode {
  if (group === 'gateway') return null;
  return FNS[group](cfg);
}
