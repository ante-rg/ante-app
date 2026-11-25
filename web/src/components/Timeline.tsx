import { scaleLinear } from '@visx/scale';
import { Bar, LinePath } from '@visx/shape';
import { useId } from 'react';
import type { TimelineDay, TimelineScore } from '../api/client';
import { formatEur } from '../format';

export type TimelineProps = {
  playerId: string;
  day: number;
  historyEndedOnDay: number | null;
  firstHighDay: number | null;
  firstRuleDay: number | null;
  scores?: TimelineScore[];
  days: TimelineDay[];
};

const X0 = 48;
const X1 = 984;
const S_TOP = 34;
const S_BOT = 128;
const B_TOP = 136;
const B_BOT = 146;
const D_TOP = 168;
const D_BOT = 290;
const MIN_DAYS = 30;
const DEP_STEP = 250;
const LABEL_DROP_GAP = 120;
const LABEL_DROP = 14;
const END_ANCHOR_X = 904;

const xTickStep = (day: number) => (day <= 80 ? 10 : day <= 200 ? 20 : 50);

function range(to: number, step: number) {
  const out: number[] = [];
  for (let t = 0; t <= to; t += step) out.push(t);
  return out;
}

export function TimelineLegend({ blind }: { blind?: boolean }) {
  return (
    <div className="tl-legend">
      {!blind && (
        <>
          <span>
            <svg width="18" height="10" aria-hidden="true">
              <line x1="0" y1="5" x2="18" y2="5" stroke="currentColor" strokeWidth="2" />
            </svg>
            Score
          </span>
          <span>
            <i className="sw high" />
            High band
          </span>
          <span>
            <i className="sw elevated" />
            Elevated band
          </span>
        </>
      )}
      <span>
        <i className="sw dep" />
        Deposits per day
      </span>
      <span>
        <i className="sw chase" />
        Day with a loss-chasing session
      </span>
    </div>
  );
}

