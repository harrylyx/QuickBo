import type { CSSProperties } from 'react';

const POSTERS = [
  'linear-gradient(155deg,#f0a35b,#8a341c 58%,#3d1a14)',
  'linear-gradient(155deg,#6fb0d0,#1b3f57 60%,#10202c)',
  'linear-gradient(155deg,#e5c46d,#8b641d 55%,#3b2a10)',
  'linear-gradient(155deg,#c4866c,#6b382b 58%,#2b1714)',
  'linear-gradient(155deg,#efa35a,#a2471d 55%,#3f1c10)',
];

function PosterRow({ offset = 0 }: { offset?: number }) {
  return (
    <div className="qb-hero-posters">
      {POSTERS.map((g, i) => (
        <span
          key={`${offset}-${i}`}
          className="qb-hero-poster"
          style={{ '--g': g } as CSSProperties}
        >
          <i />
        </span>
      ))}
    </div>
  );
}

export default function HeroArt() {
  return (
    <div
      className="hero__art span-6 start-7"
      role="img"
      aria-label="QuickBo 的窗口：侧栏是订阅与资料库，首页是通栏横幅和两排海报"
    >
      <div className="ly-piece ly-piece--main qb-hero-win" aria-hidden="true">
        <div className="qb-hero-win__bar">
          <span className="qb-traffic"><i /><i /><i /></span>
          <span>QuickBo · 家庭片库</span>
        </div>
        <div className="qb-hero-win__body">
          <div className="qb-hero-win__side">
            <span className="is-on">首页</span><span>分类</span><span>直播</span><span>历史</span><span>收藏</span>
            <b>订阅</b><span>家庭片库</span><span>晚间直播</span>
          </div>
          <div className="qb-hero-win__main">
            <div className="qb-hero-win__head"><b>晚上好</b><span>搜索全部片源</span></div>
            <div className="qb-hero-win__banner"><b>雾港来信</b><span>电影 · HD</span></div>
            <div className="qb-hero-win__label"><span>继续观看</span><span>电影</span></div>
            <PosterRow />
            <div className="qb-hero-win__label"><span>电视剧</span><span>查看更多</span></div>
            <PosterRow offset={1} />
          </div>
        </div>
      </div>
      <div className="ly-piece ly-piece--note qb-hero-note" aria-hidden="true" data-ly-steps="700">
        <div className="qb-hero-note__head"><b>线路体检</b><span className="qb-chip" data-step data-step-at="3">3 条已测</span></div>
        <div className="qb-hero-note__line" data-step><i className="ok">✓</i><span>家庭片库 · 线路 1</span><b>可播放</b></div>
        <div className="qb-hero-note__line" data-step><i className="ok">✓</i><span>晚间直播 · 线路 2</span><b>可播放</b></div>
        <div className="qb-hero-note__line" data-step><i className="bad">!</i><span>备用源 · 线路 3</span><b>无法播放</b></div>
      </div>
    </div>
  );
}
