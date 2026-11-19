import { expect, it } from 'vitest';
import { formatEur, outcomeLabel } from './format';

it('formats euros with grouping and trailing sign', () => {
  expect(formatEur(9840)).toBe('9,840 €');
  expect(formatEur(0)).toBe('0 €');
  expect(formatEur(1234.6)).toBe('1,235 €');
});

it('labels outcomes, shortening only not enough data', () => {
  expect(outcomeLabel('intervention_needed')).toBe('Intervention needed');
  expect(outcomeLabel('no_concern')).toBe('No concern');
  expect(outcomeLabel('not_enough_data')).toBe('Not enough data to decide');
  expect(outcomeLabel('not_enough_data', true)).toBe('Not enough data');
  expect(outcomeLabel('intervention_needed', true)).toBe('Intervention needed');
  expect(outcomeLabel('no_concern', true)).toBe('No concern');
});
