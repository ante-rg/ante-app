import { useQuery } from '@tanstack/react-query';
import { ApiError, getModelCard, type ModelCard, type ModelMetric } from '../api/client';

const WINDOWS = [7, 14, 30];

const pct = (m?: ModelMetric) => (m ? `${Math.round(m.early_recall * 100)}%` : '—');

function lead(m?: ModelMetric) {
  const d = m?.median_lead_days;
  if (d == null) return '—';
  if (d === 1) return '1 day';
  return Number.isInteger(d) ? `${d} days` : `${d.toFixed(1)} days`;
}

function Metrics({ card }: { card: ModelCard }) {
  if (card.metrics.length === 0) return <p className="soft">No comparison in this model card.</p>;
  const splits = [...new Set(card.metrics.map((m) => m.split))].sort(
    (a, b) => Number(b === 'all') - Number(a === 'all'),
  );
  return (
    <>
      <p>
        Both methods review {card.default_capacity} players at each assessment point. Early recall is the share of
        harm-related events whose player was flagged at least the window ahead; lead time is how many days earlier the
        flag came.
      </p>
      {splits.map((split) => {
        const find = (w: number, method: ModelMetric['method']) =>
          card.metrics.find((m) => m.split === split && m.window_days === w && m.method === method);
        return (
          <div className="tablewrap" key={split}>
            <table>
              <caption>{split === 'all' ? 'All held-out players' : `Held-out brand ${split}`}</caption>
              <thead>
                <tr>
                  <th rowSpan={2} scope="col">
                    Window
                  </th>
                  <th colSpan={2} scope="colgroup" className="r">
                    Early recall
                  </th>
                  <th colSpan={2} scope="colgroup" className="r">
                    Median lead time
                  </th>
                </tr>
                <tr>
                  <th scope="col" className="r">
                    Model
                  </th>
                  <th scope="col" className="r">
                    Rules
                  </th>
                  <th scope="col" className="r">
                    Model
                  </th>
                  <th scope="col" className="r">
                    Rules
                  </th>
                </tr>
              </thead>
              <tbody>
                {WINDOWS.map((w) => (
                  <tr key={w}>
                    <th scope="row">{w} days</th>
                    <td className="r">{pct(find(w, 'model'))}</td>
                    <td className="r">{pct(find(w, 'rules'))}</td>
                    <td className="r">{lead(find(w, 'model'))}</td>
                    <td className="r">{lead(find(w, 'rules'))}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        );
      })}
    </>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="panel about-sec">
      <h2>{title}</h2>
      {children}
    </section>
  );
}

export function AboutScreen() {
  const { data, error } = useQuery({ queryKey: ['model-card'], queryFn: ({ signal }) => getModelCard(signal) });

  let body: React.ReactNode;
  if (error) {
    const msg =
      error instanceof ApiError && error.code === 'no_model_card' ? error.message : "Can't load the model card.";
    body = (
      <p className="placeholder" role="alert">
        {msg}
      </p>
    );
  } else if (!data) {
    body = <p className="placeholder soft">Loading the model card…</p>;
  } else {
    body = (
      <>
        {data.synthetic && <p className="about-note">Synthetic data. These numbers say nothing about gambling harm.</p>}
        <Section title="What the score means">
          <p>{data.summary}</p>
        </Section>
        <Section title="How the label is defined">
          <p>{data.label_definition}</p>
        </Section>
        <Section title="Intended use">
          <p>{data.intended_use}</p>
        </Section>
        <Section title="Model and rules compared">
          <Metrics card={data} />
        </Section>
        <Section title="Known limits">
          {data.limits.length === 0 ? (
            <p className="soft">None listed.</p>
          ) : (
            <ul>
              {data.limits.map((l, i) => (
                // biome-ignore lint/suspicious/noArrayIndexKey: static list, limits may repeat
                <li key={i}>{l}</li>
              ))}
            </ul>
          )}
        </Section>
        <Section title="Data protection">
          <p>{data.data_protection}</p>
        </Section>
        <Section title="Ethical risks">
          <p>{data.ethical_risks}</p>
        </Section>
        <p className="soft">Model {data.model_version}</p>
      </>
    );
  }

  return (
    <section className="screen" aria-labelledby="about-title">
      <div className="qhead">
        <h1 id="about-title">About the score</h1>
      </div>
      {body}
    </section>
  );
}
