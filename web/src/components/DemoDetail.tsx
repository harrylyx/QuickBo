import type { CSSProperties } from 'react';
import { useDemo } from './DemoContext';
import { LINES } from '../data/catalog';
import { Icon } from './Icons';

export default function DemoDetail() {
  const { current, line, setLine, episode, setEpisode, favs, toggleFav, go, play } = useDemo();
  const count = current.episodes ?? 1;
  const fav = Boolean(favs[current.id]);

  return (
    <div className="qb-panel qb-detail-panel">
      <button type="button" className="qb-back" onClick={() => go('home')}>
        <Icon name="back" size={13} /> 返回首页
      </button>
      <div className="qb-detail">
        <div
          className="qb-detail-art"
          role="img"
          aria-label={`${current.name} 的海报`}
          style={{ '--g': current.gradient } as CSSProperties}
        ><i /></div>
        <div className="qb-detail-body">
          <h3>{current.name}</h3>
          <div className="qb-pills">
            {[current.kind, current.year, current.area, current.remark].map((pill) => <span key={pill}>{pill}</span>)}
          </div>
          <div className="qb-lines">
            {LINES.map((item, index) => (
              <button
                type="button"
                key={item.name}
                className="qb-line"
                disabled={!item.ok}
                aria-pressed={index === line}
                onClick={() => setLine(index)}
              >
                <b>{item.name}</b><span>{item.ok ? (index === line ? '当前' : '可播放') : '无法播放'}</span>
              </button>
            ))}
          </div>
          <div className="qb-eps">
            {Array.from({ length: count }, (_, index) => index + 1).map((value) => (
              <button
                type="button"
                key={value}
                className="qb-ep"
                aria-pressed={value === episode}
                onClick={() => setEpisode(value)}
              >
                {count === 1 ? '正片' : String(value).padStart(2, '0')}
              </button>
            ))}
          </div>
          <div className="qb-detail-actions">
            <button type="button" className="qb-btn qb-btn--primary" onClick={() => play(current.id)}>
              <Icon name="play" size={13} /> 播放
            </button>
            <button type="button" className="qb-fav" aria-pressed={fav} aria-label={fav ? '取消收藏' : '收藏'} onClick={toggleFav}>
              <Icon name="heart" size={18} />
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
