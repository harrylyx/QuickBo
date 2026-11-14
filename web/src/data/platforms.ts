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
    name: 'macOS',
    files: ['Apple 芯片', 'Intel 芯片'],
    requirement: 'macOS 12 及以上',
    player: '根据电脑芯片选择对应版本。首次播放前，需要安装 mpv 播放组件。',
  },
  {
    id: 'windows',
    name: 'Windows',
    files: ['64 位', 'ARM'],
    requirement: 'Windows 10 及以上',
    player: '下载安装后即可使用，无需额外安装其他组件。',
  },
  {
    id: 'linux',
    name: 'Linux',
    files: ['64 位', 'ARM'],
    requirement: '常见桌面发行版',
    player: '下载对应安装包，完成安装即可使用。',
  },
];
