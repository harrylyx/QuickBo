import { Icon, type IconName } from './Icons';
import { setMode, useTheme, type Mode } from '../hooks/useTheme';

const MODES: { id: Mode; label: string; icon: IconName }[] = [
  { id: 'light', label: '浅色', icon: 'sun' },
  { id: 'dark', label: '深色', icon: 'moon' },
  { id: 'system', label: '跟随系统', icon: 'monitor' },
];

export default function ThemeControl() {
  const mode = useTheme();
  return (
    <div className="qb-modes" role="radiogroup" aria-label="外观">
      {MODES.map((item) => (
        <button
          type="button"
          key={item.id}
          role="radio"
          aria-checked={mode === item.id}
          aria-label={item.label}
          data-mode={item.id}
          onClick={() => setMode(item.id)}
        >
          <Icon name={item.icon} size={15} />
          <span>{item.label}</span>
        </button>
      ))}
    </div>
  );
}
