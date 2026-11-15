import { expect, it } from 'vitest';
import { formatEur } from './format';

it('formats euros with grouping and trailing sign', () => {
  expect(formatEur(9840)).toBe('9,840 €');
  expect(formatEur(0)).toBe('0 €');
  expect(formatEur(1234.6)).toBe('1,235 €');
});
