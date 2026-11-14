import { render } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { Sparkline } from './Sparkline';

describe('Sparkline', () => {
  it('draws one hidden svg with an end dot', () => {
    const values = Array.from({ length: 30 }, (_, i) => i / 30);
    const { container } = render(<Sparkline values={values} />);
    expect(container.querySelectorAll('svg')).toHaveLength(1);
    expect(container.querySelector('svg')).toHaveAttribute('aria-hidden', 'true');
    expect(container.querySelector('circle')).not.toBeNull();
  });

  it('renders nothing when all values are null', () => {
    const { container } = render(<Sparkline values={[null, null]} />);
    expect(container).toBeEmptyDOMElement();
  });

  it('breaks the line at a null', () => {
    const { container } = render(<Sparkline values={[0.1, null, 0.2, 0.3]} />);
    expect(container.querySelectorAll('polyline')).toHaveLength(2);
  });
});
