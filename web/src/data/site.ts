export const REPO = 'https://github.com/harrylyx/QuickBo';
export const RELEASES = `${REPO}/releases`;
export const LICENSE = `${REPO}/blob/master/LICENSE`;
export const MYGO = 'https://github.com/egoist/mygo';

export const MAIN_ACTION = '下载 QuickBo';
export const SOURCE_ACTION = '查看源代码';

export const NAV_LINKS = [
  { label: '窗口', href: '#window' },
  { label: '功能', href: '#features' },
  { label: '片源', href: '#sources' },
  { label: '下载', href: '#install' },
] as const;

export const MENU_LINKS = [
  { label: '多份订阅', href: '#features', note: '启用、停用、批量管理' },
  { label: '全源搜索', href: '#features', note: '同名结果并成一条' },
  { label: '窗口播放', href: '#features', note: 'libmpv 在应用窗口内' },
  { label: '订阅格式', href: '#sources', note: 'JSON、Base64、gzip' },
] as const;

export const FOOTER_COLUMNS = [
  {
    title: '产品',
    links: [
      { label: '窗口', href: '#window' },
      { label: '功能', href: '#features' },
      { label: '片源', href: '#sources' },
    ],
  },
  {
    title: '安装',
    links: [
      { label: '下载', href: '#install' },
      { label: 'Releases', href: RELEASES },
      { label: '构建说明', href: `${REPO}#开发者构建` },
    ],
  },
  {
    title: '项目',
    links: [
      { label: '源代码', href: REPO },
      { label: '许可', href: LICENSE },
      { label: 'mygo', href: MYGO },
    ],
  },
] as const;
