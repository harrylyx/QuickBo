import { flushSync } from 'react-dom';
import { createRoot, hydrateRoot } from 'react-dom/client';
import App from './App';
import { initTheme } from './hooks/useTheme';
import './styles/tokens.css';
import './styles/window.css';
import './styles/sections.css';
import './styles/responsive.css';
import './styles/finish.css';

initTheme();

const root = document.getElementById('root');
if (!root) throw new Error('#root is missing');

if (root.hasChildNodes()) flushSync(() => hydrateRoot(root, <App />));
else createRoot(root).render(<App />);
