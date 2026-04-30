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

export async function apiPut<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(path, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  const ctype = res.headers.get('content-type') ?? '';
  const parsed = ctype.includes('application/json') ? await res.json() : await res.text();
  if (!res.ok) {
    throw new ApiError(res.status, parsed);
  }
  return parsed as T;
}

export async function apiDelete(path: string): Promise<void> {
  const res = await fetch(path, { method: 'DELETE' });
  if (!res.ok) {
    const text = await res.text();
    throw new ApiError(res.status, text);
  }
}

export async function apiUpload(path: string, file: File): Promise<unknown> {
  const formData = new FormData();
  formData.append('file', file);
  const res = await fetch(path, { method: 'POST', body: formData });
  if (!res.ok) {
    const text = await res.text();
    throw new ApiError(res.status, text);
  }
  return res.json();
}
