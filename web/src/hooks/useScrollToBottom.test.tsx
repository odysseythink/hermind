import { describe, it, expect } from 'vitest';
import { renderHook } from '@testing-library/react';
import { useRef } from 'react';
import { useScrollToBottom } from './useScrollToBottom';

function Wrapper({ children }: { children: React.ReactNode }) {
  return <div>{children}</div>;
}

describe('useScrollToBottom', () => {
  it('returns isAtBottom true when scrollTop + clientHeight >= scrollHeight', () => {
    const { result } = renderHook(() => {
      const ref = useRef<HTMLDivElement>(null);
      const hook = useScrollToBottom(ref);
      return { hook, ref };
    }, { wrapper: Wrapper });

    // Simulate a DOM element
    const el = document.createElement('div');
    Object.defineProperty(el, 'scrollTop', { value: 0, writable: true });
    Object.defineProperty(el, 'clientHeight', { value: 100, writable: true });
    Object.defineProperty(el, 'scrollHeight', { value: 100, writable: true });
    (result.current.ref as React.MutableRefObject<HTMLDivElement | null>).current = el as unknown as HTMLDivElement;

    el.dispatchEvent(new Event('scroll'));
    expect(result.current.hook.isAtBottom).toBe(true);
  });
});
