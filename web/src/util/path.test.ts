import { describe, expect, it } from 'vitest';
import { getPath, setPath } from './path';

describe('getPath', () => {
  it('returns a flat field', () => {
    expect(getPath({ a: 1 }, 'a')).toBe(1);
  });
  it('returns a nested field', () => {
    expect(getPath({ a: { b: 2 } }, 'a.b')).toBe(2);
  });
  it('returns undefined when an intermediate key is missing', () => {
    expect(getPath({}, 'a.b')).toBeUndefined();
  });
});

describe('setPath', () => {
  it('writes a flat field', () => {
    expect(setPath({ a: 1 }, 'a', 2)).toEqual({ a: 2 });
  });
  it('writes a nested field, creating intermediates', () => {
    expect(setPath({}, 'a.b', 2)).toEqual({ a: { b: 2 } });
  });
  it('does not mutate the input', () => {
    const input = { a: { b: 1 } };
    const out = setPath(input, 'a.b', 2);
    expect(input).toEqual({ a: { b: 1 } });
    expect(out).toEqual({ a: { b: 2 } });
    expect(out).not.toBe(input);
  });
});
