import type { components, operations } from './schema';

export type Health = components['schemas']['Health'];
export type Replay = components['schemas']['Replay'];
export type Queue = components['schemas']['Queue'];
export type QueueItem = components['schemas']['QueueItem'];
export type PlayerCase = components['schemas']['PlayerCase'];
export type Timeline = components['schemas']['Timeline'];
export type TimelineDay = components['schemas']['TimelineDay'];
export type TimelineScore = components['schemas']['TimelineScore'];
export type Review = components['schemas']['Review'];
export type ReviewCreate = components['schemas']['ReviewCreate'];
export type Outcome = components['schemas']['Outcome'];
export type OperatorSummary = components['schemas']['OperatorSummary'];
export type Operators = operations['listOperators']['responses'][200]['content']['application/json'];
export type Band = components['schemas']['Band'];
export type Mode = 'blind';

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
  reviewStatus?: 'pending' | 'reviewed';
  mode?: Mode;
};

export async function getHealth(): Promise<Health> {
  const res = await fetch('/healthz');
  if (res.status !== 200 && res.status !== 503) throw new Error(`healthz returned ${res.status}`);
  return (await res.json()) as Health;
}

async function getJSON<T>(url: string, signal?: AbortSignal, init?: RequestInit): Promise<T> {
  const res = await fetch(url, { ...init, signal });
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
  if (p.reviewStatus) q.set('review_status', p.reviewStatus);
  if (p.mode === 'blind') q.set('mode', 'blind');
  return getJSON<Queue>(`/api/v1/queue?${q}`, signal);
}

export function getOperators(asOfDay: number, signal?: AbortSignal): Promise<Operators> {
  return getJSON<Operators>(`/api/v1/operators?as_of_day=${asOfDay}`, signal);
}

const modeQ = (mode?: Mode) => (mode === 'blind' ? '&mode=blind' : '');

export function getPlayer(id: string, asOfDay: number, signal?: AbortSignal, mode?: Mode): Promise<PlayerCase> {
  return getJSON<PlayerCase>(`/api/v1/players/${encodeURIComponent(id)}?as_of_day=${asOfDay}${modeQ(mode)}`, signal);
}

export function getTimeline(id: string, asOfDay: number, signal?: AbortSignal, mode?: Mode): Promise<Timeline> {
  return getJSON<Timeline>(
    `/api/v1/players/${encodeURIComponent(id)}/timeline?as_of_day=${asOfDay}${modeQ(mode)}`,
    signal,
  );
}

export function getReviews(
  id: string,
  asOfDay: number,
  signal?: AbortSignal,
  mode?: Mode,
): Promise<{ items: Review[] }> {
  return getJSON(`/api/v1/players/${encodeURIComponent(id)}/reviews?as_of_day=${asOfDay}${modeQ(mode)}`, signal);
}

export function createReview(id: string, body: ReviewCreate): Promise<Review> {
  return getJSON<Review>(`/api/v1/players/${encodeURIComponent(id)}/reviews`, undefined, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
}
