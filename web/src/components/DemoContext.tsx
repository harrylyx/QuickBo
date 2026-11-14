import { createContext, useContext } from 'react';
import type { Title } from '../data/catalog';
import type { useDemoPlayer } from '../hooks/useDemoPlayer';

export type View = 'home' | 'detail' | 'cats' | 'live' | 'history' | 'fav' | 'manage' | 'settings';

export interface DemoApi {
  view: View;
  go: (view: View) => void;
  query: string;
  setQuery: (value: string) => void;
  titles: Title[];
  subsOn: boolean[];
  toggleSub: (index: number) => void;
  current: Title;
  line: number;
  setLine: (index: number) => void;
  episode: number;
  setEpisode: (value: number) => void;
  favs: Record<string, boolean>;
  toggleFav: () => void;
  open: (id: string) => void;
  play: (id: string, fromStart?: boolean) => void;
  playLive: (name: string) => void;
  player: ReturnType<typeof useDemoPlayer>;
}

const DemoContext = createContext<DemoApi | null>(null);

export const DemoProvider = DemoContext.Provider;

export function useDemo(): DemoApi {
  const value = useContext(DemoContext);
  if (!value) throw new Error('useDemo must be used inside the demo window');
  return value;
}
