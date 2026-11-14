import { PLATFORMS } from '../data/platforms';
import { Arrow, GithubMark } from './Icons';
import { MAIN_ACTION, RELEASES } from '../data/site';

export default function Install() {
  return (
    <section className="section" id="install" aria-labelledby="install-h">
      <div className="wrap">
        <div className="section-head">
          <h2 id="install-h">下载到你的电脑</h2>
          <p className="t-lead muted">Mac、Windows 和 Linux 都能装，用来看你的 TVBox、FongMi 订阅。</p>
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
          <a className="link qb-install-foot__repo" href={RELEASES}><GithubMark size={15} /> 查看全部版本</a>
        </div>
        <p className="t-small muted qb-install-note">
          第一次打开，系统可能会拦一下，点允许就行。这一版需要手动安装，之后可以在应用里更新。
        </p>
      </div>
    </section>
  );
}
