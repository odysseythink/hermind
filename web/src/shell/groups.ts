export type GroupId =
  | 'models'
  | 'gateway'
  | 'memory'
  | 'skills'
  | 'runtime'
  | 'advanced'
  | 'observability';

export interface GroupDef {
  id: GroupId;
  label: string;
  plannedStage: string;
  configKeys: readonly string[];
  description: string;
  bullets: readonly string[];
}

export const GROUPS: readonly GroupDef[] = [
  {
    id: 'models',
    label: 'Models',
    plannedStage: '3 & 4',
    configKeys: ['model', 'providers', 'fallback_providers'],
    description: 'Default model and provider configuration.',
    bullets: [
      'Default model selection',
      'Provider configs (OpenAI, Anthropic, local, …)',
      'Fallback providers',
      'Per-provider fetch-models button',
    ],
  },
  {
    id: 'gateway',
    label: 'Gateway',
    plannedStage: 'done',
    configKeys: ['gateway'],
    description: 'Messaging platform instances (Feishu, DingTalk, WeChat, …).',
    bullets: ['Per-platform instance configuration', 'Secret handling', 'Connection test'],
  },
  {
    id: 'memory',
    label: 'Memory',
    plannedStage: '5',
    configKeys: ['memory'],
    description: 'Long-term memory backend configuration.',
    bullets: [
      'Backend selection (RetainDB, OpenViking, Byterover, Honcho, Mem0, …)',
      'Per-backend credentials and endpoints',
      'Enable/disable toggle',
    ],
  },
  {
    id: 'skills',
    label: 'Skills',
    plannedStage: 'done',
    configKeys: ['skills'],
    description: 'Skill enable/disable and per-platform overrides.',
    bullets: [
      'Global disabled list',
      'Per-platform overrides (CLI, gateway, cron)',
      'Auto-discovered skill catalog',
    ],
  },
  {
    id: 'runtime',
    label: 'Runtime',
    plannedStage: '3',
    configKeys: ['agent', 'auxiliary', 'terminal', 'storage'],
    description: 'Agent prompt, auxiliary config, terminal, and storage.',
    bullets: [
      'Agent system prompt',
      'Auxiliary model',
      'Terminal config',
      'Storage backend',
    ],
  },
  {
    id: 'advanced',
    label: 'Advanced',
    plannedStage: '7',
    configKeys: ['mcp', 'browser', 'cron'],
    description: 'MCP servers, browser automation, and scheduled jobs.',
    bullets: ['MCP server list', 'Browser (Browserbase / Camofox) config', 'Cron jobs'],
  },
  {
    id: 'observability',
    label: 'Observability',
    plannedStage: '3',
    configKeys: ['logging', 'metrics', 'tracing'],
    description: 'Logging level, metrics, and tracing.',
    bullets: ['Logging level and output', 'Metrics exporter', 'Tracing exporter'],
  },
] as const;

export const GROUP_IDS: ReadonlySet<GroupId> = new Set(GROUPS.map(g => g.id));

export function findGroup(id: GroupId): GroupDef {
  const g = GROUPS.find(x => x.id === id);
  if (!g) throw new Error(`unknown group id: ${id}`);
  return g;
}
