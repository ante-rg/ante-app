import { render } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { Band, TimelineDay, TimelineScore } from '../api/client';
import { Timeline, TimelineLegend, type TimelineProps } from './Timeline';

const bands: (Band | undefined)[] = ['low', 'elevated', 'high', 'high', 'insufficient_data'];
const scores: TimelineScore[] = bands.map((band, day) => ({
  day,
  score: band === 'insufficient_data' ? null : 0.2 * day,
  band,
}));
const days: TimelineDay[] = [
  { day: 1, deposits_eur: 0 },
  { day: 2, deposits_eur: 300 },
  { day: 3, deposits_eur: 500, chase_sessions_n: 1 },
];
const base: TimelineProps = {
  playerId: 'S01',
  day: 4,
  historyEndedOnDay: null,
  firstHighDay: 2,
  firstRuleDay: 3,
  scores,
  days,
};

const setup = (p: Partial<TimelineProps> = {}) => render(<Timeline {...base} {...p} />).container;

describe('Timeline without scores', () => {
  const blind = () => setup({ scores: undefined, firstHighDay: null, firstRuleDay: null });

  it('draws deposit bars but no score line, dots or band cells', () => {
    const c = blind();
    expect(c.querySelectorAll('rect.dep')).toHaveLength(2);
    expect(c.querySelector('path.score-line')).toBeNull();
    expect(c.querySelector('.score-dot')).toBeNull();
    expect(c.querySelector('.band-cell')).toBeNull();
  });

  it('crops the viewBox above the deposits block, full height with scores', () => {
    const y = (c: Element) => Number(c.querySelector('svg')?.getAttribute('viewBox')?.split(' ')[1]);
    expect(y(blind())).toBeGreaterThan(0);
    expect(y(setup())).toBe(0);
  });

  it('has no band sentence in desc', () => {
    expect(blind().querySelector('desc')?.textContent).toBe('Deposits per day for player S01 from day 0 to day 4.');
  });

  it('lists only the deposit entries in the blind legend', () => {
    const { container } = render(<TimelineLegend blind />);
    expect(container.querySelectorAll('span')).toHaveLength(2);
    expect(container.textContent).toBe('Deposits per dayDay with a loss-chasing session');
  });
});

describe('Timeline', () => {
  it('draws one band cell per high or elevated day', () => {
    const c = setup();
    expect(c.querySelectorAll('rect.band-cell.elevated')).toHaveLength(1);
    expect(c.querySelectorAll('rect.band-cell.high')).toHaveLength(2);
  });

  it('draws deposit bars only for days with deposits, chase days marked', () => {
    const c = setup();
    expect(c.querySelectorAll('rect.dep')).toHaveLength(2);
    expect(c.querySelectorAll('rect.dep.chase')).toHaveLength(1);
  });

  it('draws each marker only when its day is set', () => {
    const c = setup({ firstRuleDay: null });
    expect(c.querySelector('.marker-high text')?.textContent).toBe('High from day 2');
    expect(c.querySelector('.marker-rule')).toBeNull();
    expect(setup().querySelector('.marker-rule text')?.textContent).toBe('First rule, day 3');
  });

  it('describes the data in desc', () => {
    expect(setup().querySelector('desc')?.textContent).toBe(
      'Score and deposits per day for player S01 from day 0 to day 4. High band from day 2. First fixed rule on day 3.',
    );
    const none = setup({ firstHighDay: null, firstRuleDay: null }).querySelector('desc')?.textContent;
    expect(none).toMatch(/Not in the high band\. No fixed rule\.$/);
  });

  it('labels the cut-off', () => {
    expect(setup().querySelector('.cutoff text')?.textContent).toBe('Day 4');
    expect(setup({ historyEndedOnDay: 4 }).querySelector('.cutoff text')?.textContent).toBe('History ends on day 4');
  });

  it('renders an empty history at day 0 without NaN', () => {
    const c = setup({
      day: 0,
      scores: [{ day: 0, score: null, band: 'insufficient_data' }],
      days: [],
      firstHighDay: null,
      firstRuleDay: null,
    });
    expect(c.querySelector('.band-cell')).toBeNull();
    expect(c.querySelector('.cutoff text')?.getAttribute('text-anchor')).toBe('start');
    expect(c.innerHTML).not.toContain('NaN');
  });

  it('anchors the cut-off label by position', () => {
    expect(setup({ day: 30 }).querySelector('.cutoff text')?.getAttribute('text-anchor')).toBe('end');
    expect(setup({ day: 10 }).querySelector('.cutoff text')?.getAttribute('text-anchor')).toBe('middle');
  });

  it('anchors marker labels by position', () => {
    const edge = setup({ day: 120, firstHighDay: 120, firstRuleDay: 120 });
    expect(edge.querySelector('.marker-high text')?.getAttribute('text-anchor')).toBe('end');
    expect(edge.querySelector('.marker-rule text')?.getAttribute('text-anchor')).toBe('end');
    const early = setup({ day: 120, firstHighDay: 2, firstRuleDay: 2 });
    expect(early.querySelector('.marker-high text')?.getAttribute('text-anchor')).toBe('start');
    expect(early.querySelector('.marker-rule text')?.getAttribute('text-anchor')).toBe('start');
  });

  it('puts the Score label in the left gutter', () => {
    const label = [...setup().querySelectorAll('svg.timeline text')].find((t) => t.textContent === 'Score');
    expect(label?.getAttribute('text-anchor')).toBe('end');
    expect(Number(label?.getAttribute('x'))).toBeLessThan(48);
  });

  it('keeps band cells and deposit bars inside the plot', () => {
    const c = setup({
      day: 4,
      scores: [
        { day: 0, score: 0.9, band: 'high' },
        { day: 4, score: 0.9, band: 'high' },
      ],
      days: [
        { day: 0, deposits_eur: 100 },
        { day: 4, deposits_eur: 100 },
      ],
    });
    const cutX = Number(c.querySelector('.cutoff line')?.getAttribute('x1'));
    const rects = [...c.querySelectorAll('rect.band-cell'), ...c.querySelectorAll('rect.dep')];
    expect(rects).toHaveLength(4);
    for (const r of rects) {
      const x = Number(r.getAttribute('x'));
      expect(x).toBeGreaterThanOrEqual(48);
      expect(x + Number(r.getAttribute('width'))).toBeLessThanOrEqual(cutX + 0.01);
    }
    const dep0 = setup({ day: 0, scores: [], days: [{ day: 0, deposits_eur: 100 }] }).querySelector('rect.dep');
    expect(Number(dep0?.getAttribute('width'))).toBeGreaterThan(0);
  });

  it('draws a dot only for an isolated score', () => {
    const s = (score: number | null, day: number): TimelineScore => ({
      day,
      score,
      band: score === null ? 'insufficient_data' : 'low',
    });
    expect(setup({ day: 2, scores: [s(null, 0), s(0.5, 1), s(null, 2)] }).querySelectorAll('.score-dot')).toHaveLength(
      1,
    );
    expect(setup({ day: 2, scores: [s(null, 0), s(0.5, 1), s(0.6, 2)] }).querySelectorAll('.score-dot')).toHaveLength(
      0,
    );
  });

  it('shows the legend', () => {
    const t = render(<TimelineLegend />).container.querySelector('.tl-legend')?.textContent ?? '';
    expect(t).not.toBe('');
    for (const s of ['Score', 'High band', 'Elevated band', 'Deposits per day', 'Day with a loss-chasing session'])
      expect(t).toContain(s);
  });
});
