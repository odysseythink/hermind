import type { ToolDetailProps, McpDetailProps } from './types';
import BrowserControlConfig from './browser/BrowserControlConfig';
import FilesystemConfig from './filesystem/FilesystemConfig';

export const toolDetailRegistry: Record<string, React.FC<ToolDetailProps>> = {
  browser_control: BrowserControlConfig,
  filesystem: FilesystemConfig,
};

export const mcpDetailRegistry: Record<string, React.FC<McpDetailProps>> = {};
