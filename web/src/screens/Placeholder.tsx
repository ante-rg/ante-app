import { Link, useParams } from 'react-router';
import { useReplayDay } from '../replay/ReplayDay';

export function Placeholder({ title }: { title: string }) {
  return (
    <section className="screen">
      <h1>{title}</h1>
      <p className="soft">Not part of this build yet.</p>
    </section>
  );
}

export function PlayerPlaceholder() {
  const { playerId } = useParams();
  const { day } = useReplayDay();
  return (
    <section className="screen">
      <h1>Player {playerId}</h1>
      <p>
        <Link to={`/?day=${day}`}>Review queue</Link>
      </p>
      <p className="soft">The case view is not built yet.</p>
    </section>
  );
}
