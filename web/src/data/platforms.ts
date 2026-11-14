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
    files: ['quickbo-<版本>-arm64.dmg', 'quickbo-<版本>-amd64.dmg'],
    requirement: 'macOS 12 及以上',
    player: '播放前先 brew install mpv。Apple 芯片用 arm64，Intel 用 amd64。',
  },
  {
    id: 'windows',
    name: 'Windows',
    files: ['quickbo-setup-<版本>-amd64.exe', 'quickbo-setup-<版本>-arm64.exe'],
    requirement: 'Windows 10 及以上',
    player: '安装包自带对应架构的 libmpv-2.dll，装好就能在窗口里放。',
  },
  {
    id: 'linux',
    name: 'Linux',
    files: ['quickbo_<版本>_amd64.deb', 'quickbo_<版本>_arm64.deb'],
    requirement: '需要 GTK 3',
    player: '.deb 会依赖 libmpv2；.tar.gz 里有 install.sh，需要自己 sudo apt install libmpv2。',
  },
];
