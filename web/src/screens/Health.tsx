import { useQuery } from '@tanstack/react-query';
import { getHealth } from '../api/client';

export function Health() {
  const { data, isPending, isError } = useQuery({ queryKey: ['health'], queryFn: getHealth });

  if (isPending) return <p>Checking the API…</p>;
  if (isError) return <p role="alert">Can't reach the API</p>;
  return (
    <section aria-labelledby="health-heading">
      <h2 id="health-heading">System health</h2>
      <p>API status: {data.status}</p>
      <dl>
        {data.database && (
          <>
            <dt>Database</dt>
            <dd>{data.database}</dd>
          </>
        )}
        {data.scorer && (
          <>
            <dt>Scorer</dt>
            <dd>{data.scorer}</dd>
          </>
        )}
        <dt>Version</dt>
        <dd>{data.version}</dd>
      </dl>
    </section>
  );
}
