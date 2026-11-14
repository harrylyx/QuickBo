import DemoWindow from './DemoWindow';

export default function DemoSection() {
  return (
    <section className="section" id="window" aria-labelledby="window-h">
      <div className="wrap">
        <div className="grid grid--top qb-window-head">
          <div className="span-5"><h2 id="window-h">打开就是这个样子</h2></div>
          <div className="span-6 start-7">
            <p className="t-lead">
              左边切换首页、直播和收藏。TVBox 和 FongMi 的片子，点进去就能换线路。
            </p>
          </div>
        </div>
        <DemoWindow />
      </div>
    </section>
  );
}
