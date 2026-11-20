interface BandSquaresProps {
  high: number;
  elevated: number;
  low: number;
  none: number;
  className: string;
}

export function BandSquares({ high, elevated, low, none, className }: BandSquaresProps) {
  const groups = [
    { cls: 'h', n: high },
    { cls: 'e', n: elevated },
    { cls: 'l', n: low },
    { cls: 'n', n: none },
  ];
  return (
    <div
      className={className}
      role="img"
      aria-label={`${high} high, ${elevated} elevated, ${low} low, ${none} with not enough data`}
    >
      {groups.flatMap((g) =>
        // biome-ignore lint/suspicious/noArrayIndexKey: static squares, never reordered
        Array.from({ length: g.n }, (_, i) => <i key={`${g.cls}${i}`} className={g.cls} />),
      )}
    </div>
  );
}
