import { act, fireEvent, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { mockApi, renderApp, replayBody } from '../test/render';

const ok = { '/api/v1/replay': { status: 200, body: replayBody } };

async function start(url: string) {
  mockApi(ok);
  const { router } = renderApp(url);
  const play = await screen.findByRole('button', { name: 'Play' });
  vi.useFakeTimers();
  return { router, play };
}

describe('ReplayControl', () => {
  afterEach(() => vi.useRealTimers());

  it('plays one day per 400 ms and stops at max_day', async () => {
    const { play } = await start('/?day=28');
    fireEvent.click(play);
    expect(screen.getByRole('button', { name: 'Pause' })).toHaveAttribute('aria-pressed', 'true');
    await act(() => vi.advanceTimersByTimeAsync(400));
    expect(screen.getByRole('status')).toHaveTextContent('29');
    await act(() => vi.advanceTimersByTimeAsync(400));
    expect(screen.getByRole('status')).toHaveTextContent('30');
    expect(screen.getByRole('button', { name: 'Play' })).toBeInTheDocument();
  });

  it('restarts from 0 when pressed at max_day', async () => {
    const { play } = await start('/?day=30');
    fireEvent.click(play);
    expect(screen.getByRole('status').textContent).toBe('0');
    expect(screen.getByRole('button', { name: 'Pause' })).toBeInTheDocument();
  });
});
