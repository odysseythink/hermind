import type { z } from 'zod';

/** Thrown for any non-2xx response; carries the decoded JSON error if present. */
export class ApiError extends Error {
  constructor(public status: number, public body: unknown) {
    super(`api: ${status}`);
  }
}

/**
 * apiFetch sends a JSON request to hermind. No auth header is attached;
 * hermind binds to 127.0.0.1 only, and access is gated by localhost
 * reachability.
 */
export async function apiFetch<T>(
  path: string,
  opts: {
    method?: string;
    body?: unknown;
    schema?: z.ZodType<T>;
    signal?: AbortSignal;
  } = {},
): Promise<T> {
  const res = await fetch(path, {
    method: opts.method ?? 'GET',
    headers: { 'Content-Type': 'application/json' },
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
    signal: opts.signal,
  });

  const ctype = res.headers.get('content-type') ?? '';
  const parsed = ctype.includes('application/json') ? await res.json() : await res.text();

  if (!res.ok) {
    throw new ApiError(res.status, parsed);
  }

  if (opts.schema) {
    return opts.schema.parse(parsed);
  }
  return parsed as T;
}
