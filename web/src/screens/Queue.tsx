import { useQuery } from '@tanstack/react-query';
import { type KeyboardEvent, useState } from 'react';
import { useLocation, useNavigate, useSearchParams } from 'react-router';
import { getQueue, type Queue, type QueueItem } from '../api/client';
import { BandChip } from '../components/BandChip';
import { Sparkline } from '../components/Sparkline';
import { outcomeLabel } from '../format';
import { useReplayDay } from '../replay/ReplayDay';

const OPERATORS = ['P1', 'P2', 'P3', 'P4', 'P5'];
const MAX_CAPACITY = 1000;

function parseCapacity(s: string): number | null {
  const n = Number(s);
  return /^\d+$/.test(s) && n >= 1 && n <= MAX_CAPACITY ? n : null;
}

function playerTotal(c: Queue['counts']): number {
  return (c.high ?? 0) + (c.elevated ?? 0) + (c.low ?? 0) + (c.insufficient_data ?? 0);
}

function Population({ counts, day }: { counts: Queue['counts']; day: number }) {
  const { high = 0, elevated = 0, low = 0, insufficient_data: none = 0, new_high_since_previous_day: fresh } = counts;
  const groups = [
    { cls: 'h', n: high, label: 'high' },
    { cls: 'e', n: elevated, label: 'elevated' },
    { cls: 'l', n: low, label: 'low' },
    { cls: 'n', n: none, label: 'not enough data' },
  ];
  const total = playerTotal(counts);
  return (
    <div className="panel panel-b">
      <div className="panel-h-row">
        <h2>All {total} players by band</h2>
        <span className="example">Each square is one player, ordered by score.</span>
      </div>
      <div
        className="pop"
        role="img"
        aria-label={`${high} high, ${elevated} elevated, ${low} low, ${none} with not enough data`}
      >
        {groups.flatMap((g) =>
          // biome-ignore lint/suspicious/noArrayIndexKey: static squares, never reordered
          Array.from({ length: g.n }, (_, i) => <i key={`${g.cls}${i}`} className={g.cls} />),
        )}
      </div>
      <div className="popkey">
        {groups.map((g) => (
          <span key={g.cls}>
            <i className={g.cls} />
            <strong>{g.n}</strong> {g.label}
          </span>
        ))}
        {day > 0 && fresh !== undefined && (
          <span className="soft">
            <strong>{fresh}</strong> entered high since day {day - 1}
          </span>
        )}
      </div>
    </div>
  );
}

function Row({ item, day, blind, blindCase }: { item: QueueItem; day: number; blind: boolean; blindCase?: number }) {
  const navigate = useNavigate();
  const { search } = useLocation();
  const open = () =>
    navigate(`/players/${encodeURIComponent(item.player_id)}?day=${day}${blind ? '&mode=blind' : ''}`, {
      state: { queueSearch: search },
    });
  const onKeyDown = (e: KeyboardEvent<HTMLTableRowElement>) => {
    if (e.metaKey || e.ctrlKey || e.altKey) return;
    if (e.key === 'Enter') return open();
    const to =
      e.key === 'j'
        ? e.currentTarget.nextElementSibling
        : e.key === 'k'
          ? e.currentTarget.previousElementSibling
          : null;
    if (!(to instanceof HTMLElement)) return;
    e.preventDefault();
    to.focus();
  };
  const band = item.band ?? 'insufficient_data';
  const dayCell = (
    <td className="r">
      {item.effective_day}
      {item.history_ended_on_day != null && (
        <span className="ended">History ends on day {item.history_ended_on_day}</span>
      )}
    </td>
  );
  const reviewCell = (
    <td>
      {item.last_outcome ? (
        <span className="status-ok">{outcomeLabel(item.last_outcome, true)}</span>
      ) : (
        <span className="status-pending">To review</span>
      )}
    </td>
  );
  return (
    <tr className="go" tabIndex={0} aria-label={`Open case ${item.player_id}`} onClick={open} onKeyDown={onKeyDown}>
      {blindCase ? (
        <>
          <td>{blindCase}</td>
          <td className="pid">{item.player_id}</td>
          <td>{item.operator_code}</td>
          {dayCell}
          {reviewCell}
        </>
      ) : (
        <>
          <td className="r">{item.rank ?? ''}</td>
          <td className="pid">{item.player_id}</td>
          <td>{item.operator_code}</td>
          <td>
            <BandChip band={band} />
          </td>
          <td className="r">{item.score == null ? '' : item.score.toFixed(2)}</td>
          <td>{item.score_trend_30d && <Sparkline values={item.score_trend_30d} />}</td>
          <td className="reason">{item.main_reason}</td>
          <td>{item.flagged_by_rules ? 'Also flagged' : <span className="soft">Not flagged</span>}</td>
          {dayCell}
          {reviewCell}
        </>
      )}
    </tr>
  );
}

