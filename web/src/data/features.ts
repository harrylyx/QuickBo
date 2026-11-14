export type VisualKind = 'subs' | 'search' | 'lines' | 'transport';

export interface Feature {
  id: string;
  title: string;
  body: string;
  visual: VisualKind;
}

export const FEATURES: Feature[] = [
  {
    id: 'merge',
    title: '合并多份订阅',
    body: '同时启用几份 TVBox / FongMi 订阅，或打开本地 JSON。各份里的片源汇到同一面海报墙，同一部片跨源并成一张。',
    visual: 'subs',
  },
  {
    id: 'search',
    title: '一次搜完全部片源',
    body: '首页顶上的搜索会走所有已启用的源，同名结果并成一条。搜完可以直接进详情页选线路。',
    visual: 'search',
  },
  {
    id: 'lines',
    title: '列出可播放的线路',
    body: '详情页把每个源的线路摆在一起。体检不通的线路会标出来，不用点进去才发现播不了。',
    visual: 'lines',
  },
  {
    id: 'play',
    title: '在窗口里播放',
    body: 'libmpv 把画面铺在应用窗口内。全屏、悬浮小窗、字幕和音轨都在；倍速 0.5× 到 3×，按住空格临时 2 倍。',
    visual: 'transport',
  },
];
