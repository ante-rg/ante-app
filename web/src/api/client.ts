import type { components } from './schema';

export type Health = components['schemas']['Health'];
export type Replay = components['schemas']['Replay'];
export type Queue = components['schemas']['Queue'];
export type QueueItem = components['schemas']['QueueItem'];
export type Band = components['schemas']['Band'];

export class ApiError extends Error {
  override name = 'ApiError';
  constructor(
    public status: number,
    public code?: string,
    message?: string,
  ) {
    super(message ?? `request failed with ${status}`);
  }
}

export type QueueParams = {
  asOfDay: number;
  operator?: string;
  band?: 'high,elevated' | 'high';
  capacity?: number;
};

export async function getHealth(): Promise<Health> {
  const res = await fetch('/healthz');
  if (res.status !== 200 && res.status !== 503) throw new Error(`healthz returned ${res.status}`);
  return (await res.json()) as Health;
}

async function getJSON<T>(url: string, signal?: AbortSignal): Promise<T> {
  const res = await fetch(url, { signal });
  if (!res.ok) {
    const body = ((await res.json().catch(() => null)) ?? {}) as Partial<components['schemas']['Error']>;
    throw new ApiError(res.status, body.code, body.message);
  }
  return (await res.json()) as T;
}

export function getReplay(signal?: AbortSignal): Promise<Replay> {
  return getJSON<Replay>('/api/v1/replay', signal);
}

export function getQueue(p: QueueParams, signal?: AbortSignal): Promise<Queue> {
  const q = new URLSearchParams({ as_of_day: String(p.asOfDay) });
  if (p.operator) q.set('operator', p.operator);
  if (p.band) q.set('band', p.band);
  if (p.capacity !== undefined) q.set('capacity', String(p.capacity));
  return getJSON<Queue>(`/api/v1/queue?${q}`, signal);
}
