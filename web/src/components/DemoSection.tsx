import DemoWindow from './DemoWindow';

export default function DemoSection() {
  return (
    <section className="section" id="window" aria-labelledby="window-h">
      <div className="wrap">
        <div className="grid grid--top qb-window-head">
          <div className="span-5"><h2 id="window-h">你的影视片库，现在也能在电脑上使用</h2></div>
          <div className="span-6 start-7">
            <p className="t-lead">
              导入订阅后，电影、剧集和直播会自动整理到首页。无需在多个片源之间反复切换，打开 QuickBo 就能浏览和播放。
            </p>
          </div>
        </div>
        <DemoWindow />
      </div>
    </section>
  );
}
