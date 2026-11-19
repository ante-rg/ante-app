import { describe, expect, it, vi } from 'vitest';
import { ApiError, createReview, getPlayer, getQueue, getReviews, getTimeline } from './client';

describe('getQueue', () => {
  it('sends only the params given', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response('{}', { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);
    await getQueue({ asOfDay: 5, band: 'high', capacity: 3 });
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/queue?as_of_day=5&band=high&capacity=3');
  });

  it('sends review_status only when given', async () => {
    const fetchMock = vi.fn(async (_url: string) => new Response('{}', { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);
    await getQueue({ asOfDay: 3, reviewStatus: 'pending' });
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/queue?as_of_day=3&review_status=pending');
    await getQueue({ asOfDay: 3 });
    expect(fetchMock.mock.calls[1][0]).not.toContain('review_status');
  });

  it('sends mode=blind only when asked', async () => {
    const fetchMock = vi.fn(async (_url: string) => new Response('{}', { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);
    await getQueue({ asOfDay: 3, mode: 'blind' });
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/queue?as_of_day=3&mode=blind');
    await getQueue({ asOfDay: 3 });
    expect(fetchMock.mock.calls[1][0]).not.toContain('mode');
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

describe('getPlayer / getTimeline', () => {
  it('encodes the player id and sends as_of_day', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response('{}', { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);
    await getPlayer('a/b c', 7);
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/players/a%2Fb%20c?as_of_day=7');
  });

  it('fetches the timeline', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response('{}', { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);
    await getTimeline('S01', 7);
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/players/S01/timeline?as_of_day=7');
  });

  it('rejects with ApiError on 404', async () => {
    const body = JSON.stringify({ code: 'not_found', message: 'nope' });
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(body, { status: 404 })));
    const err = await getPlayer('x', 1).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ status: 404, code: 'not_found' });
  });
});

describe('blind mode', () => {
  it('adds mode=blind to the three getters only when blind', async () => {
    const fetchMock = vi.fn(async (_u: string) => new Response('{"items":[]}', { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);
    await getPlayer('S01', 7, undefined, 'blind');
    await getTimeline('S01', 7, undefined, 'blind');
    await getReviews('S01', 7, undefined, 'blind');
    for (const c of fetchMock.mock.calls) expect(new URL(c[0], 'http://x').searchParams.get('mode')).toBe('blind');
    fetchMock.mockClear();
    await getPlayer('S01', 7);
    await getTimeline('S01', 7);
    await getReviews('S01', 7);
    for (const c of fetchMock.mock.calls) expect(c[0]).not.toContain('mode');
  });
});

describe('reviews', () => {
  it('fetches the reviews as of a day', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response('{"items":[]}', { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);
    await getReviews('S 1', 7);
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/players/S%201/reviews?as_of_day=7');
  });

  it('posts a review as JSON', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response('{"id":1}', { status: 201 }));
    vi.stubGlobal('fetch', fetchMock);
    const body = { as_of_day: 3, outcome: 'no_concern', mode: 'normal' } as const;
    await createReview('S01', body);
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe('/api/v1/players/S01/reviews');
    expect(init).toMatchObject({ method: 'POST', headers: { 'Content-Type': 'application/json' } });
    expect(JSON.parse(init.body)).toEqual(body);
  });

  it('rejects with ApiError on 422', async () => {
    const res = JSON.stringify({ code: 'invalid', message: 'm' });
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(res, { status: 422 })));
    const err = await createReview('S01', { as_of_day: 3, outcome: 'no_concern', mode: 'normal' }).catch(
      (e: unknown) => e,
    );
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ status: 422, code: 'invalid' });
  });
});
