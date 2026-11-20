import { useQuery } from '@tanstack/react-query';
import { Link, useSearchParams } from 'react-router';
import { getOperators, type OperatorSummary } from '../api/client';
import { BandSquares } from '../components/BandSquares';
import { useReplayDay } from '../replay/ReplayDay';

const COLUMNS = 9;

function Strip({ o }: { o: OperatorSummary }) {
  return (
    <BandSquares className="pop pop-row" high={o.high} elevated={o.elevated} low={o.low} none={o.insufficient_data} />
  );
}

export function OperatorsScreen() {
  const { day } = useReplayDay();
  const blind = useSearchParams()[0].get('mode') === 'blind';
  const { data, isError, isPlaceholderData } = useQuery({
    queryKey: ['operators', day],
    queryFn: ({ signal }) => getOperators(day, signal),
    placeholderData: (prev, prevQuery) => (prevQuery && (prevQuery.queryKey[1] as number) <= day ? prev : undefined),
  });

  return (
    <section className="screen" aria-labelledby="ops-title">
      <div className="qhead">
        <div>
          <h1 id="ops-title">Operators</h1>
          <p className="soft">
            Band counts for each brand as of day {day}. Bands rank all five brands together: high is the group's top
            10%, elevated the next 10%.
          </p>
        </div>
      </div>
      <div className="panel">
        {isError && !data ? (
          <p className="placeholder" role="alert">
            Can't load operators.
          </p>
        ) : (
          <>
            {isError && (
              <p className="more soft" role="alert">
                Can't refresh operators. Showing the last loaded rows.
              </p>
            )}
            <div className="tablewrap" aria-busy={isPlaceholderData}>
              <table>
                <thead>
                  <tr>
                    <th>Operator</th>
                    <th className="r">Players</th>
                    <th>Bands</th>
                    <th className="r">High</th>
                    <th className="r">Elevated</th>
                    <th className="r">Low</th>
                    <th className="r">Not enough data</th>
                    <th className="r">Also flagged by rules</th>
                    <th className="r">Reviewed</th>
                  </tr>
                </thead>
                <tbody>
                  {data?.items.map((o) => (
                    <tr key={o.operator_code}>
                      <td>
                        <Link to={`/?operator=${o.operator_code}&day=${day}${blind ? '&mode=blind' : ''}`}>
                          {o.operator_code}
                        </Link>
                      </td>
                      <td className="r">{o.players}</td>
                      <td>
                        <Strip o={o} />
                      </td>
                      <td className="r">{o.high}</td>
                      <td className="r">{o.elevated}</td>
                      <td className="r">{o.low}</td>
                      <td className="r">{o.insufficient_data}</td>
                      <td className="r">{o.flagged_by_rules_too}</td>
                      <td className="r">{o.reviewed}</td>
                    </tr>
                  ))}
                  {!data && (
                    <tr>
                      <td colSpan={COLUMNS} className="soft">
                        Loading operators…
                      </td>
                    </tr>
                  )}
                  {data?.items.length === 0 && (
                    <tr>
                      <td colSpan={COLUMNS} className="soft">
                        No operators in this dataset.
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </>
        )}
      </div>
    </section>
  );
}
