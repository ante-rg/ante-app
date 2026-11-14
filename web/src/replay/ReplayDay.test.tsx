import { fireEvent, screen, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { mockApi, renderApp, replayBody } from '../test/render';

const ok = { '/api/v1/replay': { status: 200, body: replayBody } };

describe('ReplayDayProvider', () => {
  it('uses default_day and leaves the URL alone when there is no day', async () => {
    mockApi(ok);
    const { router } = renderApp('/');
    expect(await screen.findByRole('status')).toHaveTextContent('12');
    expect(router.state.location.search).toBe('');
  });

  it('clamps a day above max_day and corrects the URL', async () => {
    mockApi(ok);
    const { router } = renderApp('/?day=999');
    expect(await screen.findByRole('status')).toHaveTextContent('30');
    expect(router.state.location.search).toBe('?day=30');
  });

  it('falls back to default_day for a non-number', async () => {
    mockApi(ok);
    renderApp('/?day=abc');
    expect(await screen.findByRole('status')).toHaveTextContent('12');
  });

  it('setDay keeps the other params', async () => {
    mockApi(ok);
    const { router } = renderApp('/?day=7&band=high');
    fireEvent.change(await screen.findByLabelText('Data as of day'), { target: { value: '9' } });
    expect(router.state.location.search).toBe('?day=9&band=high');
  });

  it.each(['no_ranks', 'no_active_model'])('says no model is loaded on 404 %s', async (code) => {
    mockApi({ '/api/v1/replay': { status: 404, body: { code, message: 'x' } } });
    renderApp('/');
    expect(await screen.findByText(/No model is loaded\. Load artifacts with/)).toBeInTheDocument();
  });

  it('says the API is unreachable on 500', async () => {
    mockApi({ '/api/v1/replay': { status: 500, body: { code: 'internal', message: 'x' } } });
    renderApp('/');
    expect(await screen.findByText("Can't reach the API")).toBeInTheDocument();
  });

  it('renders /health when no model is loaded', async () => {
    mockApi({
      '/api/v1/replay': { status: 404, body: { code: 'no_ranks', message: 'x' } },
      '/healthz': { status: 200, body: { status: 'ok', version: 'abc' } },
    });
    renderApp('/health');
    expect(await screen.findByText('API status: ok')).toBeInTheDocument();
  });

  it('keeps the day on nav links and marks the queue current on a player route', async () => {
    mockApi(ok);
    renderApp('/players/S01?day=7');
    await screen.findByRole('status');
    const link = screen.getByRole('link', { name: 'Review queue', current: 'page' });
    expect(link).toHaveAttribute('href', '/?day=7');
  });

  it('keeps the mark and nav, and puts the message in main, when no model is loaded', async () => {
    mockApi({ '/api/v1/replay': { status: 404, body: { code: 'no_ranks', message: 'x' } } });
    renderApp('/');
    const msg = await screen.findByText(/No model is loaded/);
    expect(within(screen.getByRole('main')).getByText(/No model is loaded/)).toBe(msg);
    expect(screen.getByText('Ante')).toBeVisible();
    expect(screen.getByRole('link', { name: 'Review queue' })).toHaveAttribute('href', '/');
    expect(screen.queryByLabelText('Data as of day')).not.toBeInTheDocument();
  });

  it('shows the mark and the loading text in main while loading', () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(() => new Promise(() => {})),
    );
    renderApp('/');
    expect(screen.getByText('Ante')).toBeVisible();
    expect(within(screen.getByRole('main')).getByText('Loading the replay…')).toBeInTheDocument();
  });
});
