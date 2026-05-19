import { describe, it, expect } from 'vitest';
import { toolDetailRegistry, mcpDetailRegistry } from './registry';

describe('toolDetailRegistry', () => {
  it('contains browser_control mapped to a component', () => {
    expect(toolDetailRegistry['browser_control']).toBeDefined();
    expect(typeof toolDetailRegistry['browser_control']).toBe('function');
  });

  it('does not contain an unregistered tool', () => {
    expect(toolDetailRegistry['nonexistent_tool']).toBeUndefined();
  });
});

describe('mcpDetailRegistry', () => {
  it('is initially empty', () => {
    expect(Object.keys(mcpDetailRegistry)).toHaveLength(0);
  });
});
