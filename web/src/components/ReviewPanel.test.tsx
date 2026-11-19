import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ReviewPanel } from './ReviewPanel';

const review = (o: Record<string, unknown> = {}) => ({
  id: 1,
  player_id: 'S01',
  as_of_day: 12,
  outcome: 'no_concern',
  note: null,
  reviewer: 'a@b.co',
  blind: false,
  model_version: 'v',
  created_at: '2026-01-01T00:00:00Z',
  ...o,
});

type Opts = { items?: unknown[]; getStatus?: number; postStatus?: number };
function setup({ items = [], getStatus = 200, postStatus = 201 }: Opts = {}) {
  const fetchMock = vi.fn(async (_url: RequestInfo | URL, init?: RequestInit) => {
    if (init?.method === 'POST') {
      const b = JSON.parse(String(init.body));
      return new Response(JSON.stringify(review({ ...b, as_of_day: 12 })), { status: postStatus });
    }
    return new Response(JSON.stringify(getStatus === 200 ? { items } : { code: 'x', message: 'x' }), {
      status: getStatus,
    });
  });
  vi.stubGlobal('fetch', fetchMock);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <ReviewPanel playerId="S01" day={15} effectiveDay={12} />
    </QueryClientProvider>,
  );
  const posts = () => fetchMock.mock.calls.filter((c) => c[1]?.method === 'POST');
  const gets = () => fetchMock.mock.calls.filter((c) => c[1]?.method !== 'POST');
  return { posts, gets };
}

afterEach(() => vi.unstubAllGlobals());

