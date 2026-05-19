import type { ConfigField } from '../../../../api/schemas';

export interface ToolDetailProps {
  name: string;
  description?: string;
  toolset?: string;
  enabled: boolean;
  settings_schema?: ConfigField[];
  onToggle: (nextEnabled: boolean) => void;
  config?: Record<string, unknown>;
  onSectionField: (sectionKey: string, field: string, value: unknown) => void;
}

export interface McpDetailProps {
  key: string;
  command: string;
  enabled: boolean;
  onToggle: (nextEnabled: boolean) => void;
  serverConfig: Record<string, unknown>;
  onServerChange: (next: Record<string, unknown>) => void;
}
