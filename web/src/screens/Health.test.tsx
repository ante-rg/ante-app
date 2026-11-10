import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Health } from './Health';

function mockFetch(status: number, body: unknown) {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify(body), { status })));
}

function renderHealth() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <Health />
    </QueryClientProvider>,
  );
}

describe('Health', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('shows ok status and version', async () => {
    mockFetch(200, { status: 'ok', database: 'ok', scorer: 'ok', version: 'abc123' });
    renderHealth();
    expect(await screen.findByText('API status: ok')).toBeInTheDocument();
    expect(screen.getByText('abc123')).toBeInTheDocument();
  });

  it('shows down when the API answers 503', async () => {
    mockFetch(503, { status: 'down', database: 'down', version: 'abc123' });
    renderHealth();
    expect(await screen.findByText('API status: down')).toBeInTheDocument();
  });

  it('shows an error when the API is unreachable', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('network')));
    renderHealth();
    expect(await screen.findByText("Can't reach the API")).toBeInTheDocument();
  });
});
