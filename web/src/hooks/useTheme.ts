import { useEffect, useState } from 'react';

export type Mode = 'light' | 'dark' | 'system';

let current: Mode = 'dark';
const listeners = new Set<(mode: Mode) => void>();

function apply(mode: Mode) {
  const dark = mode === 'dark' || (mode === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches);
  document.documentElement.dataset.theme = dark ? 'dark' : 'light';
  document.querySelector('meta[name="theme-color"]')?.setAttribute('content', dark ? '#100d0e' : '#fbf7f1');
  try {
    localStorage.setItem('qb-theme', mode);
  } catch {
    /* private mode: the page still changes */
  }
}

export function setMode(mode: Mode) {
  current = mode;
  apply(mode);
  listeners.forEach((fn) => fn(mode));
}

export function initTheme() {
  let saved: Mode = 'dark';
  try {
    const value = localStorage.getItem('qb-theme');
    if (value === 'light' || value === 'dark' || value === 'system') saved = value;
  } catch {
    /* no saved preference */
  }
  current = saved;
  apply(saved);
  listeners.forEach((fn) => fn(saved));
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (current === 'system') apply('system');
  });
}

export function useTheme(): Mode {
  const [mode, setLocal] = useState<Mode>(current);
  useEffect(() => {
    listeners.add(setLocal);
    return () => {
      listeners.delete(setLocal);
    };
  }, []);
  return mode;
}
