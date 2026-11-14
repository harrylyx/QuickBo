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
        <PanelHead title="分类" note="电影、电视剧按你设的规则汇总。" />
        <div className="qb-poster-grid">{titles.map((item) => <Poster item={item} key={item.id} />)}</div>
      </div>
    );
  }

  if (view === 'live') {
    return (
      <div className="qb-panel">
        <PanelHead title="直播" note="M3U，或带 #genre# 的「名称,地址」文本。" />
        {LIVE_GROUPS.map((group) => (
          <div className="qb-live-group" key={group.genre}>
            <p className="qb-side-label">{group.genre}</p>
            {group.channels.map((channel) => (
              <button type="button" className="qb-list-row" key={channel} onClick={() => playLive(channel)}>
                <span className="qb-live-dot" aria-hidden="true" />
                <span className="qb-list-meta"><b>{channel}</b><small>晚间直播 · M3U</small></span>
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
          note={view === 'history' ? '断点记在本机。' : '只存在这台电脑上。'}
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
        <PanelHead title="管理" note="订阅可以启用、停用。片源能测速，也能进黑名单。" />
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
      <p>选一项，窗口和网页一起变。偏好写在本机的 settings.json。</p>
      <ThemeControl />
    </div>
  );
}
