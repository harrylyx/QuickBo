import { useCallback, useEffect, useState } from 'react';

export interface Playing {
  title: string;
  gradient: string;
  line: string;
  episode: string;
  duration: number;
  at: number;
}

export function useDemoPlayer() {
  const [now, setNow] = useState<Playing | null>(null);
  const [playing, setPlaying] = useState(false);
  const [boost, setBoost] = useState(false);
  const active = now !== null;

  useEffect(() => {
    if (!playing || !active) return;
    const timer = window.setInterval(() => {
      setNow((prev) => (prev ? { ...prev, at: Math.min(prev.duration, prev.at + (boost ? 2 : 1)) } : prev));
    }, 1000);
    return () => window.clearInterval(timer);
  }, [playing, boost, active]);

  useEffect(() => {
    if (now && now.at >= now.duration) setPlaying(false);
  }, [now?.at, now?.duration]);

  const start = useCallback((item: Playing) => {
    setNow(item);
    setPlaying(true);
  }, []);

  const toggle = useCallback(() => {
    setPlaying((on) => !on);
  }, []);

  const seek = useCallback((fraction: number) => {
    setNow((prev) => (prev ? { ...prev, at: Math.round(prev.duration * fraction) } : prev));
  }, []);

  return { now, playing, boost, start, toggle, seek, setBoost };
}
