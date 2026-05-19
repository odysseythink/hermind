import type { ToolDetailProps, McpDetailProps } from './types';
import BrowserControlConfig from './browser/BrowserControlConfig';

export const toolDetailRegistry: Record<string, React.FC<ToolDetailProps>> = {
  browser_control: BrowserControlConfig,
};

export const mcpDetailRegistry: Record<string, React.FC<McpDetailProps>> = {};
