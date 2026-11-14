import { useEffect, useState, type CSSProperties } from 'react';
import { useDemo } from './DemoContext';
import type { Title } from '../data/catalog';
import { Icon } from './Icons';

export function Poster({ item }: { item: Title }) {
  const { open } = useDemo();
  return (
    <button type="button" className="qb-poster" onClick={() => open(item.id)} aria-label={`打开${item.name}`}>
      <span
        className="qb-poster-art"
        role="img"
        aria-label={`${item.name} 的海报`}
        style={{ '--g': item.gradient } as CSSProperties}
      >
        <i /><em>{item.remark}</em>
      </span>
      <span className="qb-poster-name">{item.name}</span>
    </button>
  );
}

function ContinueCard({ item }: { item: Title }) {
  const { open } = useDemo();
  return (
    <button type="button" className="qb-cont" onClick={() => open(item.id)}>
      <span className="qb-cont-art" role="img" aria-label={`${item.name} 的海报`} style={{ '--g': item.gradient } as CSSProperties}><i /></span>
      <span className="qb-cont-meta">
        <b>{item.name}</b><small>{item.at}</small>
        <span className="qb-bar"><i style={{ width: `${Math.round(item.progress * 100)}%` }} /></span>
      </span>
    </button>
  );
}

function Rail({ label, items, onMore }: { label: string; items: Title[]; onMore?: () => void }) {
  const { go } = useDemo();
  return (
    <>
      <div className="qb-rail-head">
        <span>{label}</span>
        {onMore && <button type="button" onClick={onMore ?? (() => go('cats'))}>查看更多</button>}
      </div>
      <div className="qb-rail">{items.slice(0, 5).map((item) => <Poster item={item} key={item.id} />)}</div>
    </>
  );
}

export default function DemoHome() {
  const { titles, query, play, go } = useDemo();
  const [greet, setGreet] = useState('你好');
  const [today, setToday] = useState('今天');

  useEffect(() => {
    const hour = new Date().getHours();
    setGreet(hour < 11 ? '早上好' : hour < 18 ? '下午好' : '晚上好');
    setToday(new Date().toLocaleDateString('zh-CN', { month: 'long', day: 'numeric', weekday: 'long' }));
  }, []);

  const hits = titles.filter((item) => !query || item.name.includes(query));
  const movies = hits.filter((item) => item.kind === '电影');
  const shows = hits.filter((item) => item.kind === '电视剧');
  const continued = hits.filter((item) => item.progress > 0);

  return (
    <div className="qb-panel">
      <div className="qb-greet"><h3>{greet}</h3><p>{today} · 已启用 2 份订阅</p></div>
      {!query && (
        <div className="qb-banner-wrap">
          <div className="qb-banner" role="img" aria-label="雾港来信，电影 HD">
            <span>电影 · HD</span><strong>雾港来信</strong>
          </div>
          <button type="button" className="qb-banner-play" onClick={() => play('wu')}>
            <Icon name="play" size={12} /> 播放
          </button>
        </div>
      )}
      {query && hits.length === 0 && <p className="qb-empty">没有结果。换个词再搜。</p>}
      {continued.length > 0 && (
        <>
          <div className="qb-rail-head"><span>继续观看</span></div>
          <div className="qb-continue">{continued.slice(0, 3).map((item) => <ContinueCard item={item} key={item.id} />)}</div>
        </>
      )}
      <Rail label="电影" items={movies} onMore={() => go('cats')} />
      <Rail label="电视剧" items={shows} />
    </div>
  );
}
