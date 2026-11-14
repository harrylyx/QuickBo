import { PLATFORMS } from '../data/platforms';
import { Arrow, GithubMark } from './Icons';
import { MAIN_ACTION, RELEASES } from '../data/site';

export default function Install() {
  return (
    <section className="section" id="install" aria-labelledby="install-h">
      <div className="wrap">
        <div className="section-head">
          <h2 id="install-h">三个系统的安装包</h2>
          <p className="t-lead muted">从 Releases 下对应的包。播放需要 libmpv，Windows 安装包已经带上。</p>
        </div>
        <div className="grid qb-platforms">
          {PLATFORMS.map((platform) => (
            <article className="card span-4 qb-platform" key={platform.id}>
              <h3>{platform.name}</h3>
              <div className="qb-platform__art" role="img" aria-label={`${platform.name} 安装后的窗口`} data-os={platform.id}>
                <span className="qb-platform__art-bar"><i /><i /><i /></span>
                <span className="qb-platform__art-disc" />
                <span className="qb-platform__art-row"><i /><i /></span>
              </div>
              <div className="qb-platform__files">
                {platform.files.map((file) => <span key={file}>{file}</span>)}
              </div>
              <p className="qb-platform__req">{platform.requirement}</p>
              <p>{platform.player}</p>
            </article>
          ))}
        </div>
        <div className="qb-install-foot">
          <a className="btn btn--primary" href={RELEASES}>{MAIN_ACTION}<Arrow /></a>
          <a className="link qb-install-foot__repo" href={RELEASES}><GithubMark size={15} /> 前往 Releases</a>
        </div>
        <p className="t-small muted qb-install-note">
          安装包还没有开发者签名，首次打开要在系统里放行一次。0.1.0 没有应用内更新，需要先手动装一版带更新的包。
        </p>
      </div>
    </section>
  );
}
