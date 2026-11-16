const eur = new Intl.NumberFormat('en-GB', { maximumFractionDigits: 0 });

export function formatEur(n: number): string {
  return `${eur.format(Math.round(n))} €`;
}
