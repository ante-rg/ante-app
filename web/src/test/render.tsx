import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render } from '@testing-library/react';
import { createMemoryRouter, RouterProvider } from 'react-router';
import { vi } from 'vitest';
import { routes } from '../routes';

type Route = { status: number; body: unknown };

export function mockApi(routes: Record<string, Route>) {
  const mock = vi.fn(async (input: RequestInfo | URL) => {
    const path = new URL(String(input), 'http://localhost').pathname;
    const { status, body } = routes[path] ?? { status: 404, body: { code: 'not_found', message: 'not found' } };
    return new Response(JSON.stringify(body), { status });
  });
  vi.stubGlobal('fetch', mock);
  return mock;
}

export const replayBody = {
  min_day: 0,
  max_day: 30,
  default_day: 12,
  model_version: 'test-v0',
  meta: { model_version: 'test-v0', dataset_version: null, feature_set_version: null, capacity: 10 },
};

export function renderApp(url: string) {
  const router = createMemoryRouter(routes, { initialEntries: [url] });
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return { router };
}
