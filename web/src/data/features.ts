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
    title: '几份订阅，一个首页',
    body: '同时加上几份 TVBox、FongMi 订阅，海报汇到同一面墙。同一部片子只留一张，不用来回翻。',
    visual: 'subs',
  },
  {
    id: 'search',
    title: '搜一次就够',
    body: '在首页搜，已经打开的片源都会找一遍。同名结果合成一条，点进去就能选线路。',
    visual: 'search',
  },
  {
    id: 'lines',
    title: '播不了就换一条',
    body: '同一部片子的线路摆在一起。不能播的会标出来，不用点进去才发现。',
    visual: 'lines',
  },
  {
    id: 'play',
    title: '就在这个窗口里看',
    body: '全屏、小窗、字幕和音轨都有。可以倍速，按住空格会临时加速，下次从上次的进度继续。',
    visual: 'transport',
  },
];
