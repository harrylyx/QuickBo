import { useEffect, useMemo, useRef, useState } from 'react';
import { DemoProvider, type DemoApi, type View } from './DemoContext';
import DemoSidebar from './DemoSidebar';
import DemoHome from './DemoHome';
import DemoDetail from './DemoDetail';
import DemoPanels from './DemoPanels';
import DemoDock from './DemoDock';
import { LINES, SUBSCRIPTIONS, TITLES } from '../data/catalog';
import { useDemoPlayer } from '../hooks/useDemoPlayer';

export default function DemoWindow() {
  const player = useDemoPlayer();
  const [view, setView] = useState<View>('home');
  const [query, setQuery] = useState('');
  const [selectedId, setSelectedId] = useState(TITLES[0].id);
  const [line, setLine] = useState(0);
  const [episode, setEpisode] = useState(1);
  const [favs, setFavs] = useState<Record<string, boolean>>({ wan: true });
  const [progress, setProgress] = useState<Record<string, number>>({});
  const [subsOn, setSubsOn] = useState<boolean[]>(SUBSCRIPTIONS.map((sub) => sub.on));
  const hold = useRef<{ timer: number; held: boolean }>({ timer: 0, held: false });

  const titles = useMemo(
    () => TITLES.map((item) => ({
      ...item,
      progress: progress[item.id] ?? item.progress,
      fav: favs[item.id] ?? item.fav,
    })),
    [progress, favs],
  );
  const current = titles.find((item) => item.id === selectedId) ?? titles[0];

  const go = (next: View) => {
    if (next === 'home') setQuery('');
    setView(next);
  };

  const open = (id: string) => {
    const item = titles.find((entry) => entry.id === id);
    if (!item) return;
    setSelectedId(id);
    setLine(0);
    setEpisode(item.episode ?? 1);
    setView('detail');
  };

  const play = (id: string, fromStart = false) => {
    const item = titles.find((entry) => entry.id === id) ?? titles[0];
    const at = fromStart ? 0 : Math.round(item.progress * item.duration);
    const label = item.episodes ? `第 ${item.episodes === 1 ? 1 : episode} 集` : '正片';
    player.start({
      title: item.name,
      gradient: item.gradient,
      line: LINES[line]?.name ?? '直播',
      episode: label,
      duration: item.duration,
      at,
    });
    setProgress((prev) => ({ ...prev, [item.id]: at / item.duration }));
  };

  const playLive = (name: string) => {
    player.start({
      title: name,
      gradient: 'linear-gradient(155deg,#d1694f,#7a2820 58%,#301110)',
      line: '晚间直播',
      episode: '直播',
      duration: 3600,
      at: 720,
    });
  };

  useEffect(() => {
    const isControl = (target: EventTarget | null) =>
      target instanceof Element && Boolean(target.closest('button, a, input, textarea, select'));

    const down = (event: KeyboardEvent) => {
      if ((event.code !== 'Space' && event.code !== 'KeyB') || event.repeat) return;
      if (isControl(event.target) || !player.now) return;
      event.preventDefault();
      hold.current.held = false;
      hold.current.timer = window.setTimeout(() => {
        hold.current.held = true;
        player.setBoost(true);
      }, 180);
    };
    const up = (event: KeyboardEvent) => {
      if (event.code !== 'Space' && event.code !== 'KeyB') return;
      window.clearTimeout(hold.current.timer);
      if (isControl(event.target) || !player.now) return;
      if (hold.current.held) player.setBoost(false);
      else if (event.code === 'Space') player.toggle();
      hold.current.held = false;
    };
    document.addEventListener('keydown', down);
    document.addEventListener('keyup', up);
    return () => {
      document.removeEventListener('keydown', down);
      document.removeEventListener('keyup', up);
    };
  }, [player.now, player.setBoost, player.toggle]);

  const api: DemoApi = {
    view, go, query, setQuery, titles, subsOn,
    toggleSub: (index) => setSubsOn((prev) => prev.map((on, i) => (i === index ? !on : on))),
    current, line, setLine, episode, setEpisode, favs,
    toggleFav: () => setFavs((prev) => ({ ...prev, [current.id]: !prev[current.id] })),
    open, play, playLive, player,
  };

  return (
    <DemoProvider value={api}>
      <div className="ui ui-window qb-window" data-visual aria-label="QuickBo 窗口演示">
        <div className="qb-win-body">
          <DemoSidebar />
          <div className="qb-main">
            {view === 'home' && <DemoHome />}
            {view === 'detail' && <DemoDetail />}
            {view !== 'home' && view !== 'detail' && <DemoPanels />}
          </div>
        </div>
        {player.now && <DemoDock />}
      </div>
    </DemoProvider>
  );
}
