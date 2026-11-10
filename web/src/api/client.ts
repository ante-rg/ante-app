import type { components } from './schema';

export type Health = components['schemas']['Health'];

export async function getHealth(): Promise<Health> {
  const res = await fetch('/healthz');
  if (res.status !== 200 && res.status !== 503) throw new Error(`healthz returned ${res.status}`);
  return (await res.json()) as Health;
}
