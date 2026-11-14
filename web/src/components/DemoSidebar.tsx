import { Icon, type IconName } from './Icons';
import { useDemo, type View } from './DemoContext';

const MAIN: { id: View; label: string; icon: IconName }[] = [
  { id: 'home', label: '首页', icon: 'home' },
  { id: 'cats', label: '分类', icon: 'list' },
  { id: 'live', label: '直播', icon: 'live' },
];
const LIBRARY: { id: View; label: string; icon: IconName }[] = [
  { id: 'history', label: '历史', icon: 'clock' },
  { id: 'fav', label: '收藏', icon: 'heart' },
];
const FOOT: { id: View; label: string; icon: IconName }[] = [
  { id: 'manage', label: '管理', icon: 'sliders' },
  { id: 'settings', label: '设置', icon: 'settings' },
];

function Group({ items }: { items: typeof MAIN }) {
  const { view, go } = useDemo();
  return (
    <>
      {items.map((item) => (
        <button
          type="button"
          key={item.id}
          className="qb-side-btn"
          aria-pressed={view === item.id || (view === 'detail' && item.id === 'home')}
          onClick={() => go(item.id)}
        >
          <Icon name={item.icon} size={15} />
          <span>{item.label}</span>
        </button>
      ))}
    </>
  );
}

export default function DemoSidebar() {
  const { query, setQuery, go } = useDemo();
  return (
    <aside className="qb-side">
      <div className="qb-traffic" aria-hidden="true"><i /><i /><i /></div>
      <div className="qb-side-brand"><img src="./icon.png" width="22" height="22" alt="" loading="lazy" /><span>QuickBo</span></div>
      <div className="qb-search-field">
        <label htmlFor="qb-q">搜索</label>
        <div className="qb-search">
          <Icon name="search" size={14} />
          <input
            id="qb-q"
            type="search"
            value={query}
            placeholder="搜索影片"
            aria-label="搜索影片"
            onChange={(event) => { setQuery(event.target.value); go('home'); }}
          />
        </div>
      </div>
      <nav className="qb-side-nav" aria-label="窗口侧栏">
        <p className="qb-side-label">导航</p>
        <Group items={MAIN} />
        <p className="qb-side-label">资料库</p>
        <Group items={LIBRARY} />
      </nav>
      <div className="qb-side-foot"><Group items={FOOT} /></div>
    </aside>
  );
}
