import DemoWindow from './DemoWindow';

export default function DemoSection() {
  return (
    <section className="section" id="window" aria-labelledby="window-h">
      <div className="wrap">
        <div className="grid grid--top qb-window-head">
          <div className="span-5"><h2 id="window-h">窗口里的首页与线路</h2></div>
          <div className="span-6 start-7">
            <p className="t-lead">
              侧栏切换订阅、分类、直播和资料库。搜索一次走完所有已启用的片源；详情页把同一部片的线路列在一起。
            </p>
          </div>
        </div>
        <DemoWindow />
      </div>
    </section>
  );
}
