import HeroArt from './HeroArt';
import { Arrow } from './Icons';
import { MAIN_ACTION } from '../data/site';

export default function Hero() {
  return (
    <section className="hero hero--collage" data-enter aria-labelledby="hero-h">
      <div className="wrap">
        <div className="hero-text grid hero__grid">
          <div className="hero__text span-5">
            <p className="hero-notice">免费 · Mac、Windows、Linux</p>
            <h1 id="hero-h">电脑上的 TVBox、FongMi 播放器</h1>
            <p className="hero__lead t-lead">
              把 TVBox 或 FongMi 的订阅贴进来就能看。电影、剧集和直播都在一个窗口里，这条播不了，就换一条。
            </p>
            <div className="btn-row">
              <a className="btn btn--primary" href="#install">{MAIN_ACTION}<Arrow /></a>
              <a className="btn btn--secondary" href="#window">先看看界面</a>
            </div>
            <p className="hero__proof">
              不自带影片。订阅由你自己添加，装好就能看。
            </p>
          </div>
          <HeroArt />
        </div>
      </div>
    </section>
  );
}
