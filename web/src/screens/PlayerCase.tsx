import { useQuery } from '@tanstack/react-query';
import { useRef } from 'react';
import { Link, useLocation, useParams } from 'react-router';
import { ApiError, getPlayer, getTimeline, type PlayerCase } from '../api/client';
import { BandChip } from '../components/BandChip';
import { ReviewPanel } from '../components/ReviewPanel';
import { Timeline, TimelineLegend } from '../components/Timeline';
import { formatEur } from '../format';
import { useReplayDay } from '../replay/ReplayDay';
import { leadParts } from './lead';

type CrumbState = { queueSearch?: string } | null;

function Crumb({ id, day }: { id: string; day: number }) {
  const state = useLocation().state as CrumbState;
  const q = new URLSearchParams(state?.queueSearch ?? '');
  q.set('day', String(day));
  return (
    <p className="crumb">
      <Link to={`/?${q}`}>Review queue</Link> <span className="soft">/ {id}</span>
    </p>
  );
}

function Reasons({ p }: { p: PlayerCase }) {
  const reasons = p.reasons ?? [];
  const rules = p.rule_flags ?? [];
  return (
    <div className="panel">
      <div className="panel-h">
        <h2>Why this player is flagged</h2>
        <span className="example">Share of the score</span>
      </div>
      <div className="panel-b">
        {reasons.length === 0 ? (
          <p className="soft">No reasons for this day. The score needs more history.</p>
        ) : (
          <ul className="reasons">
            {reasons.map((r) => (
              <li key={r.code}>
                <span>{r.text}</span>
                <span className="wbar">
                  <span style={{ width: `${r.weight * 100}%` }} />
                </span>
                <span className="soft">{Math.round(r.weight * 100)}%</span>
              </li>
            ))}
          </ul>
        )}
        <div className="rules">
          <strong>Fixed rules</strong>
          {rules.length === 0 ? (
            <p className="soft">No fixed rule has fired up to day {p.effective_day}.</p>
          ) : (
            <ul>
              {rules.map((r) => (
                <li key={r.rule_id}>
                  {r.text}, first fired on day {r.first_day}
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </div>
  );
}

export function PlayerCaseScreen() {
  const id = useParams().playerId ?? '';
  const { day } = useReplayDay();
  const placeholderData = <T,>(prev: T | undefined, q: { queryKey: readonly unknown[] } | undefined) =>
    q && q.queryKey[1] === id && (q.queryKey[2] as number) <= day ? prev : undefined;
  const player = useQuery({
    queryKey: ['player', id, day],
    queryFn: ({ signal }) => getPlayer(id, day, signal),
    placeholderData,
  });
  const timeline = useQuery({
    queryKey: ['timeline', id, day],
    queryFn: ({ signal }) => getTimeline(id, day, signal),
    placeholderData,
  });
  const lastP = useRef<{ id: string; day: number; data: PlayerCase }>(null);
  const lastTl = useRef<{ id: string; day: number; data: NonNullable<typeof timeline.data> }>(null);
  if (player.data && !player.isPlaceholderData) lastP.current = { id, day, data: player.data };
  if (timeline.data && !timeline.isPlaceholderData) lastTl.current = { id, day, data: timeline.data };
  const kept = <T,>(r: { id: string; day: number; data: T } | null) =>
    r && r.id === id && r.day <= day ? r.data : undefined;
  const notFound = player.error instanceof ApiError && player.error.code === 'not_found';
  const p = player.data ?? (player.isError && !notFound ? kept(lastP.current) : undefined);
  const tl = timeline.data ?? (timeline.isError ? kept(lastTl.current) : undefined);

  if (!p) {
    return (
      <section className="screen" aria-labelledby="c-title">
        <Crumb id={id} day={day} />
        <h1 id="c-title">Player {id}</h1>
        {notFound ? (
          <p className="placeholder">Player {id} is not in this dataset.</p>
        ) : player.isError ? (
          <p className="placeholder" role="alert">
            Can't load this case.
          </p>
        ) : (
          <p className="placeholder">Loading the case…</p>
        )}
      </section>
    );
  }

  const e = p.effective_day;
  return (
    <section className="screen" aria-labelledby="c-title" aria-busy={player.isPlaceholderData}>
      <Crumb id={id} day={day} />
      {player.isError && (
        <p className="more soft" role="alert">
          Can't refresh this case. Showing the last loaded day.
        </p>
      )}
      <div className="case-h">
        <div>
          <h1 id="c-title">Player {p.player_id}</h1>
          <p className="soft">
            Operator {p.operator_code}. Day {e}. {p.sessions_to_date} sessions and {p.deposits_to_date} deposits so far,{' '}
            {formatEur(p.deposits_eur_to_date ?? 0)} deposited.
            {p.history_ended_on_day != null && ` History ends on day ${p.history_ended_on_day}.`}
          </p>
        </div>
        <div className="scorebox">
          <BandChip band={p.band ?? 'insufficient_data'} />
          {p.score != null && (
            <>
              <span className="score">{p.score.toFixed(2)}</span>
              {p.rank != null && (
                <span className="soft">
                  Rank {p.rank} of {p.rank_of}
                </span>
              )}
            </>
          )}
          <span className="soft scorebox-caption">Relative priority for review</span>
        </div>
      </div>

      <div className="lead">
        {leadParts(p.first_high_day ?? null, p.first_rule_flag_day ?? null, e, p.history_ended_on_day != null).map(
          (part) => (
            <span key={part.before + (part.bold ?? '')} className={part.soft ? 'soft' : undefined}>
              {part.before}
              {part.bold && <strong>{part.bold}</strong>}
              {part.after}
            </span>
          ),
        )}
      </div>

      <div className="panel">
        <div className="panel-h">
          <h2>Activity from day 0 to day {e}</h2>
          <TimelineLegend />
        </div>
        <div className="panel-b">
          {tl && tl.as_of_day === p.as_of_day ? (
            <Timeline
              playerId={p.player_id}
              day={e}
              historyEndedOnDay={p.history_ended_on_day ?? null}
              firstHighDay={p.first_high_day ?? null}
              firstRuleDay={p.first_rule_flag_day ?? null}
              scores={tl.scores}
              days={tl.days}
            />
          ) : timeline.isError ? (
            <p className="soft" role="alert">
              Can't load the timeline.
            </p>
          ) : (
            <p className="soft">Loading the timeline…</p>
          )}
        </div>
      </div>

      <div className="grid2">
        <Reasons p={p} />
        <ReviewPanel key={p.player_id} playerId={p.player_id} day={p.as_of_day} effectiveDay={p.effective_day} />
      </div>
    </section>
  );
}
