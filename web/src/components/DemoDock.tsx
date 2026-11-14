import type { CSSProperties } from 'react';
import { useDemo } from './DemoContext';
import { Icon } from './Icons';

function fmt(seconds: number) {
  const total = Math.max(0, Math.floor(seconds));
  return `${String(Math.floor(total / 60)).padStart(2, '0')}:${String(total % 60).padStart(2, '0')}`;
}

export default function DemoDock() {
  const { player } = useDemo();
  const { now, playing, boost, toggle, seek } = player;
  if (!now) return null;

  return (
    <div className="qb-dock">
      <span
        className="qb-dock-art"
        role="img"
        aria-label={`${now.title} 的海报`}
        style={{ '--g': now.gradient } as CSSProperties}
      ><i /></span>
      <div className="qb-dock-meta"><b>{now.title}</b><small>{now.line} · {now.episode}</small></div>
      <button type="button" className="qb-round" onClick={toggle} aria-label={playing ? '暂停' : '播放'}>
        <Icon name={playing ? 'pause' : 'play'} size={14} />
      </button>
      <div className="qb-scrub">
        <label htmlFor="qb-scrub" className="qb-sr-only">播放进度</label>
        <span>{fmt(now.at)}</span>
        <input
          id="qb-scrub"
          type="range"
          min="0"
          max="1000"
          value={Math.round((now.at / now.duration) * 1000)}
          aria-label="播放进度"
          onChange={(event) => seek(Number(event.target.value) / 1000)}
        />
        <span>{fmt(now.duration)}</span>
      </div>
      <span className="qb-boost" data-on={boost}>2×</span>
    </div>
  );
}
