import { fireEvent, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { mockApi, renderApp, replayBody } from '../test/render';

const trend = Array.from({ length: 30 }, (_, i) => 0.5 + i / 100);
const queueBody = {
  as_of_day: 7,
  mode: 'normal',
  counts: { high: 2, elevated: 2, low: 15, insufficient_data: 1, pending_review: 4, new_high_since_previous_day: 1 },
  meta: { ...replayBody.meta, capacity: 10 },
  items: [
    {
      player_id: 'S01',
      operator_code: 'P1',
      effective_day: 7,
      days_observed: 7,
      rank: 1,
      score: 0.91,
      band: 'high',
      score_trend_30d: trend,
      main_reason: 'R1',
      flagged_by_rules: true,
      review_status: 'pending',
    },
    {
      player_id: 'S02',
      operator_code: 'P2',
      effective_day: 5,
      history_ended_on_day: 5,
      days_observed: 5,
      rank: 2,
      score: 0.8,
      band: 'elevated',
      score_trend_30d: trend,
      main_reason: 'R2',
      flagged_by_rules: false,
      review_status: 'pending',
    },
    {
      player_id: 'S20',
      operator_code: 'P3',
      effective_day: 7,
      days_observed: 7,
      band: 'insufficient_data',
      score_trend_30d: Array(30).fill(null),
      main_reason: 'R3',
      flagged_by_rules: false,
      review_status: 'pending',
    },
  ],
};

const api = (q: { status: number; body: unknown } = { status: 200, body: queueBody }) =>
  mockApi({ '/api/v1/replay': { status: 200, body: replayBody }, '/api/v1/queue': q });

const queueUrls = (mock: ReturnType<typeof api>) =>
  mock.mock.calls.map((c) => String(c[0])).filter((u) => u.startsWith('/api/v1/queue'));

const rowOf = (name: string) => screen.getByRole('row', { name });

describe('QueueScreen', () => {
  it('renders rows from the API', async () => {
    api();
    renderApp('/?day=7');
    const r1 = await screen.findByRole('row', { name: 'Open case S01' });
    const rows = screen.getAllByRole('row').slice(1);
    expect(rows.map((r) => r.getAttribute('aria-label'))).toEqual(['Open case S01', 'Open case S02', 'Open case S20']);
    expect(r1).toHaveTextContent('High');
    expect(r1).toHaveTextContent('0.91');
    expect(r1).toHaveTextContent('R1');
    expect(r1).toHaveTextContent('Also flagged');
    expect(rowOf('Open case S02')).toHaveTextContent('History ends on day 5');
    const s20 = rowOf('Open case S20');
    expect(s20).toHaveTextContent('Not enough data');
    expect(s20.querySelector('svg')).toBeNull();
    for (const r of rows) expect(r).toHaveTextContent('To review');
  });

  it('shows head and population', async () => {
    api();
    renderApp('/?day=7');
    expect(await screen.findByText(/20 players from 5 operators/)).toBeInTheDocument();
    expect(screen.getByText(/relative priority for human review, not a diagnosis/)).toBeInTheDocument();
    expect(screen.getByRole('img', { name: '2 high, 2 elevated, 15 low, 1 with not enough data' })).toBeInTheDocument();
    expect(screen.getByText(/entered high since day/)).toHaveTextContent('1 entered high since day 6');
  });

  it('requests with the day', async () => {
    const mock = api();
    const { router } = renderApp('/?day=7');
    await waitFor(() => expect(queueUrls(mock).some((u) => u.includes('as_of_day=7'))).toBe(true));
    expect(queueUrls(mock)[0]).toContain('band=high%2Celevated');
    await router.navigate('/?day=8');
    await waitFor(() => expect(queueUrls(mock).some((u) => u.includes('as_of_day=8'))).toBe(true));
  });

  it('filters by band and keeps the day', async () => {
    const mock = api();
    const { router } = renderApp('/?day=7');
    await screen.findByRole('row', { name: 'Open case S01' });
    fireEvent.click(screen.getByRole('button', { name: 'High' }));
    expect(router.state.location.search).toBe('?day=7&band=high');
    await waitFor(() =>
      expect(queueUrls(mock).some((u) => u.includes('band=high&') || u.endsWith('band=high'))).toBe(true),
    );
    fireEvent.click(screen.getByRole('button', { name: 'High and elevated' }));
    expect(router.state.location.search).toBe('?day=7');
  });

  it('filters by operator', async () => {
    const mock = api();
    const { router } = renderApp('/?day=7');
    await screen.findByRole('row', { name: 'Open case S01' });
    fireEvent.change(screen.getByLabelText('Operator'), { target: { value: 'P3' } });
    expect(router.state.location.search).toBe('?day=7&operator=P3');
    await waitFor(() => expect(queueUrls(mock).some((u) => u.includes('operator=P3'))).toBe(true));
  });

  it('changes show top and ignores invalid values', async () => {
    const mock = api();
    const { router } = renderApp('/?day=7');
    await screen.findByRole('row', { name: 'Open case S01' });
    const input = screen.getByLabelText('Show top');
    expect(input).toHaveValue(10);
    fireEvent.change(input, { target: { value: '3' } });
    expect(router.state.location.search).toBe('?day=7&capacity=3');
    await waitFor(() => expect(queueUrls(mock).some((u) => u.includes('capacity=3'))).toBe(true));
    fireEvent.change(input, { target: { value: '' } });
    expect(router.state.location.search).toBe('?day=7');
    fireEvent.change(input, { target: { value: '0' } });
    fireEvent.change(input, { target: { value: '5000' } });
    expect(router.state.location.search).toBe('?day=7');
    expect(queueUrls(mock).some((u) => u.includes('capacity=0') || u.includes('capacity=5000'))).toBe(false);
  });

  it('marks an invalid show top draft', async () => {
    api();
    renderApp('/?day=7');
    await screen.findByRole('row', { name: 'Open case S01' });
    const input = screen.getByLabelText('Show top');
    expect(input).not.toHaveAttribute('aria-invalid');
    fireEvent.change(input, { target: { value: '0' } });
    expect(input).toHaveAttribute('aria-invalid', 'true');
    fireEvent.change(input, { target: { value: '4' } });
    expect(input).not.toHaveAttribute('aria-invalid');
  });

  it('applies filters from a shared link to the first request', async () => {
    const mock = api();
    renderApp('/?day=7&operator=P3&band=high&capacity=3');
    await screen.findByRole('row', { name: 'Open case S01' });
    expect(queueUrls(mock)[0]).toBe('/api/v1/queue?as_of_day=7&operator=P3&band=high&capacity=3');
    expect(screen.getByLabelText('Operator')).toHaveValue('P3');
    expect(screen.getByRole('button', { name: 'High' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByLabelText('Show top')).toHaveValue(3);
  });

  it('keeps the rows when a background refetch fails', async () => {
    let queueCalls = 0;
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const u = new URL(String(input), 'http://localhost');
        if (u.pathname === '/api/v1/replay') return new Response(JSON.stringify(replayBody));
        queueCalls += 1;
        return queueCalls < 3
          ? new Response(JSON.stringify(queueBody))
          : new Response(JSON.stringify({ code: 'internal', message: 'x' }), { status: 500 });
      }),
    );
    const { router } = renderApp('/?day=7');
    await screen.findByRole('row', { name: 'Open case S01' });
    await router.navigate('/?day=8');
    await waitFor(() => expect(queueCalls).toBe(2));
    await router.navigate('/?day=7');
    await waitFor(() => expect(queueCalls).toBe(3));
    expect(await screen.findByRole('alert')).toHaveTextContent("Can't refresh the queue");
    expect(screen.getByRole('row', { name: 'Open case S01' })).toBeInTheDocument();
  });

  it('does not show later-day rows under an earlier day', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const u = new URL(String(input), 'http://localhost');
        if (u.pathname === '/api/v1/replay') return new Response(JSON.stringify(replayBody));
        if (u.searchParams.get('as_of_day') === '5') return new Promise<Response>(() => {});
        return new Response(JSON.stringify(queueBody));
      }),
    );
    const { router } = renderApp('/?day=7');
    await screen.findByRole('row', { name: 'Open case S01' });
    await router.navigate('/?day=5');
    await waitFor(() => expect(screen.queryByRole('row', { name: 'Open case S01' })).toBeNull());
  });

  it('ignores an unknown operator param', async () => {
    const mock = api();
    renderApp('/?day=7&operator=zzz');
    await screen.findByRole('row', { name: 'Open case S01' });
    expect(queueUrls(mock).every((u) => !u.includes('operator='))).toBe(true);
    expect(screen.getByLabelText('Operator')).toHaveValue('');
  });

  it('footer uses the response capacity, input the card default', async () => {
    api({ status: 200, body: { ...queueBody, meta: { ...queueBody.meta, capacity: 3 } } });
    renderApp('/?day=7');
    expect(await screen.findByText('3 players shown. Top 3 by review capacity.')).toBeInTheDocument();
    expect(screen.getByLabelText('Show top')).toHaveValue(10);
  });

  it('uses singular nouns for one player and one day', async () => {
    api({ status: 200, body: { ...queueBody, items: [queueBody.items[0]] } });
    renderApp('/?day=1');
    expect(await screen.findByText('1 player shown. Top 10 by review capacity.')).toBeInTheDocument();
    expect(screen.getByText(/each shown 1 day after/)).toBeInTheDocument();
  });

  it('shows the footer', async () => {
    api();
    renderApp('/?day=7');
    expect(await screen.findByText('3 players shown. Top 10 by review capacity.')).toBeInTheDocument();
  });

  it('shows the empty row', async () => {
    api({ status: 200, body: { ...queueBody, items: [] } });
    renderApp('/?day=7');
    expect(
      await screen.findByText('No players match these filters. Choose "All operators" or another band.'),
    ).toBeInTheDocument();
  });

  it('moves with j/k and opens with Enter or click', async () => {
    api();
    const { router } = renderApp('/?day=7');
    const s01 = await screen.findByRole('row', { name: 'Open case S01' });
    s01.focus();
    fireEvent.keyDown(s01, { key: 'j' });
    expect(rowOf('Open case S02')).toHaveFocus();
    fireEvent.keyDown(rowOf('Open case S02'), { key: 'k' });
    expect(s01).toHaveFocus();
    fireEvent.keyDown(s01, { key: 'Enter' });
    expect(router.state.location.pathname).toBe('/players/S01');
    expect(router.state.location.search).toBe('?day=7');
  });

  it('ignores j/k/Enter with a modifier key', async () => {
    api();
    const { router } = renderApp('/?day=7');
    const s01 = await screen.findByRole('row', { name: 'Open case S01' });
    s01.focus();
    fireEvent.keyDown(s01, { key: 'k', ctrlKey: true });
    fireEvent.keyDown(s01, { key: 'j', ctrlKey: true });
    fireEvent.keyDown(s01, { key: 'Enter', metaKey: true });
    expect(s01).toHaveFocus();
    expect(router.state.location.pathname).toBe('/');
  });

  it('encodes the player id in the case URL', async () => {
    const body = { ...queueBody, items: [{ ...queueBody.items[0], player_id: 'a/b c' }] };
    api({ status: 200, body });
    const { router } = renderApp('/?day=7');
    fireEvent.click(await screen.findByRole('row', { name: 'Open case a/b c' }));
    expect(router.state.location.pathname).toBe('/players/a%2Fb%20c');
  });

  it('shows a loading row while the queue is pending', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) =>
        String(input).startsWith('/api/v1/queue')
          ? new Promise<Response>(() => {})
          : new Response(JSON.stringify(replayBody), { status: 200 }),
      ),
    );
    renderApp('/?day=7');
    expect(await screen.findByRole('row', { name: 'Loading the queue…' })).toBeInTheDocument();
  });

  it('opens a case on click', async () => {
    api();
    const { router } = renderApp('/?day=7');
    fireEvent.click(await screen.findByRole('row', { name: 'Open case S02' }));
    expect(router.state.location.pathname).toBe('/players/S02');
    expect(router.state.location.search).toBe('?day=7');
  });

  it('keeps the header when the queue fails', async () => {
    api({ status: 500, body: { code: 'internal', message: 'x' } });
    renderApp('/?day=7');
    expect(await screen.findByRole('alert')).toHaveTextContent("Can't load the queue.");
    expect(screen.getByLabelText('Data as of day')).toBeInTheDocument();
  });
});
