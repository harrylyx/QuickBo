import HeroArt from './HeroArt';
import { Arrow } from './Icons';
import { MAIN_ACTION, REPO, SOURCE_ACTION } from '../data/site';

export default function Hero() {
  return (
    <section className="hero hero--collage" data-enter aria-labelledby="hero-h">
      <div className="wrap">
        <div className="hero-text grid hero__grid">
          <div className="hero__text span-5">
            <p className="hero-notice">0.1.0 · 以 MIT 开源</p>
            <h1 id="hero-h">给桌面用的 TVBox 播放器</h1>
            <p className="hero__lead t-lead">
              用 Go 与 mygo 写成，界面由 GPU 绘制，没有 WebView。点播、直播、搜索和换源都在一个窗口里，画面由 libmpv 渲染。安装包不带片源，订阅由你自己添加。
            </p>
            <div className="btn-row">
              <a className="btn btn--primary" href="#install">{MAIN_ACTION}<Arrow /></a>
              <a className="btn btn--secondary" href={REPO}>{SOURCE_ACTION}</a>
            </div>
            <p className="hero__proof">
              装好是空的：没有片源、订阅地址或解析接口。支持 macOS 12、Windows 10 与带 GTK 3 的 Linux。
            </p>
          </div>
          <HeroArt />
        </div>
      </div>
    </section>
  );
}