describe('ReviewPanel', () => {
  it('shows the header, three outcomes and the note field', async () => {
    setup();
    expect(screen.getByRole('heading', { name: 'Your review' })).toBeInTheDocument();
    expect(screen.getByText('Saved with day 12')).toBeInTheDocument();
    expect(screen.getAllByRole('radio')).toHaveLength(3);
    expect(screen.getByLabelText(/Intervention needed/)).toBeInTheDocument();
    expect(screen.getByText('The operator should contact this player.')).toBeInTheDocument();
    expect(screen.getByText('Activity looks controlled.')).toBeInTheDocument();
    expect(screen.getByText('Look again later in the replay.')).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: /Note/ })).toHaveAttribute('maxlength', '2000');
    expect(await screen.findByText('None yet.')).toBeInTheDocument();
  });

  it('asks for an outcome and does not post', async () => {
    const { posts } = setup();
    fireEvent.click(screen.getByRole('button', { name: 'Save review' }));
    expect(screen.getByRole('status')).toHaveTextContent('Choose an outcome to save the review.');
    expect(posts()).toHaveLength(0);
  });

  it('saves, confirms with the effective day, resets and refetches', async () => {
    const { posts, gets } = setup();
    await screen.findByText('None yet.');
    fireEvent.click(screen.getByLabelText(/No concern/));
    fireEvent.change(screen.getByRole('textbox', { name: /Note/ }), { target: { value: 'ok' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save review' }));
    await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('Review saved: No concern, day 12.'));
    expect(JSON.parse(String(posts()[0][1]?.body))).toEqual({
      as_of_day: 15,
      outcome: 'no_concern',
      note: 'ok',
      mode: 'normal',
    });
    expect(screen.getByLabelText(/No concern/)).not.toBeChecked();
    expect(screen.getByRole('textbox', { name: /Note/ })).toHaveValue('');
    await waitFor(() => expect(gets()).toHaveLength(2));
  });

  it('clears the status line when the outcome changes', () => {
    setup();
    fireEvent.click(screen.getByRole('button', { name: 'Save review' }));
    expect(screen.getByRole('status')).toHaveTextContent('Choose an outcome');
    fireEvent.click(screen.getByLabelText(/No concern/));
    expect(screen.getByRole('status')).toBeEmptyDOMElement();
  });

  it('clears the saved status on a day change but keeps an unsent note', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (_u: RequestInfo | URL, init?: RequestInit) =>
        init?.method === 'POST'
          ? new Response(JSON.stringify(review({ as_of_day: 12 })), { status: 201 })
          : new Response(JSON.stringify({ items: [] }), { status: 200 }),
      ),
    );
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const ui = (day: number) => (
      <QueryClientProvider client={client}>
        <ReviewPanel playerId="S01" day={day} effectiveDay={day} />
      </QueryClientProvider>
    );
    const { rerender } = render(ui(12));
    fireEvent.click(screen.getByLabelText(/No concern/));
    fireEvent.click(screen.getByRole('button', { name: 'Save review' }));
    await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('Review saved: No concern, day 12.'));
    fireEvent.change(screen.getByRole('textbox', { name: /Note/ }), { target: { value: 'draft' } });
    fireEvent.click(screen.getByLabelText(/Intervention needed/));
    rerender(ui(13));
    expect(screen.getByRole('status')).toBeEmptyDOMElement();
    expect(screen.getByRole('textbox', { name: /Note/ })).toHaveValue('draft');
    expect(screen.getByLabelText(/Intervention needed/)).toBeChecked();
  });

  it('disables the form while saving', async () => {
    const fetchMock = vi.fn((_u: RequestInfo | URL, init?: RequestInit) =>
      init?.method === 'POST'
        ? new Promise<Response>(() => {})
        : Promise.resolve(new Response(JSON.stringify({ items: [] }), { status: 200 })),
    );
    vi.stubGlobal('fetch', fetchMock);
    render(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <ReviewPanel playerId="S01" day={15} effectiveDay={12} />
      </QueryClientProvider>,
    );
    fireEvent.click(screen.getByLabelText(/No concern/));
    fireEvent.click(screen.getByRole('button', { name: 'Save review' }));
    await waitFor(() => expect(screen.getByRole('textbox', { name: /Note/ })).toBeDisabled());
    for (const r of screen.getAllByRole('radio')) expect(r).toBeDisabled();
  });

  it('omits an empty note', async () => {
    const { posts } = setup();
    fireEvent.click(screen.getByLabelText(/Intervention needed/));
    fireEvent.click(screen.getByRole('button', { name: 'Save review' }));
    await waitFor(() => expect(posts()).toHaveLength(1));
    expect(JSON.parse(String(posts()[0][1]?.body))).not.toHaveProperty('note');
  });

  it('keeps the form when saving fails', async () => {
    setup({ postStatus: 500 });
    fireEvent.click(screen.getByLabelText(/No concern/));
    fireEvent.change(screen.getByRole('textbox', { name: /Note/ }), { target: { value: 'keep me' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save review' }));
    expect(await screen.findByRole('alert')).toHaveTextContent("Can't save the review.");
    expect(screen.getByLabelText(/No concern/)).toBeChecked();
    expect(screen.getByRole('textbox', { name: /Note/ })).toHaveValue('keep me');
  });

  it('lists earlier reviews as text', async () => {
    setup({
      items: [
        review({ id: 2, as_of_day: 9, outcome: 'intervention_needed', note: '<b>x</b>', reviewer: 'z@b.co' }),
        review({ id: 1, as_of_day: 4, outcome: 'not_enough_data', note: 'first' }),
      ],
    });
    const first = await screen.findByText(/<b>x<\/b>/);
    expect(first.closest('li')).toHaveTextContent('Intervention needed');
    expect(first.closest('li')).toHaveTextContent('day 9');
    expect(first.closest('li')).toHaveTextContent('z@b.co');
    expect(first.querySelector('b')).toBeNull();
    const items = screen.getAllByRole('listitem');
    expect(items[0]).toHaveTextContent('day 9');
    expect(items[1]).toHaveTextContent('Not enough data to decide');
    expect(items[1]).toHaveTextContent('first');
  });

  it('says when earlier reviews fail to load', async () => {
    setup({ getStatus: 500 });
    expect(await screen.findByText("Can't load earlier reviews.")).toBeInTheDocument();
  });
});
