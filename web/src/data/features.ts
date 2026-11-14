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
    title: '多份订阅，统一管理',
    body: '同时导入多个订阅，所有影视内容汇总到同一个媒体库。同一部影片自动合并展示，不必在不同片源之间来回查找。',
    visual: 'subs',
  },
  {
    id: 'search',
    title: '全局搜索，一次找到',
    body: '只需搜索一次，QuickBo 就会检索所有已启用的订阅。同名影片的结果自动整合，打开后即可查看对应的播放线路。',
    visual: 'search',
  },
  {
    id: 'lines',
    title: '多条线路，自由切换',
    body: '同一部影片的不同播放线路集中展示。线路状态一目了然，当前线路无法播放时，可以直接切换其他线路。',
    visual: 'lines',
  },
  {
    id: 'play',
    title: '完整的桌面播放体验',
    body: '支持全屏播放、悬浮小窗、字幕和音轨选择、倍速播放。自动保存观看进度，下次打开即可继续观看。',
    visual: 'transport',
  },
];
