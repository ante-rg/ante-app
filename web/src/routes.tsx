import { Link, Outlet, type RouteObject, useLocation } from 'react-router';
import { ReplayControl } from './replay/ReplayControl';
import { ReplayDayProvider, ReplayGate, useOptionalReplayDay } from './replay/ReplayDay';
import { Health } from './screens/Health';
import { Placeholder, PlayerPlaceholder } from './screens/Placeholder';
import { QueueScreen } from './screens/Queue';

const NAV = [
  { to: '/', label: 'Review queue', isCurrent: (p: string) => p === '/' || p.startsWith('/players/') },
  { to: '/operators', label: 'Operators', isCurrent: (p: string) => p === '/operators' },
  { to: '/about', label: 'About the score', isCurrent: (p: string) => p === '/about' },
];

function Header() {
  const replay = useOptionalReplayDay();
  const { pathname } = useLocation();
  return (
    <header className="bar">
      <div className="wrap">
        <span className="mark">
          <svg width="22" height="22" viewBox="0 0 22 22" aria-hidden="true">
            <rect x="1" y="1" width="20" height="20" rx="2" fill="none" stroke="currentColor" strokeWidth="2" />
            <rect x="5" y="12" width="3" height="5" fill="currentColor" />
            <rect x="9.5" y="9" width="3" height="8" fill="currentColor" />
            <rect x="14" y="5" width="3" height="12" fill="var(--risk-high)" />
          </svg>
          Ante
        </span>
        <nav className="nav" aria-label="Main">
          {NAV.map((n) => (
            <Link
              key={n.to}
              to={replay ? `${n.to}?day=${replay.day}` : n.to}
              aria-current={n.isCurrent(pathname) ? 'page' : undefined}
            >
              {n.label}
            </Link>
          ))}
        </nav>
        {replay && <ReplayControl />}
      </div>
    </header>
  );
}

function Layout() {
  return (
    <ReplayDayProvider>
      <Header />
      <main className="wrap">
        <ReplayGate>
          <Outlet />
        </ReplayGate>
      </main>
    </ReplayDayProvider>
  );
}

export const routes: RouteObject[] = [
  {
    element: <Layout />,
    children: [
      { path: '/', element: <QueueScreen /> },
      { path: '/players/:playerId', element: <PlayerPlaceholder /> },
      { path: '/operators', element: <Placeholder title="Operators" /> },
      { path: '/about', element: <Placeholder title="About the score" /> },
    ],
  },
  {
    path: '/health',
    element: (
      <main className="wrap">
        <Health />
      </main>
    ),
  },
];
