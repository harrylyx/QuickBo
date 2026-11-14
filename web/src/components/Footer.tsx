import { FOOTER_COLUMNS, LICENSE, MAIN_ACTION } from '../data/site';
import { Arrow, GithubMark } from './Icons';

export default function Footer() {
  return (
    <footer className="footer closing footer--dark" id="contact" aria-labelledby="close-h">
      <div className="wrap">
        <div className="closing__cta close--word">
          <div className="section-head--split">
            <div>
              <h2 id="close-h">下载，加上 TVBox、FongMi 订阅</h2>
              <p className="t-lead">免费。Mac、Windows、Linux 都能用。</p>
            </div>
            <a className="btn btn--primary" href="#install">{MAIN_ACTION}<Arrow /></a>
          </div>
          <p className="close__word" aria-hidden="true">QuickBo</p>
        </div>
        <div className="closing__links">
          <div className="grid">
            <div className="span-3 stack-4">
              <a className="nav__logo" href="#top">
                <img src="./icon.png" width="26" height="26" alt="" loading="lazy" />
                QuickBo
              </a>
              <p className="footer__lede">电脑上的 TVBox、FongMi 播放器。片源由你自己添加。</p>
            </div>
            {FOOTER_COLUMNS.map((column) => (
              <div className="span-3 footer__col" key={column.title}>
                <h3>{column.title}</h3>
                <ul>
                  {column.links.map((link) => <li key={link.label}><a href={link.href}>{link.label}</a></li>)}
                </ul>
              </div>
            ))}
          </div>
          <div className="footer__legal">
            <span>© 2026 QuickBo contributors</span>
            <a href={LICENSE}>使用许可</a>
            <div className="socials">
              <a href="https://github.com/harrylyx/QuickBo" aria-label="QuickBo 的 GitHub"><GithubMark size={16} /></a>
            </div>
          </div>
        </div>
      </div>
    </footer>
  );
}
