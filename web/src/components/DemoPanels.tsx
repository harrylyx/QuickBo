import type { CSSProperties } from 'react';
import { useDemo } from './DemoContext';
import type { Title } from '../data/catalog';
import { LIVE_GROUPS, SUBSCRIPTIONS } from '../data/catalog';
import { Poster } from './DemoHome';
import ThemeControl from './ThemeControl';

function ListRow({ item }: { item: Title }) {
  const { open } = useDemo();
  return (
    <button type="button" className="qb-list-row" onClick={() => open(item.id)}>
      <span className="qb-list-art" role="img" aria-label={`${item.name} 的海报`} style={{ '--g': item.gradient } as CSSProperties}><i /></span>
      <span className="qb-list-meta"><b>{item.name}</b><small>{item.at || item.kind}</small></span>
      <span className="qb-bar"><i style={{ width: `${Math.round(item.progress * 100)}%` }} /></span>
    </button>
  );
}

function PanelHead({ title, note }: { title: string; note: string }) {
  return <div className="qb-greet"><h3>{title}</h3><p>{note}</p></div>;
}

export default function DemoPanels() {
  const { view, titles, subsOn, toggleSub, playLive, go } = useDemo();

  if (view === 'cats') {
    return (
      <div className="qb-panel">
        <PanelHead title="分类" note="电影、电视剧按你的分类排好。" />
        <div className="qb-poster-grid">{titles.map((item) => <Poster item={item} key={item.id} />)}</div>
      </div>
    );
  }

  if (view === 'live') {
    return (
      <div className="qb-panel">
        <PanelHead title="直播" note="导入直播列表，点一个频道就开始看。" />
        {LIVE_GROUPS.map((group) => (
          <div className="qb-live-group" key={group.genre}>
            <p className="qb-side-label">{group.genre}</p>
            {group.channels.map((channel) => (
              <button type="button" className="qb-list-row" key={channel} onClick={() => playLive(channel)}>
                <span className="qb-live-dot" aria-hidden="true" />
                <span className="qb-list-meta"><b>{channel}</b><small>晚间直播</small></span>
              </button>
            ))}
          </div>
        ))}
      </div>
    );
  }

  if (view === 'history' || view === 'fav') {
    const rows = view === 'history' ? titles.filter((item) => item.progress > 0) : titles.filter((item) => item.fav);
    return (
      <div className="qb-panel">
        <PanelHead
          title={view === 'history' ? '历史' : '收藏'}
          note={view === 'history' ? '下次打开，从上次停下的地方继续。' : '只存在这台电脑上。'}
        />
        <div className="qb-list">
          {rows.length ? rows.map((item) => <ListRow item={item} key={item.id} />) : <p className="qb-empty">这里还是空的。</p>}
        </div>
      </div>
    );
  }

  if (view === 'manage') {
    return (
      <div className="qb-panel">
        <PanelHead title="管理" note="订阅可以随时打开或关掉。不好用的片源可以先停掉。" />
        <div className="qb-list">
          {SUBSCRIPTIONS.map((sub, index) => (
            <button
              type="button"
              className="qb-sub"
              key={sub.name}
              aria-pressed={subsOn[index]}
              onClick={() => toggleSub(index)}
            >
              <span className="qb-list-meta"><b>{sub.name}</b><small>{subsOn[index] ? sub.meta : '已停用'}</small></span>
              <span className="qb-switch" data-on={subsOn[index]}><i /></span>
            </button>
          ))}
        </div>
        <button type="button" className="qb-text-btn" onClick={() => go('home')}>回到首页</button>
      </div>
    );
  }

  return (
    <div className="qb-panel qb-settings">
      <PanelHead title="设置" note="外观和这个页面用同一套颜色。" />
      <p>选一种外观，这个窗口和网页会一起变。你的选择只留在这台电脑上。</p>
      <ThemeControl />
    </div>
  );
}
