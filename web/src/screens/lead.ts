export type LeadPart = { before: string; bold?: string; after: string; soft?: boolean };

const days = (n: number) => `${n} day${n === 1 ? '' : 's'}`;

export function leadParts(
  firstHigh: number | null,
  firstRule: number | null,
  day: number,
  historyEnded: boolean,
): LeadPart[] {
  const rule: LeadPart =
    firstRule === null
      ? { before: 'No fixed rule has fired yet.', after: '' }
      : { before: 'First fixed rule fired on ', bold: `day ${firstRule}`, after: '.' };
  if (firstHigh === null) {
    const head: LeadPart = { before: `Not in the high band yet on day ${day}.`, after: '' };
    if (firstRule !== null) return [head, rule];
    if (historyEnded) return [head];
    return [
      head,
      { before: 'Move the replay forward to see when the model flags this player.', after: '', soft: true },
    ];
  }
  const model: LeadPart = { before: 'Model flagged high on ', bold: `day ${firstHigh}`, after: '.' };
  if (firstRule === null) return [model, rule];
  if (firstHigh === firstRule) return [model, rule, { before: 'Both on the same day.', after: '' }];
  const modelFirst = firstHigh < firstRule;
  return [
    model,
    rule,
    {
      before: `The ${modelFirst ? 'model' : 'fixed rule'} was `,
      bold: `${days(Math.abs(firstRule - firstHigh))} earlier`,
      after: '.',
    },
  ];
}
