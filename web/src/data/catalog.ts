export interface Title {
  id: string;
  name: string;
  kind: '电影' | '电视剧';
  remark: string;
  year: string;
  area: string;
  gradient: string;
  progress: number;
  at: string;
  duration: number;
  episodes?: number;
  episode?: number;
  fav: boolean;
}

export const TITLES: Title[] = [
  { id: 'wu', name: '雾港来信', kind: '电影', remark: 'HD', year: '2024', area: '沿海', gradient: 'linear-gradient(155deg,#f0a35b,#8a341c 58%,#3d1a14)', progress: 0.42, at: '18:54', duration: 2700, fav: false },
  { id: 'bei', name: '北境', kind: '电影', remark: '4K', year: '2023', area: '北方', gradient: 'linear-gradient(155deg,#6fb0d0,#1b3f57 60%,#10202c)', progress: 0, at: '', duration: 6120, fav: false },
  { id: 'yan', name: '盐风', kind: '电影', remark: 'HD', year: '2022', area: '海岛', gradient: 'linear-gradient(155deg,#e5c46d,#8b641d 55%,#3b2a10)', progress: 0, at: '', duration: 5760, fav: false },
  { id: 'jiu', name: '旧车站', kind: '电影', remark: '完结', year: '2021', area: '内地', gradient: 'linear-gradient(155deg,#c4866c,#6b382b 58%,#2b1714)', progress: 0.12, at: '11:02', duration: 5280, fav: false },
  { id: 'wan', name: '晚潮', kind: '电视剧', remark: '更新至 12 集', year: '2025', area: '沿海', gradient: 'linear-gradient(155deg,#efa35a,#a2471d 55%,#3f1c10)', progress: 0.63, at: '第 4 集 · 28:10', duration: 2700, episodes: 12, episode: 4, fav: true },
  { id: 'qing', name: '青禾记', kind: '电视剧', remark: '完结', year: '2020', area: '江南', gradient: 'linear-gradient(155deg,#78b99c,#2c5c49 60%,#153026)', progress: 0.2, at: '第 3 集 · 09:40', duration: 2520, episodes: 16, episode: 3, fav: false },
  { id: 'deng', name: '灯塔季', kind: '电视剧', remark: '更新至 6 集', year: '2024', area: '港口', gradient: 'linear-gradient(155deg,#7d9ad0,#334a78 58%,#18243c)', progress: 0, at: '', duration: 2400, episodes: 6, episode: 1, fav: false },
  { id: 'hui', name: '回声', kind: '电影', remark: 'HD', year: '2019', area: '内地', gradient: 'linear-gradient(155deg,#dc7d6c,#8a2d28 58%,#3b1414)', progress: 0, at: '', duration: 6600, fav: false },
];

export const LINES = [
  { name: '家庭片库 · 线路 1', ok: true },
  { name: '晚间直播 · 线路 2', ok: true },
  { name: '备用源 · 线路 3', ok: false },
] as const;

export const LIVE_GROUPS = [
  { genre: '综合', channels: ['港湾一台', '港湾二台'] },
  { genre: '电影', channels: ['夜航剧场', '旧片放映'] },
] as const;

export const SUBSCRIPTIONS = [
  { name: '家庭片库', meta: 'JSON · 12 个源', on: true },
  { name: '晚间直播', meta: 'TXT · #genre#', on: true },
  { name: '备用源', meta: '已停用', on: false },
] as const;
