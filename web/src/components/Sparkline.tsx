const W = 96;
const H = 24;

type Point = [number, number];

export function Sparkline({ values }: { values: (number | null)[] }) {
  const present = values.filter((v): v is number => v !== null);
  if (present.length === 0) return null;
  const lo = Math.min(...present) - 0.02;
  const hi = Math.max(...present) + 0.02;
  const last = values.length - 1 || 1;
  const segments: Point[][] = [[]];
  for (const [i, v] of values.entries()) {
    if (v === null) segments.push([]);
    else segments[segments.length - 1].push([(i / last) * (W - 4) + 2, H - 2 - ((v - lo) / (hi - lo)) * (H - 4)]);
  }
  const lastSeg = segments.filter((s) => s.length).at(-1) as Point[];
  const [cx, cy] = lastSeg[lastSeg.length - 1];
  return (
    <svg className="spark" width={W} height={H} viewBox={`0 0 ${W} ${H}`} aria-hidden="true">
      {segments
        .filter((s) => s.length)
        .map((s) => (
          <polyline
            key={s[0][0]}
            points={s.map((p) => p.join(',')).join(' ')}
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
          />
        ))}
      <circle className="end" cx={cx} cy={cy} r="2.5" />
    </svg>
  );
}
