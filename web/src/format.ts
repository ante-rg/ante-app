import type { Outcome } from './api/client';

const eur = new Intl.NumberFormat('en-GB', { maximumFractionDigits: 0 });

export function formatEur(n: number): string {
  return `${eur.format(Math.round(n))} €`;
}

const OUTCOME_LABELS: Record<Outcome, string> = {
  intervention_needed: 'Intervention needed',
  no_concern: 'No concern',
  not_enough_data: 'Not enough data to decide',
};

export function outcomeLabel(o: Outcome, short = false): string {
  return short && o === 'not_enough_data' ? 'Not enough data' : OUTCOME_LABELS[o];
}
