import HeroArt from './HeroArt';
import { Arrow } from './Icons';
import { MAIN_ACTION } from '../data/site';

export default function Hero() {
  return (
    <section className="hero hero--collage" data-enter aria-labelledby="hero-h">
      <div className="wrap">
        <div className="hero-text grid hero__grid">
          <div className="hero__text span-5">
            <p className="hero-notice">免费桌面播放器 · macOS / Windows / Linux</p>
            <h1 id="hero-h">把 TVBox、FongMi 带到电脑上</h1>
            <p className="hero__lead t-lead">
              导入你的订阅地址，把不同片源整合到一个媒体库。电影、剧集和直播统一管理，搜索、切换线路、播放都在一个窗口里完成。
            </p>
            <div className="btn-row">
              <a className="btn btn--primary" href="#install">{MAIN_ACTION}<Arrow /></a>
              <a className="btn btn--secondary" href="#features">了解产品功能</a>
            </div>
            <p className="hero__proof">
              QuickBo 不提供影视内容。订阅和片源由你自行添加。
            </p>
          </div>
          <HeroArt />
        </div>
      </div>
    </section>
  );
}
