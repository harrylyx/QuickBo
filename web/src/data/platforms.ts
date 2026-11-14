export interface Platform {
  id: string;
  name: string;
  files: string[];
  requirement: string;
  player: string;
}

export const PLATFORMS: Platform[] = [
  {
    id: 'macos',
    name: 'Mac',
    files: ['Apple 芯片', 'Intel 芯片'],
    requirement: 'macOS 12 及以上',
    player: '两个芯片各有一个安装包。第一次播放前，请先安装 mpv，这是用来播放的组件。',
  },
  {
    id: 'windows',
    name: 'Windows',
    files: ['64 位', 'ARM'],
    requirement: 'Windows 10 及以上',
    player: '装好就能看，不用再装别的。',
  },
  {
    id: 'linux',
    name: 'Linux',
    files: ['64 位', 'ARM'],
    requirement: '常见的桌面系统',
    player: '用安装包装好后就能看。',
  },
];
