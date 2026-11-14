export const REPO = 'https://github.com/harrylyx/QuickBo';
export const RELEASES = `${REPO}/releases`;
export const LICENSE = `${REPO}/blob/master/LICENSE`;
export const MAIN_ACTION = '下载 QuickBo';

export const NAV_LINKS = [
  { label: '界面', href: '#window' },
  { label: '功能', href: '#features' },
  { label: '订阅', href: '#sources' },
  { label: '下载', href: '#install' },
] as const;

export const MENU_LINKS = [
  { label: '多份订阅', href: '#features', note: '合成一个首页' },
  { label: '搜索', href: '#features', note: '一次找完全部片子' },
  { label: '播放', href: '#features', note: '全屏、小窗和字幕' },
  { label: '添加订阅', href: '#sources', note: 'TVBox 或 FongMi 地址' },
] as const;

export const FOOTER_COLUMNS = [
  {
    title: '产品',
    links: [
      { label: '界面', href: '#window' },
      { label: '功能', href: '#features' },
      { label: '订阅', href: '#sources' },
    ],
  },
  {
    title: '下载',
    links: [
      { label: '安装包', href: '#install' },
      { label: '全部版本', href: RELEASES },
    ],
  },
  {
    title: '关于',
    links: [
      { label: '源代码', href: REPO },
      { label: '使用许可', href: LICENSE },
      { label: '反馈问题', href: `${REPO}/issues` },
    ],
  },
] as const;
