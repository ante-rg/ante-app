import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { type FormEvent, useEffect, useId, useState } from 'react';
import { createReview, getReviews, type Outcome, type ReviewCreate } from '../api/client';
import { outcomeLabel } from '../format';

const OPTIONS: { value: Outcome; hint: string }[] = [
  { value: 'intervention_needed', hint: 'The operator should contact this player.' },
  { value: 'no_concern', hint: 'Activity looks controlled.' },
  { value: 'not_enough_data', hint: 'Look again later in the replay.' },
];

type Props = { playerId: string; day: number; effectiveDay: number };

export function ReviewPanel({ playerId, day, effectiveDay }: Props) {
  const uid = useId();
  const queryClient = useQueryClient();
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const [note, setNote] = useState('');
  const [status, setStatus] = useState('');
  const reviews = useQuery({
    queryKey: ['reviews', playerId, day],
    queryFn: ({ signal }) => getReviews(playerId, day, signal),
  });
  const save = useMutation({
    mutationFn: (body: ReviewCreate) => createReview(playerId, body),
    onSuccess: (r) => {
      setStatus(`Review saved: ${outcomeLabel(r.outcome)}, day ${r.as_of_day}.`);
      setOutcome(null);
      setNote('');
      void queryClient.invalidateQueries({ queryKey: ['reviews', playerId] });
      void queryClient.invalidateQueries({ queryKey: ['queue'] });
    },
  });

  // biome-ignore lint/correctness/useExhaustiveDependencies: day is the trigger, not a read value
  useEffect(() => setStatus(''), [day]);

  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (!outcome) return setStatus('Choose an outcome to save the review.');
    setStatus('');
    save.mutate({ as_of_day: day, outcome, mode: 'normal', ...(note && { note }) });
  };

  const items = reviews.data?.items ?? [];
  return (
    <form className="panel" onSubmit={submit} noValidate>
      <div className="panel-h">
        <h2>Your review</h2>
        <span className="example">Saved with day {effectiveDay}</span>
      </div>
      <div className="panel-b">
        <fieldset className="opts" aria-label="Outcome" disabled={save.isPending}>
          {OPTIONS.map((o) => (
            <label className="opt" key={o.value}>
              <input
                type="radio"
                name="outcome"
                id={`${uid}-${o.value}`}
                value={o.value}
                checked={outcome === o.value}
                onChange={() => {
                  setOutcome(o.value);
                  setStatus('');
                }}
              />
              <span>
                <strong>{outcomeLabel(o.value)}</strong>
                <small>{o.hint}</small>
              </span>
            </label>
          ))}
        </fieldset>
        <label htmlFor={`${uid}-note`} style={{ display: 'block', marginTop: 'var(--s3)', fontSize: 'var(--text-sm)' }}>
          Note <span className="soft">(optional)</span>
        </label>
        <textarea
          id={`${uid}-note`}
          className="review-note"
          maxLength={2000}
          placeholder="What did you see in the activity?"
          value={note}
          disabled={save.isPending}
          onChange={(e) => {
            setNote(e.target.value);
            setStatus('');
          }}
        />
        <div className="formrow">
          <button className="btn primary" type="submit" disabled={save.isPending}>
            Save review
          </button>
          <span role="status" className="soft">
            {status}
          </span>
        </div>
        {save.isError && <p role="alert">Can't save the review.</p>}
        <div className="history">
          <strong>Earlier reviews</strong>
          {reviews.isError ? (
            <p className="soft">Can't load earlier reviews.</p>
          ) : items.length === 0 ? (
            <p className="soft">{reviews.isPending ? 'Loading…' : 'None yet.'}</p>
          ) : (
            <ul>
              {items.map((r) => (
                <li key={r.id}>
                  {outcomeLabel(r.outcome)}, day {r.as_of_day}, {r.reviewer}
                  {r.note && <span className="soft"> {r.note}</span>}
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </form>
  );
}
