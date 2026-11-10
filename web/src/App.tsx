import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createBrowserRouter, Outlet, RouterProvider } from 'react-router';
import { Health } from './screens/Health';

const queryClient = new QueryClient({ defaultOptions: { queries: { retry: 1 } } });

function Layout() {
  return (
    <>
      <header className="app-header">
        <h1>Ante</h1>
      </header>
      <main>
        <Outlet />
      </main>
    </>
  );
}

const router = createBrowserRouter([{ element: <Layout />, children: [{ path: '/', element: <Health /> }] }]);

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  );
}
