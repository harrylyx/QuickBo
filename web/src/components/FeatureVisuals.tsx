import type { VisualKind } from '../data/features';
import { LINES, SUBSCRIPTIONS } from '../data/catalog';

function SubsVisual() {
  return (
    <div className="qb-frag qb-frag--subs" aria-hidden="true">
      <div className="qb-frag__head"><span>订阅</span><span>3 份</span></div>
      {SUBSCRIPTIONS.map((sub) => (
        <div className="qb-frag__row" key={sub.name}>
          <span className="qb-frag__mark" data-on={sub.on} />
          <span><b>{sub.name}</b><small>{sub.meta}</small></span>
          <span className="qb-switch" data-on={sub.on}><i /></span>
        </div>
      ))}
    </div>
  );
}

function SearchVisual() {
  return (
    <div className="qb-frag qb-frag--search" aria-hidden="true">
      <div className="qb-frag__query">雾港<span className="qb-caret" /></div>
      <div className="qb-frag__hit"><b>雾港来信</b><span>2 个源</span></div>
      <div className="qb-frag__hit"><b>雾港夜航</b><span>1 个源</span></div>
      <div className="qb-frag__foot">搜索已启用订阅里的全部片源</div>
    </div>
  );
}

function LinesVisual() {
  return (
    <div className="qb-frag qb-frag--lines" aria-hidden="true">
      <div className="qb-frag__head"><span>线路</span><span>雾港来信</span></div>
      {LINES.map((line, i) => (
        <div className="qb-frag__line" key={line.name} data-state={line.ok ? (i === 1 ? 'current' : 'ok') : 'dead'}>
          <b>{line.name}</b><span>{line.ok ? (i === 1 ? '当前' : '可播放') : '无法播放'}</span>
        </div>
      ))}
    </div>
  );
}

function TransportVisual() {
  return (
    <div className="qb-frag qb-frag--transport" aria-hidden="true">
      <div className="qb-frag__head"><span>窗口内播放</span><span>libmpv</span></div>
      <div className="qb-frag__bar"><span className="qb-frag__disc"><i /></span><b>18:54</b><span className="qb-frag__track"><i /></span><b>45:00</b><em>1×</em></div>
      <div className="qb-frag__chips"><span>片头 01:24 已记住</span><span>悬浮小窗</span><span>字幕</span><span>截图</span></div>
    </div>
  );
}

export default function FeatureVisual({ kind }: { kind: VisualKind }) {
  switch (kind) {
    case 'subs': return <SubsVisual />;
    case 'search': return <SearchVisual />;
    case 'lines': return <LinesVisual />;
    case 'transport': return <TransportVisual />;
  }
}
