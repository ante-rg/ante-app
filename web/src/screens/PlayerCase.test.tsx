import { act, fireEvent, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { mockApi, renderApp, replayBody } from '../test/render';

const meta = { model_version: 'test-v0', dataset_version: null, feature_set_version: null, capacity: 10 };
const playerBody = {
  player_id: 'S01',
  operator_code: 'P3',
  as_of_day: 12,
  effective_day: 12,
  history_ended_on_day: null,
  days_observed: 13,
  sessions_to_date: 142,
  deposits_to_date: 301,
  deposits_eur_to_date: 9840,
  score: 0.91,
  band: 'high',
  rank: 1,
  rank_of: 100,
  first_high_day: 4,
  first_rule_flag_day: null,
  reasons: [{ code: 'chase', text: 'Deposited during losing sessions', weight: 0.46 }],
  rule_flags: [{ rule_id: 'dep30', text: 'Deposits over 2,000 € in 30 days', first_day: 9 }],
  meta,
};
const timelineBody = {
  player_id: 'S01',
  as_of_day: 12,
  first_high_day: 4,
  first_rule_flag_day: null,
  days: [{ day: 3, deposits_eur: 200 }],
  scores: [
    { day: 0, score: 0.1, band: 'low' },
    { day: 1, score: 0.9, band: 'high' },
  ],
  meta,
};

const api = (player: Record<string, unknown> = {}, extra: Record<string, { status: number; body: unknown }> = {}) =>
  mockApi({
    '/api/v1/replay': { status: 200, body: replayBody },
    '/api/v1/players/S01': { status: 200, body: { ...playerBody, ...player } },
    '/api/v1/players/S01/timeline': { status: 200, body: timelineBody },
    '/api/v1/players/S01/reviews': { status: 200, body: { items: [] } },
    ...extra,
  });

const urls = (mock: ReturnType<typeof api>) =>
  mock.mock.calls.map((c) => String(c[0])).filter((u) => !u.includes('/reviews'));

afterEach(() => vi.unstubAllGlobals());

describe('PlayerCaseScreen', () => {
  it('renders the header, lead, timeline, reasons and rules', async () => {
    api();
    renderApp('/players/S01?day=12');
    expect(
      await screen.findByText('Operator P3. Day 12. 142 sessions and 301 deposits so far, 9,840 € deposited.'),
    ).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Player S01' })).toBeInTheDocument();
    expect(screen.getByText('0.91')).toBeInTheDocument();
    expect(screen.getByText('Rank 1 of 100')).toBeInTheDocument();
    expect(screen.getByText('Relative priority for review')).toBeInTheDocument();
    expect(screen.getByText(/Model flagged high on/)).toHaveTextContent('Model flagged high on day 4.');
    expect(await screen.findByRole('heading', { name: 'Activity from day 0 to day 12' })).toBeInTheDocument();
    expect(document.querySelector('svg[role=img]')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Why this player is flagged' })).toBeInTheDocument();
    expect(screen.getByText('Deposited during losing sessions')).toBeInTheDocument();
    expect(screen.getByText('46%')).toBeInTheDocument();
    expect(document.querySelector('.wbar span')).toHaveStyle({ width: '46%' });
    expect(document.querySelector('.grid2 > .panel .reasons')).toBeInTheDocument();
    expect(screen.getByText('Deposits over 2,000 € in 30 days, first fired on day 9')).toBeInTheDocument();
  });

  it('renders the review panel', async () => {
    api();
    renderApp('/players/S01?day=12');
    expect(await screen.findByRole('heading', { name: 'Your review' })).toBeInTheDocument();
  });

  it('never shows the last day while the effective day is the requested one', async () => {
    api({ last_day: 315 });
    renderApp('/players/S01?day=12');
    await screen.findByText(/Operator P3/);
    expect(document.body.textContent).not.toContain('315');
  });

  it('says when the history ended', async () => {
    api({ effective_day: 10, history_ended_on_day: 10 });
    renderApp('/players/S01?day=12');
    expect(await screen.findByText(/History ends on day 10\./)).toBeInTheDocument();
  });

  it('shows empty reasons and rules copy', async () => {
    api({ reasons: [], rule_flags: [] });
    renderApp('/players/S01?day=12');
    expect(await screen.findByText('No reasons for this day. The score needs more history.')).toBeInTheDocument();
    expect(screen.getByText('No fixed rule has fired up to day 12.')).toBeInTheDocument();
  });

  it('asks both endpoints for the replay day and follows it', async () => {
    const mock = api();
    const { router } = renderApp('/players/S01?day=12');
    await screen.findByText(/Operator P3/);
    expect(urls(mock).filter((u) => u.includes('as_of_day=12') && u.includes('/players/S01'))).toHaveLength(2);
    await act(() => router.navigate('/players/S01?day=8'));
    await waitFor(() => expect(urls(mock).filter((u) => u.includes('as_of_day=8'))).toHaveLength(2));
  });

  it('shows "Not enough data" and no rank for a null score', async () => {
    api({ score: null, band: 'insufficient_data', rank: undefined, rank_of: undefined });
    renderApp('/players/S01?day=12');
    expect(await screen.findByText('Not enough data')).toBeInTheDocument();
    expect(screen.queryByText(/Rank/)).not.toBeInTheDocument();
  });

  it('says the player is not in the dataset on 404', async () => {
    api({}, { '/api/v1/players/S01': { status: 404, body: { code: 'not_found', message: 'x' } } });
    renderApp('/players/S01?day=12');
    expect(await screen.findByText('Player S01 is not in this dataset.')).toBeInTheDocument();
    expect(document.querySelector('.crumb a')).toHaveTextContent('Review queue');
  });

  it('says it cannot load on a server error', async () => {
    api({}, { '/api/v1/players/S01': { status: 500, body: { code: 'internal', message: 'x' } } });
    renderApp('/players/S01?day=12');
    expect(await screen.findByText("Can't load this case.")).toBeInTheDocument();
  });

  it('never shows data of another day or player while loading', async () => {
    let hang = false;
    vi.stubGlobal(
      'fetch',
      vi.fn((input: RequestInfo | URL) => {
        const u = new URL(String(input), 'http://localhost');
        if (hang && u.pathname.startsWith('/api/v1/players/')) return new Promise<Response>(() => {});
        const body = u.pathname.endsWith('/timeline')
          ? timelineBody
          : u.pathname === '/api/v1/replay'
            ? replayBody
            : { ...playerBody, effective_day: 20, as_of_day: 20 };
        return Promise.resolve(new Response(JSON.stringify(body), { status: 200 }));
      }),
    );
    const { router } = renderApp('/players/S01?day=20');
    expect(await screen.findByText(/Operator P3\. Day 20\./)).toBeInTheDocument();
    hang = true;
    await act(() => router.navigate('/players/S01?day=10'));
    expect(await screen.findByText('Loading the case…')).toBeInTheDocument();
    expect(screen.queryByText(/Day 20\./)).not.toBeInTheDocument();
    await act(() => router.navigate('/players/S02?day=10'));
    expect(await screen.findByRole('heading', { name: 'Player S02' })).toBeInTheDocument();
    expect(screen.queryByText(/Operator P3/)).not.toBeInTheDocument();
  });

  it('keeps the cached case with an alert when a refetch fails', async () => {
    const mock = api();
    const { router } = renderApp('/players/S01?day=12');
    await screen.findByText(/Operator P3\. Day 12\./);
    await act(() => router.navigate('/players/S01?day=8'));
    await waitFor(() => expect(urls(mock).filter((u) => u.includes('as_of_day=8'))).toHaveLength(2));
    mock.mockImplementation(async (input: RequestInfo | URL) => {
      const u = new URL(String(input), 'http://localhost');
      return u.pathname.startsWith('/api/v1/players/')
        ? new Response('{}', { status: 500 })
        : new Response(JSON.stringify(replayBody), { status: 200 });
    });
    await act(() => router.navigate('/players/S01?day=12'));
    expect(await screen.findByText("Can't refresh this case. Showing the last loaded day.")).toHaveAttribute(
      'role',
      'alert',
    );
    expect(screen.getByText(/Operator P3/)).toBeInTheDocument();
  });

  it('keeps the last loaded day with an alert when a new day fails to load', async () => {
    const mock = api();
    const { router } = renderApp('/players/S01?day=12');
    await screen.findByText(/Operator P3\. Day 12\./);
    mock.mockImplementation(async (input: RequestInfo | URL) => {
      const u = new URL(String(input), 'http://localhost');
      if (u.pathname === '/api/v1/replay') return new Response(JSON.stringify(replayBody), { status: 200 });
      if (u.pathname.endsWith('/timeline')) return new Response(JSON.stringify(timelineBody), { status: 200 });
      return new Response('{}', { status: 500 });
    });
    await act(() => router.navigate('/players/S01?day=15'));
    expect(await screen.findByText("Can't refresh this case. Showing the last loaded day.")).toHaveAttribute(
      'role',
      'alert',
    );
    expect(screen.getByRole('heading', { name: 'Player S01' })).toBeInTheDocument();
    expect(screen.getByText(/Operator P3\. Day 12\./)).toBeInTheDocument();
    expect(screen.queryByText("Can't load this case.")).not.toBeInTheDocument();
  });

  it('saves a review with the day of the case shown, not the requested day', async () => {
    const mock = api();
    const { router } = renderApp('/players/S01?day=12');
    await screen.findByText(/Operator P3\. Day 12\./);
    mock.mockImplementation(async (input: RequestInfo | URL, init?: RequestInit) => {
      const u = new URL(String(input), 'http://localhost');
      if (u.pathname === '/api/v1/replay') return new Response(JSON.stringify(replayBody), { status: 200 });
      if (u.pathname.endsWith('/timeline')) return new Response(JSON.stringify(timelineBody), { status: 200 });
      if (u.pathname.endsWith('/reviews')) {
        return init?.method === 'POST'
          ? new Response(
              JSON.stringify({
                id: 1,
                player_id: 'S01',
                reviewer: 'a@b.co',
                blind: false,
                model_version: 'v',
                created_at: '2026-01-01T00:00:00Z',
                ...JSON.parse(String(init.body)),
              }),
              { status: 201 },
            )
          : new Response(JSON.stringify({ items: [] }), { status: 200 });
      }
      return new Response('{}', { status: 500 });
    });
    await act(() => router.navigate('/players/S01?day=15'));
    await screen.findByText("Can't refresh this case. Showing the last loaded day.");
    fireEvent.click(screen.getByLabelText(/No concern/));
    fireEvent.click(screen.getByRole('button', { name: 'Save review' }));
    const calls = () => mock.mock.calls as unknown as [RequestInfo | URL, RequestInit?][];
    await waitFor(() => expect(calls().some((c) => c[1]?.method === 'POST')).toBe(true));
    const post = calls().find((c) => c[1]?.method === 'POST');
    expect(JSON.parse(String(post?.[1]?.body)).as_of_day).toBe(12);
  });

  it('puts the timeline legend in the panel header', async () => {
    api();
    renderApp('/players/S01?day=12');
    await screen.findByText(/Operator P3/);
    expect(document.querySelector('.panel-h .tl-legend')).toBeInTheDocument();
  });

  it('shows the chart only once the timeline day matches the case day', async () => {
    let hang = false;
    vi.stubGlobal(
      'fetch',
      vi.fn((input: RequestInfo | URL) => {
        const u = new URL(String(input), 'http://localhost');
        const d = Number(u.searchParams.get('as_of_day'));
        if (u.pathname === '/api/v1/replay') return Promise.resolve(new Response(JSON.stringify(replayBody)));
        if (u.pathname.endsWith('/timeline'))
          return Promise.resolve(new Response(JSON.stringify({ ...timelineBody, as_of_day: d })));
        if (hang) return new Promise<Response>(() => {});
        return Promise.resolve(new Response(JSON.stringify({ ...playerBody, as_of_day: d, effective_day: d })));
      }),
    );
    const { router } = renderApp('/players/S01?day=12');
    expect(await screen.findByText(/Operator P3\. Day 12\./)).toBeInTheDocument();
    await waitFor(() => expect(document.querySelector('svg[role=img]')).toBeInTheDocument());
    hang = true;
    await act(() => router.navigate('/players/S01?day=15'));
    await waitFor(() => expect(document.querySelector('svg[role=img]')).not.toBeInTheDocument());
    expect(screen.getByText(/Operator P3\. Day 12\./)).toBeInTheDocument();
  });

  it('alerts about the timeline alone and keeps the rest of the case', async () => {
    api({}, { '/api/v1/players/S01/timeline': { status: 500, body: { code: 'internal', message: 'x' } } });
    renderApp('/players/S01?day=12');
    expect(await screen.findByText("Can't load the timeline.")).toHaveAttribute('role', 'alert');
    expect(screen.getByText(/Operator P3/)).toBeInTheDocument();
    expect(screen.getByText('Deposited during losing sessions')).toBeInTheDocument();
    expect(screen.queryByText(/Can't refresh this case/)).not.toBeInTheDocument();
    expect(document.querySelector('svg[role=img]')).not.toBeInTheDocument();
  });

  it('links the crumb back to the queue with its filters at the current day', async () => {
    api();
    const { router } = renderApp('/');
    await act(() => router.navigate('/players/S01?day=12', { state: { queueSearch: '?band=high&day=5' } }));
    expect(await screen.findByRole('link', { name: 'Review queue', current: false })).toHaveAttribute(
      'href',
      '/?band=high&day=12',
    );
  });

  it('links the crumb to the plain queue without state', async () => {
    api();
    renderApp('/players/S01?day=12');
    await screen.findByText(/Operator P3/);
    expect(document.querySelector('.crumb a')).toHaveAttribute('href', '/?day=12');
  });

  it('encodes odd ids and shows them in the crumb', async () => {
    const mock = mockApi({
      '/api/v1/replay': { status: 200, body: replayBody },
      '/api/v1/players/a%2Fb%20c': { status: 200, body: { ...playerBody, player_id: 'a/b c' } },
      '/api/v1/players/a%2Fb%20c/timeline': { status: 200, body: timelineBody },
    });
    renderApp('/players/a%2Fb%20c?day=12');
    await screen.findByText(/Operator P3/);
    expect(urls(mock)).toContain('/api/v1/players/a%2Fb%20c?as_of_day=12');
    expect(document.querySelector('.crumb')).toHaveTextContent('Review queue / a/b c');
  });
});
