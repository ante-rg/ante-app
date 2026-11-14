import { useEffect, useRef, useState } from 'react';
import { useReplayDay } from './ReplayDay';

const TICK_MS = 400;

export function ReplayControl() {
  const { day, minDay, maxDay, setDay } = useReplayDay();
  const [playing, setPlaying] = useState(false);
  const dayRef = useRef(day);
  dayRef.current = day;

  useEffect(() => {
    if (!playing) return;
    const id = setInterval(() => {
      const next = dayRef.current + 1;
      dayRef.current = next;
      setDay(next);
      if (next >= maxDay) setPlaying(false);
    }, TICK_MS);
    return () => clearInterval(id);
  }, [playing, maxDay, setDay]);

  function toggle() {
    if (!playing && day >= maxDay) setDay(minDay);
    setPlaying(!playing);
  }

  return (
    <div className="replay">
      <label htmlFor="day">Data as of day</label>
      <input
        type="range"
        id="day"
        min={minDay}
        max={maxDay}
        value={day}
        onChange={(e) => setDay(Number(e.target.value))}
      />
      <output htmlFor="day" aria-live={playing ? 'off' : undefined}>
        {day}
      </output>
      <button type="button" className="btn" aria-pressed={playing} onClick={toggle}>
        {playing ? 'Pause' : 'Play'}
      </button>
    </div>
  );
}
