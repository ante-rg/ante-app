import { useQuery } from '@tanstack/react-query';
import { createContext, type ReactNode, useCallback, useContext, useEffect, useMemo } from 'react';
import { useSearchParams } from 'react-router';
import { ApiError, getReplay, type Replay } from '../api/client';

type ReplayDay = {
  day: number;
  minDay: number;
  maxDay: number;
  defaultDay: number;
  setDay: (n: number) => void;
  replay: Replay;
};

const Ctx = createContext<ReplayDay | null>(null);

const replayQuery = {
  queryKey: ['replay'],
  queryFn: ({ signal }: { signal: AbortSignal }) => getReplay(signal),
  staleTime: Infinity,
};

export function useOptionalReplayDay(): ReplayDay | null {
  return useContext(Ctx);
}

export function useReplayDay(): ReplayDay {
  const v = useOptionalReplayDay();
  if (!v) throw new Error('useReplayDay outside ReplayDayProvider');
  return v;
}

function Loaded({ replay, children }: { replay: Replay; children: ReactNode }) {
  const [params, setParams] = useSearchParams();
  const { min_day: minDay, max_day: maxDay, default_day: defaultDay } = replay;
  const clamp = useCallback((n: number) => Math.min(maxDay, Math.max(minDay, n)), [minDay, maxDay]);

  const raw = params.get('day');
  const parsed = raw !== null && /^-?\d+$/.test(raw) ? Number(raw) : null;
  const day = parsed === null ? defaultDay : clamp(parsed);

  const setDay = useCallback(
    (n: number) =>
      setParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          next.set('day', String(clamp(n)));
          return next;
        },
        { replace: true },
      ),
    [clamp, setParams],
  );

  useEffect(() => {
    if (raw !== null && raw !== String(day)) setDay(day);
  }, [raw, day, setDay]);

  const value = useMemo(
    () => ({ day, minDay, maxDay, defaultDay, setDay, replay }),
    [day, minDay, maxDay, defaultDay, setDay, replay],
  );
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function ReplayDayProvider({ children }: { children: ReactNode }) {
  const { data } = useQuery(replayQuery);
  return data ? <Loaded replay={data}>{children}</Loaded> : <Ctx.Provider value={null}>{children}</Ctx.Provider>;
}

export function ReplayGate({ children }: { children: ReactNode }) {
  const { data, error } = useQuery(replayQuery);
  if (data) return children;
  if (!error) return <p className="placeholder">Loading the replay…</p>;
  const noModel = error instanceof ApiError && (error.code === 'no_active_model' || error.code === 'no_ranks');
  return (
    <p className="placeholder" role="alert">
      {noModel ? (
        <>
          No model is loaded. Load artifacts with <code>make load MODEL=&lt;version&gt;</code>.
        </>
      ) : (
        "Can't reach the API"
      )}
    </p>
  );
}
