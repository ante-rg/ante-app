import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createBrowserRouter, RouterProvider } from 'react-router';
import { ApiError } from './api/client';
import { routes } from './routes';

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: (n, e) => !(e instanceof ApiError && e.status === 404) && n < 1 } },
});

const router = createBrowserRouter(routes);

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  );
}
