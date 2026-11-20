import { act, screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { mockApi, renderApp, replayBody } from '../test/render';

const body = {
  as_of_day: 7,
  items: [
    {
      operator_code: 'P1',
      players: 20,
      high: 2,
      elevated: 3,
      low: 14,
      insufficient_data: 1,
      flagged_by_rules_too: 4,
      reviewed: 5,
    },
    {
      operator_code: 'P2',
      players: 20,
      high: 0,
      elevated: 1,
      low: 19,
      insufficient_data: 0,
      flagged_by_rules_too: 0,
      reviewed: 0,
    },
  ],
};

const api = (q: { status: number; body: unknown } = { status: 200, body }) =>
  mockApi({ '/api/v1/replay': { status: 200, body: replayBody }, '/api/v1/operators': q });

const urls = (mock: ReturnType<typeof api>) =>
  mock.mock.calls.map((c) => String(c[0])).filter((u) => u.startsWith('/api/v1/operators'));

describe('OperatorsScreen', () => {
  it('renders one row per operator with its numbers', async () => {
    api();
    renderApp('/operators?day=7');
    const p1 = (await screen.findByRole('link', { name: 'P1' })).closest('tr') as HTMLElement;
    const heads = screen.getAllByRole('columnheader').map((h) => h.textContent);
    expect(heads).toEqual([
      'Operator',
      'Players',
      'Bands',
      'High',
      'Elevated',
      'Low',
      'Not enough data',
      'Also flagged by rules',
      'Reviewed',
    ]);
    const cells = within(p1)
      .getAllByRole('cell')
      .map((c) => c.textContent);
    expect([cells[1], ...cells.slice(3)]).toEqual(['20', '2', '3', '14', '1', '4', '5']);
    expect(screen.getAllByRole('row')).toHaveLength(3);
  });

  it('links the operator to the queue, keeping the day', async () => {
    api();
    renderApp('/operators?day=7');
    expect(await screen.findByRole('link', { name: 'P1' })).toHaveAttribute('href', '/?operator=P1&day=7');
  });

  it('keeps blind mode in the link', async () => {
    api();
    renderApp('/operators?day=7&mode=blind');
    expect(await screen.findByRole('link', { name: 'P1' })).toHaveAttribute('href', '/?operator=P1&day=7&mode=blind');
  });

  it('draws the band strip', async () => {
    api();
    renderApp('/operators?day=7');
    expect(
      await screen.findByRole('img', { name: '2 high, 3 elevated, 14 low, 1 with not enough data' }),
    ).toBeInTheDocument();
  });

  it('requests with the day and again when it changes', async () => {
    const mock = api();
    const { router } = renderApp('/operators?day=7');
    await waitFor(() => expect(urls(mock).some((u) => u.includes('as_of_day=7'))).toBe(true));
    await act(() => router.navigate('/operators?day=8'));
    await waitFor(() => expect(urls(mock).some((u) => u.includes('as_of_day=8'))).toBe(true));
  });

  it('shows an error when the first load fails', async () => {
    api({ status: 500, body: { code: 'internal', message: 'x' } });
    renderApp('/operators?day=7');
    expect(await screen.findByRole('alert')).toHaveTextContent("Can't load operators.");
  });

  it('says so when there are no operators', async () => {
    api({ status: 200, body: { as_of_day: 7, items: [] } });
    renderApp('/operators?day=7');
    expect(await screen.findByText('No operators in this dataset.')).toBeInTheDocument();
  });

  const pendingAt = (pendingDay: string) =>
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const u = new URL(String(input), 'http://localhost');
        if (u.pathname === '/api/v1/replay') return new Response(JSON.stringify(replayBody));
        if (u.searchParams.get('as_of_day') === pendingDay) return new Promise<Response>(() => {});
        return new Response(JSON.stringify(body));
      }),
    );

  it('does not show later-day rows under an earlier day', async () => {
    pendingAt('7');
    const { router } = renderApp('/operators?day=8');
    await screen.findByRole('link', { name: 'P1' });
    await act(() => router.navigate('/operators?day=7'));
    await waitFor(() => expect(screen.queryByRole('link', { name: 'P1' })).toBeNull());
    expect(screen.getByText('Loading operators…')).toBeInTheDocument();
  });

  it('keeps earlier-day rows while a later day loads', async () => {
    pendingAt('8');
    const { router } = renderApp('/operators?day=7');
    await screen.findByRole('link', { name: 'P1' });
    await act(() => router.navigate('/operators?day=8'));
    expect(screen.getByRole('link', { name: 'P1' })).toBeInTheDocument();
  });
});
