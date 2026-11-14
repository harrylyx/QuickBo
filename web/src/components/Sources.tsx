import { SOURCE_FACTS } from '../data/sources';

export default function Sources() {
  return (
    <section className="section" id="sources" aria-labelledby="sources-h">
      <div className="wrap">
        <div className="grid grid--top qb-sources-grid">
          <div className="span-5">
            <h2 id="sources-h">导入你的订阅，建立自己的媒体库</h2>
            <p className="t-lead qb-sources-lead">
              通过订阅地址或本地文件导入片源。QuickBo 会根据内容类型整理电影、剧集和直播，让分散的资源在一个界面中井然有序。
            </p>
            <div className="qb-facts">
              {SOURCE_FACTS.map((fact) => (
                <div className="qb-fact" key={fact.title}>
                  <b>{fact.title}</b><span>{fact.body}</span>
                </div>
              ))}
            </div>
          </div>
          <div className="span-6 start-7">
            <div className="qb-code qb-add" role="img" aria-label="添加订阅：贴上地址，或打开电脑里的文件">
              <div className="qb-code__bar" aria-hidden="true"><i /><i /><i /><span>添加订阅</span></div>
              <div className="qb-add__body">
                <p className="qb-add__label">订阅地址</p>
                <div className="qb-add__field">把地址贴在这里</div>
                <div className="qb-add__actions">
                  <span className="qb-add__btn">添加</span>
                  <span>或打开本地文件</span>
                </div>
                <div className="qb-add__list">
                  <div><b>家庭片库</b><span>12 个片源 · 已打开</span></div>
                  <div><b>晚间直播</b><span>直播列表 · 已打开</span></div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
