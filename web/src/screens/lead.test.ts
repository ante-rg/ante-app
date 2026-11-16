import { describe, expect, it } from 'vitest';
import { type LeadPart, leadParts } from './lead';

const md = (ps: LeadPart[]) => ps.map((p) => p.before + (p.bold ? `**${p.bold}**` : '') + p.after).join(' ');

describe('leadParts', () => {
  it('neither fired', () => {
    expect(md(leadParts(null, null, 120, false))).toBe(
      'Not in the high band yet on day 120. Move the replay forward to see when the model flags this player.',
    );
    expect(leadParts(null, null, 120, false)[1].soft).toBe(true);
  });
  it('drops the soft hint once the history has ended', () => {
    expect(md(leadParts(null, null, 3, true))).toBe('Not in the high band yet on day 3.');
  });
  it('rule only', () => {
    expect(md(leadParts(null, 97, 120, false))).toBe(
      'Not in the high band yet on day 120. First fixed rule fired on **day 97**.',
    );
  });
  it('model only', () => {
    expect(md(leadParts(41, null, 120, false))).toBe('Model flagged high on **day 41**. No fixed rule has fired yet.');
  });
  it('model earlier', () => {
    expect(md(leadParts(41, 97, 120, false))).toBe(
      'Model flagged high on **day 41**. First fixed rule fired on **day 97**. The model was **56 days earlier**.',
    );
    expect(md(leadParts(41, 42, 120, false))).toContain('**1 day earlier**');
  });
  it('rule earlier', () => {
    expect(md(leadParts(97, 41, 120, false))).toBe(
      'Model flagged high on **day 97**. First fixed rule fired on **day 41**. The fixed rule was **56 days earlier**.',
    );
  });
  it('same day', () => {
    expect(md(leadParts(41, 41, 120, false))).toBe(
      'Model flagged high on **day 41**. First fixed rule fired on **day 41**. Both on the same day.',
    );
  });
});
