import { describe, expect, it, vi } from 'vitest';

import { createHttpClient } from './http';

describe('createHttpClient auth headers', () => {
  it('attaches configured bearer token to requests', async () => {
    const fetchImplementation = vi.fn().mockResolvedValue(new Response('{}', { status: 200 }));
    const client = createHttpClient({
      baseURL: 'http://localhost:8880',
      authToken: 'control-token',
      telemetryHeaders: null,
      fetchImplementation,
    });

    await client.request({ path: '/providers', method: 'POST', body: {} });

    const init = fetchImplementation.mock.calls[0]?.[1] as RequestInit;
    expect(new Headers(init.headers).get('Authorization')).toBe('Bearer control-token');
  });

  it('keeps explicit Authorization header when supplied', async () => {
    const fetchImplementation = vi.fn().mockResolvedValue(new Response('{}', { status: 200 }));
    const client = createHttpClient({
      baseURL: 'http://localhost:8880',
      authToken: 'control-token',
      defaultHeaders: { Authorization: 'Bearer explicit-token' },
      telemetryHeaders: null,
      fetchImplementation,
    });

    await client.request({ path: '/providers', method: 'POST', body: {} });

    const init = fetchImplementation.mock.calls[0]?.[1] as RequestInit;
    expect(new Headers(init.headers).get('Authorization')).toBe('Bearer explicit-token');
  });
});
