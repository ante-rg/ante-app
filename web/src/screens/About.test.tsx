import { screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { ModelCard } from '../api/client';
import { mockApi, renderApp, replayBody } from '../test/render';

const m = (
  window_days: 7 | 14 | 30,
  split: string,
  method: 'model' | 'rules',
  early_recall: number,
  lead: number | null,
) => ({
  window_days,
  split,
  method,
  early_recall,
  median_lead_days: lead,
});

const card: ModelCard = {
  model_version: 'test-v0',
  synthetic: true,
  trained_at: '2031-02-03T04:05:06Z',
  summary: 'Summary text',
  label_definition: 'Label text',
  intended_use: 'Use text',
  data_protection: 'Protection text',
  ethical_risks: 'Risk text',
  limits: ['Limit one', 'Limit two'],
  default_capacity: 10,
  metrics: [
    m(7, 'all', 'model', 0.424, 12),
    m(7, 'all', 'rules', 0.3, 1),
    m(14, 'all', 'model', 0.5, 4.5),
    m(14, 'all', 'rules', 0.4, null),
  ],
};

const api = (q: { status: number; body: unknown } = { status: 200, body: card }) =>
  mockApi({ '/api/v1/replay': { status: 200, body: replayBody }, '/api/v1/model': q });

const load = async (c: Partial<ModelCard> = {}, url = '/about?day=7') => {
  api({ status: 200, body: { ...card, ...c } });
  renderApp(url);
  await screen.findByText('Model test-v0');
};

describe('AboutScreen', () => {
  it('renders every section and the footer, without trained_at', async () => {
    await load();
    expect(screen.getByRole('heading', { level: 1, name: 'About the score' })).toBeInTheDocument();
    for (const h of [
      'What the score means',
      'How the label is defined',
      'Intended use',
      'Model and rules compared',
      'Known limits',
      'Data protection',
      'Ethical risks',
    ])
      expect(screen.getByRole('heading', { name: h })).toBeInTheDocument();
    for (const t of [
      'Summary text',
      'Label text',
      'Use text',
      'Protection text',
      'Risk text',
      'Limit one',
      'Limit two',
    ])
      expect(screen.getByText(t)).toBeInTheDocument();
    expect(
      screen.getByText(
        'Both methods review 10 players at each assessment point. Early recall is the share of harm-related events whose player was flagged at least the window ahead; lead time is how many days earlier the flag came.',
      ),
    ).toBeInTheDocument();
    expect(document.body.textContent).not.toMatch(/2031/);
  });

  it('shows the synthetic notice only when synthetic is true', async () => {
    await load();
    expect(screen.getByText('Synthetic data. These numbers say nothing about gambling harm.')).toBeInTheDocument();
  });

  it.each([{ synthetic: false }, { synthetic: undefined }])('hides the notice for %j', async (c) => {
    await load(c);
    expect(screen.queryByText(/Synthetic data/)).toBeNull();
  });

  it('puts all first and sorts rows by window', async () => {
    await load({
      metrics: [
        m(30, 'P2', 'model', 0.1, 2),
        m(7, 'P2', 'model', 0.2, 3),
        m(14, 'P2', 'model', 0.3, 4),
        ...card.metrics,
      ],
    });
    const tables = screen.getAllByRole('table');
    expect(tables.map((t) => within(t).getByRole('caption').textContent)).toEqual([
      'All held-out players',
      'Held-out brand P2',
    ]);
    const rows = within(tables[1] as HTMLElement).getAllByRole('rowheader');
    expect(rows.map((r) => r.textContent)).toEqual(['7 days', '14 days', '30 days']);
  });

  it('formats recall and lead time', async () => {
    await load();
    const t = screen.getByRole('table');
    const row = (n: string) =>
      within(within(t).getByRole('rowheader', { name: n }).closest('tr') as HTMLElement)
        .getAllByRole('cell')
        .map((c) => c.textContent);
    expect(row('7 days')).toEqual(['42%', '30%', '12 days', '1 day']);
    expect(row('14 days')[2]).toBe('4.5 days');
  });

  it('shows a dash for missing rows and null lead', async () => {
    await load();
    const t = screen.getByRole('table');
    const cells = (n: string) =>
      within(within(t).getByRole('rowheader', { name: n }).closest('tr') as HTMLElement)
        .getAllByRole('cell')
        .map((c) => c.textContent);
    expect(cells('14 days')[3]).toBe('—');
    expect(cells('30 days')).toEqual(['—', '—', '—', '—']);
  });

  it('renders model-only split with dashes for rules', async () => {
    await load({ metrics: [m(7, 'all', 'model', 0.5, 2)] });
    const cells = within(screen.getByRole('rowheader', { name: '7 days' }).closest('tr') as HTMLElement)
      .getAllByRole('cell')
      .map((c) => c.textContent);
    expect(cells).toEqual(['50%', '—', '2 days', '—']);
  });

  it('scopes the two-row table header cells', async () => {
    await load();
    const t = screen.getByRole('table');
    const models = within(t).getAllByRole('columnheader', { name: 'Model' });
    const rules = within(t).getAllByRole('columnheader', { name: 'Rules' });
    for (const h of [...models, ...rules]) expect(h).toHaveAttribute('scope', 'col');
    expect(within(t).getByRole('columnheader', { name: 'Early recall' })).toHaveAttribute('scope', 'colgroup');
    expect(within(t).getByRole('columnheader', { name: 'Median lead time' })).toHaveAttribute('scope', 'colgroup');
  });

  it('renders card text as plain text', async () => {
    await load({ summary: '<b>x</b>\nline2' });
    expect(screen.getByText(/<b>x<\/b>/).textContent).toContain('\n');
    expect(document.querySelector('b')).toBeNull();
  });

  it('says so when metrics and limits are empty', async () => {
    await load({ metrics: [], limits: [] });
    expect(screen.getByText('No comparison in this model card.')).toBeInTheDocument();
    expect(screen.getByText('None listed.')).toBeInTheDocument();
  });

  it('shows a generic error on 500', async () => {
    api({ status: 500, body: { code: 'internal', message: 'x' } });
    renderApp('/about?day=7');
    expect(await screen.findByRole('alert')).toHaveTextContent("Can't load the model card.");
  });

  it('shows the API message for no_model_card', async () => {
    const message = 'The active model has no card in the current format. Load its artifacts again.';
    api({ status: 404, body: { code: 'no_model_card', message } });
    renderApp('/about?day=7');
    expect(await screen.findByRole('alert')).toHaveTextContent(message);
  });

  it('keeps blind mode in the nav links', async () => {
    await load({}, '/about?day=5&mode=blind');
    expect(screen.getByRole('link', { name: 'Review queue' }).getAttribute('href')).toContain('day=5&mode=blind');
  });
});