export function QueueScreen() {
  const { day, replay } = useReplayDay();
  const [params, setParams] = useSearchParams();
  const defaultCapacity = replay.meta.capacity;
  const rawOperator = params.get('operator');
  const operator = OPERATORS.find((o) => o === rawOperator);
  const band = params.get('band') === 'high' ? 'high' : 'high,elevated';
  const rawReview = params.get('review');
  const reviewStatus = rawReview === 'pending' || rawReview === 'reviewed' ? rawReview : undefined;
  const capacity = parseCapacity(params.get('capacity') ?? '') ?? undefined;
  const blind = params.get('mode') === 'blind';
  const [draft, setDraft] = useState<string | null>(null);

  const setParam = (key: string, value: string | null) =>
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        if (value === null) next.delete(key);
        else next.set(key, value);
        return next;
      },
      { replace: true },
    );

  const { data, isError, isPlaceholderData } = useQuery({
    queryKey: ['queue', day, operator, blind ? undefined : band, blind ? undefined : capacity, reviewStatus, blind],
    queryFn: ({ signal }) =>
      getQueue(
        blind
          ? { asOfDay: day, operator, reviewStatus, mode: 'blind' }
          : { asOfDay: day, operator, band, capacity, reviewStatus },
        signal,
      ),
    placeholderData: (prev, prevQuery) =>
      prevQuery && prevQuery.queryKey[6] === blind && (prevQuery.queryKey[1] as number) <= day ? prev : undefined,
  });
  const invalidDraft = draft !== null && parseCapacity(draft) === null;

  const onCapacity = (text: string) => {
    setDraft(text);
    const n = parseCapacity(text);
    setParam('capacity', n === null || n === defaultCapacity ? null : String(n));
  };

  return (
    <section className="screen" aria-labelledby="q-title">
      <div className="qhead">
        <div>
          <h1 id="q-title">Review queue</h1>
          <p className="soft">
            {data && !blind ? `${playerTotal(data.counts)} players` : 'Players'} from 5 operators, each shown {day}{' '}
            {day === 1 ? 'day' : 'days'} after their first activity. Players whose history is shorter are shown at their
            last day.
            {blind ? null : ' The score is a relative priority for human review, not a diagnosis.'}
          </p>
        </div>
      </div>

      {data && !blind && <Population counts={data.counts} day={day} />}

      <div className="panel">
        <div className="filters">
          <label className="field" htmlFor="op">
            Operator
            <select id="op" value={operator ?? ''} onChange={(e) => setParam('operator', e.target.value || null)}>
              <option value="">All operators</option>
              {OPERATORS.map((o) => (
                <option key={o}>{o}</option>
              ))}
            </select>
          </label>
          {!blind && (
            <div className="field">
              Band
              <fieldset className="seg" aria-label="Band">
                <button type="button" aria-pressed={band === 'high,elevated'} onClick={() => setParam('band', null)}>
                  High and elevated
                </button>
                <button type="button" aria-pressed={band === 'high'} onClick={() => setParam('band', 'high')}>
                  High
                </button>
              </fieldset>
            </div>
          )}
          <div className="field">
            Review
            <fieldset className="seg" aria-label="Review status">
              <button type="button" aria-pressed={!reviewStatus} onClick={() => setParam('review', null)}>
                All
              </button>
              <button
                type="button"
                aria-pressed={reviewStatus === 'pending'}
                onClick={() => setParam('review', 'pending')}
              >
                To review
              </button>
              <button
                type="button"
                aria-pressed={reviewStatus === 'reviewed'}
                onClick={() => setParam('review', 'reviewed')}
              >
                Reviewed
              </button>
            </fieldset>
          </div>
          {!blind && (
            <label className="field" htmlFor="top">
              Show top
              <input
                id="top"
                type="number"
                min={1}
                max={MAX_CAPACITY}
                value={draft ?? capacity ?? defaultCapacity}
                onChange={(e) => onCapacity(e.target.value)}
                aria-invalid={invalidDraft || undefined}
                onBlur={() => setDraft(null)}
              />
            </label>
          )}
          <label className="switch" id="blindlbl">
            <button
              type="button"
              role="switch"
              aria-checked={blind}
              aria-labelledby="blindlbl"
              onClick={() => setParam('mode', blind ? null : 'blind')}
            />
            Blind review
          </label>
        </div>
        {blind && (
          <p className="blindnote">
            Blind review shows the fixed review sample in random order. The server leaves out score, band, rank and
            reasons, so you judge each case from its activity alone.
          </p>
        )}

        {isError && !data ? (
          <p className="placeholder" role="alert">
            Can't load the queue.
          </p>
        ) : (
          <>
            {isError && (
              <p className="more soft" role="alert">
                Can't refresh the queue. Showing the last loaded rows.
              </p>
            )}
            <div className="tablewrap" aria-busy={isPlaceholderData}>
              <table>
                <thead>
                  {blind ? (
                    <tr>
                      <th>Case</th>
                      <th>Player</th>
                      <th>Operator</th>
                      <th className="r">Day shown</th>
                      <th>Your review</th>
                    </tr>
                  ) : (
                    <tr>
                      <th className="r">Rank</th>
                      <th>Player</th>
                      <th>Operator</th>
                      <th>Band</th>
                      <th className="r">Score</th>
                      <th>Last 30 days</th>
                      <th>Main reason</th>
                      <th>Rules</th>
                      <th className="r">Day shown</th>
                      <th>Review</th>
                    </tr>
                  )}
                </thead>
                <tbody>
                  {data?.items.map((item, i) => (
                    <Row
                      key={item.player_id}
                      item={item}
                      day={day}
                      blind={blind}
                      blindCase={blind ? i + 1 : undefined}
                    />
                  ))}
                  {!data && (
                    <tr>
                      <td colSpan={blind ? 5 : 10} className="soft">
                        Loading the queue…
                      </td>
                    </tr>
                  )}
                  {data?.items.length === 0 && (
                    <tr>
                      <td colSpan={blind ? 5 : 10} className="soft">
                        {blind && !operator && !reviewStatus
                          ? 'No review sample is loaded for this model.'
                          : reviewStatus || blind
                            ? 'No players match these filters. Choose "All operators" or another review status.'
                            : 'No players match these filters. Choose "All operators" or another band.'}
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
            {data && blind && (
              <p className="more soft">
                {data.items.length} {data.items.length === 1 ? 'case' : 'cases'} in the review sample shown.
              </p>
            )}
            {data && !blind && (
              <p className="more soft">
                {data.items.length} {data.items.length === 1 ? 'player' : 'players'} shown. Top {data.meta.capacity} by
                review capacity.
              </p>
            )}
          </>
        )}
      </div>
    </section>
  );
}
