import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { BandChip } from './BandChip';

describe('BandChip', () => {
  it.each([
    ['high', 'High', 'high'],
    ['elevated', 'Elevated', 'elevated'],
    ['low', 'Low', 'low'],
    ['insufficient_data', 'Not enough data', 'none'],
  ] as const)('shows text and class for %s', (band, text, cls) => {
    render(<BandChip band={band} />);
    expect(screen.getByText(text)).toHaveClass('chip', cls);
  });
});