export function Timeline({
  playerId,
  day,
  historyEndedOnDay,
  firstHighDay,
  firstRuleDay,
  scores,
  days,
}: TimelineProps) {
  const id = useId();
  const x = scaleLinear({ domain: [-0.5, Math.max(day, MIN_DAYS) + 0.5], range: [X0, X1] });
  const ys = scaleLinear({ domain: [0, 1], range: [S_BOT, S_TOP] });
  const bars = days.filter((d) => (d.deposits_eur ?? 0) > 0);
  const maxEur = Math.max(0, ...bars.map((d) => d.deposits_eur ?? 0));
  const depMax = Math.max(DEP_STEP, Math.ceil(maxEur / DEP_STEP) * DEP_STEP);
  const yd = scaleLinear({ domain: [0, depMax], range: [D_BOT, D_TOP] });
  const step = x(1) - x(0);
  const barW = Math.max(1.2, step * 0.7);
  const cutX = x(day + 0.5);
  const cutAnchor = day === 0 ? 'start' : cutX >= END_ANCHOR_X ? 'end' : 'middle';
  const dropRule =
    firstHighDay !== null && firstRuleDay !== null && Math.abs(x(firstRuleDay) - x(firstHighDay)) < LABEL_DROP_GAP;
  const desc = scores
    ? `Score and deposits per day for player ${playerId} from day 0 to day ${day}. ` +
      `${firstHighDay === null ? 'Not in the high band.' : `High band from day ${firstHighDay}.`} ` +
      `${firstRuleDay === null ? 'No fixed rule.' : `First fixed rule on day ${firstRuleDay}.`}`
    : `Deposits per day for player ${playerId} from day 0 to day ${day}.`;

  const top = scores ? S_TOP : D_TOP - 40;
  return (
    <svg
      className="timeline"
      viewBox={scores ? '0 0 1000 330' : `0 ${top - 30} 1000 ${330 - top + 30}`}
      role="img"
      aria-labelledby={`${id}-t`}
      aria-describedby={`${id}-d`}
    >
      <title id={`${id}-t`}>Activity timeline</title>
      <desc id={`${id}-d`}>{desc}</desc>
      {scores && (
        <>
          <text className="lbl" x={X0 - 8} y={20} textAnchor="end">
            Score
          </text>
          {[0, 0.5, 1].map((t) => (
            <text key={t} x={X0 - 8} y={ys(t) + 4} textAnchor="end">
              {t === 0 ? '0' : t.toFixed(1)}
            </text>
          ))}
          <line className="axis" x1={X0} y1={S_BOT} x2={X1} y2={S_BOT} />
          <LinePath
            className="score-line"
            data={scores}
            defined={(s) => s.score !== null}
            x={(s) => x(s.day)}
            y={(s) => ys(s.score ?? 0)}
          />
          {scores.map((s, i) =>
            s.score !== null && scores[i - 1]?.score == null && scores[i + 1]?.score == null ? (
              <circle key={s.day} className="score-dot" r={3} cx={x(s.day)} cy={ys(s.score)} fill="currentColor" />
            ) : null,
          )}
          <text x={X0 - 8} y={(B_TOP + B_BOT) / 2 + 4} textAnchor="end">
            Band
          </text>
          {scores.map((s) =>
            s.band === 'high' || s.band === 'elevated' ? (
              <rect
                key={s.day}
                className={`band-cell ${s.band}`}
                x={x(s.day - 0.5)}
                width={step}
                y={B_TOP}
                height={B_BOT - B_TOP}
              />
            ) : null,
          )}
        </>
      )}
      <text className="lbl" x={X0} y={D_TOP - 14}>
        Deposits per day
      </text>
      {[0, depMax / 2, depMax].map((t) => (
        <g key={t}>
          <line className={t ? 'grid' : 'axis'} x1={X0} y1={yd(t)} x2={X1} y2={yd(t)} />
          <text x={X0 - 8} y={yd(t) + 4} textAnchor="end">
            {formatEur(t)}
          </text>
        </g>
      ))}
      {bars.map((d) => (
        <Bar
          key={d.day}
          className={(d.chase_sessions_n ?? 0) > 0 ? 'dep chase' : 'dep'}
          x={x(d.day) - barW / 2}
          width={barW}
          y={yd(d.deposits_eur ?? 0)}
          height={D_BOT - yd(d.deposits_eur ?? 0)}
        />
      ))}
      {range(day, xTickStep(day)).map((t) => (
        <text key={t} x={x(t)} y={D_BOT + 18} textAnchor="middle">
          {t}
        </text>
      ))}
      <text x={X1} y={D_BOT + 36} textAnchor="end">
        Day since first activity
      </text>
      {firstHighDay !== null && (
        <g className="marker-high">
          <line x1={x(firstHighDay)} y1={S_TOP - 6} x2={x(firstHighDay)} y2={D_BOT} />
          <text
            x={x(firstHighDay) + (x(firstHighDay) >= END_ANCHOR_X ? -5 : 5)}
            y={S_TOP - 6}
            textAnchor={x(firstHighDay) >= END_ANCHOR_X ? 'end' : 'start'}
          >
            High from day {firstHighDay}
          </text>
        </g>
      )}
      {firstRuleDay !== null && (
        <g className="marker-rule">
          <line x1={x(firstRuleDay)} y1={S_TOP - 6} x2={x(firstRuleDay)} y2={D_BOT} />
          <text
            x={x(firstRuleDay) + (x(firstRuleDay) >= END_ANCHOR_X ? -5 : 5)}
            y={S_TOP - 6 + (dropRule ? LABEL_DROP : 0)}
            textAnchor={x(firstRuleDay) >= END_ANCHOR_X ? 'end' : 'start'}
          >
            First rule, day {firstRuleDay}
          </text>
        </g>
      )}
      <g className="cutoff">
        <line x1={cutX} y1={top - 14} x2={cutX} y2={D_BOT} />
        <text x={cutX} y={top - 20} textAnchor={cutAnchor}>
          {historyEndedOnDay === null ? `Day ${day}` : `History ends on day ${historyEndedOnDay}`}
        </text>
      </g>
    </svg>
  );
}
