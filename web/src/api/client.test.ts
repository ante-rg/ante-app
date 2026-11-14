import { describe, expect, it, vi } from 'vitest';
import { ApiError, getQueue } from './client';

describe('getQueue', () => {
  it('sends only the params given', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response('{}', { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);
    await getQueue({ asOfDay: 5, band: 'high', capacity: 3 });
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/queue?as_of_day=5&band=high&capacity=3');
  });

  it('passes the abort signal to fetch', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response('{}', { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);
    const { signal } = new AbortController();
    await getQueue({ asOfDay: 5 }, signal);
    expect(fetchMock.mock.calls[0][1]).toMatchObject({ signal });
  });

  it('rejects with ApiError carrying status and code', async () => {
    const body = JSON.stringify({ code: 'no_ranks', message: 'No ranks loaded' });
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(body, { status: 404 })));
    const err = await getQueue({ asOfDay: 1 }).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ status: 404, code: 'no_ranks' });
  });

  it('rejects with ApiError when the error body is JSON null', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('null', { status: 500 })));
    const err = await getQueue({ asOfDay: 1 }).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ status: 500, name: 'ApiError' });
  });
});
