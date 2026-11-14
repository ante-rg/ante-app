import type { Band } from '../api/client';

const BANDS: Record<Band, { cls: string; text: string }> = {
  high: { cls: 'high', text: 'High' },
  elevated: { cls: 'elevated', text: 'Elevated' },
  low: { cls: 'low', text: 'Low' },
  insufficient_data: { cls: 'none', text: 'Not enough data' },
};

export function BandChip({ band }: { band: Band }) {
  const { cls, text } = BANDS[band];
  return <span className={`chip ${cls}`}>{text}</span>;
}
