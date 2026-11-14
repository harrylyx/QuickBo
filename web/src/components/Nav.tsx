import { useState } from 'react';
import { MAIN_ACTION, MENU_LINKS, NAV_LINKS } from '../data/site';

const Chevron = () => (
  <svg className="chev" viewBox="0 0 12 12" aria-hidden="true">
    <path d="M3 4.5l3 3 3-3" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
  </svg>
);

export default function Nav() {
  const [open, setOpen] = useState(false);
  const toggle = () => {
    const next = !open;
    setOpen(next);
    const bar = document.querySelector('header.nav');
    bar?.classList.toggle('is-open', next);
    bar?.querySelector('.nav__toggle')?.setAttribute('aria-expanded', String(next));
  };
  return (
    <header className={open ? 'nav is-open' : 'nav'}>
      <div className="wrap nav__bar">
        <a className="skip-link" href="#top">跳到内容</a>
        <a className="nav__logo" href="#top">
          <img src="./icon.png" width="26" height="26" alt="" />
          QuickBo
        </a>
        <nav className="nav__links" aria-label="主导航">
          {NAV_LINKS.map((link) => (
            <div className="nav__item" key={link.label}>
              <a className="nav__link" href={link.href}>
                {link.label}
                {link.label === '功能' && <Chevron />}
              </a>
              {link.label === '功能' && (
                <div className="nav__menu">
                  <div className="wrap grid">
                    <div className="menu-col span-3">
                      <p className="t-label">播放器</p>
                      {MENU_LINKS.slice(0, 3).map((item) => (
                        <a href={item.href} key={item.label}>
                          {item.label}<span>{item.note}</span>
                        </a>
                      ))}
                    </div>
                    <div className="menu-col span-3">
                      <p className="t-label">开始</p>
                      {MENU_LINKS.slice(3).map((item) => (
                        <a href={item.href} key={item.label}>
                          {item.label}<span>{item.note}</span>
                        </a>
                      ))}
                      <a href="#window">界面预览<span>首页、线路和播放</span></a>
                    </div>
                    <div className="menu-card span-3 start-10 qb-menu-card">
                      <span className="t-label">装好之后</span>
                      <span className="qb-menu-card__line">没有自带影片</span>
                      <span className="qb-menu-card__line">加上 TVBox 或 FongMi</span>
                      <span className="qb-menu-card__line">打开就能看</span>
                    </div>
                  </div>
                </div>
              )}
            </div>
          ))}
        </nav>
        <div className="nav__actions">
          <a className="btn btn--primary" href="#install">{MAIN_ACTION}</a>
        </div>
        <button className="nav__toggle" type="button" aria-expanded={open} aria-label="打开菜单" onClick={toggle}>
          <svg width="18" height="18" viewBox="0 0 18 18" aria-hidden="true">
            <path d="M2 5h14M2 13h14" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
          </svg>
        </button>
      </div>
    </header>
  );
}
