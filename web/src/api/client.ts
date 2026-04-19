import { z } from 'zod';

/**
 * Token resolution order:
 *  1. VITE_HERMIND_TOKEN env (Vite dev mode, via web/.env.local).
 *  2. window.HERMIND.token (prod — injected by api/server.go::handleIndex).
 */
export function resolveToken(): string {
  const envTok = import.meta.env.VITE_HERMIND_TOKEN as string | undefined;
  if (envTok && envTok.length > 0) return envTok;
  const globalTok = (window as unknown as { HERMIND?: { token?: string } }).HERMIND?.token;
  if (globalTok && globalTok.length > 0 && globalTok !== '{{TOKEN}}') {
    return globalTok;
  }
  return '';
}

/** Thrown for any non-2xx response; carries the decoded JSON error if present. */
export class ApiError extends Error {
  constructor(public status: number, public body: unknown) {
    super(`api: ${status}`);
  }
}

/**
 * apiFetch sends a JSON request to hermind with the Bearer token
 * automatically attached. The response is parsed as JSON and passed
 * through the optional zod schema — callers get a typed value or a
 * thrown ApiError. 401/403 responses throw ApiError with status set
 * so the caller can surface a "token invalid" banner.
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
  const token = resolveToken();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  };
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const res = await fetch(path, {
    method: opts.method ?? 'GET',
    headers,
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
