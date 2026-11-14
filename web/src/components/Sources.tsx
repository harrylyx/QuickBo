import { CONFIG_CODE, SOURCE_FACTS } from '../data/sources';

export default function Sources() {
  return (
    <section className="section" id="sources" aria-labelledby="sources-h">
      <div className="wrap">
        <div className="grid grid--top qb-sources-grid">
          <div className="span-5">
            <h2 id="sources-h">片源来自订阅文件</h2>
            <p className="t-lead qb-sources-lead">
              程序不内置片源、订阅地址或解析接口。贴一份配置网址，或打开本地 JSON，首页才有内容。删掉配置目录里的 settings.json，就回到空壳。
            </p>
            <div className="qb-facts">
              {SOURCE_FACTS.map((fact) => (
                <div className="qb-fact" key={fact.title}>
                  <b>{fact.title}</b><span>{fact.body}</span>
                </div>
              ))}
            </div>
            <p className="t-small muted qb-sources-formats">订阅可以是纯 JSON、宽松 JSON、Base64 或 gzip。</p>
          </div>
          <div className="span-6 start-7">
            <div className="qb-code" role="img" aria-label="一份订阅 JSON：sites 数组里有一个 type 1 的 MacCMS 片源">
              <div className="qb-code__bar" aria-hidden="true"><i /><i /><i /></div>
              <pre><code>{CONFIG_CODE}</code></pre>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
