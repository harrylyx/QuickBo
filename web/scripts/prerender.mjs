import { build } from 'esbuild';
import { readFile, writeFile, rm, mkdir } from 'node:fs/promises';
import { pathToFileURL } from 'node:url';
import { resolve } from 'node:path';

const root = resolve(import.meta.dirname, '..');
const out = resolve(root, '.prerender/entry-server.mjs');

await mkdir(resolve(root, '.prerender'), { recursive: true });
await build({
  entryPoints: [resolve(root, 'src/entry-server.tsx')],
  outfile: out,
  bundle: true,
  platform: 'node',
  format: 'esm',
  jsx: 'automatic',
  packages: 'external',
  logLevel: 'warning',
});

const { render } = await import(pathToFileURL(out).href);
const markup = render();
const file = resolve(root, 'dist/index.html');
const html = await readFile(file, 'utf8');
const marker = '<div id="root"></div>';
if (!html.includes(marker)) throw new Error('dist/index.html no longer holds #root');
const localPaths = html.replace(/(href|src)="\/(?!\/)/g, '$1="./');
await writeFile(file, localPaths.replace(marker, `<div id="root">${markup}</div>`), 'utf8');
await rm(resolve(root, '.prerender'), { recursive: true, force: true });
console.log(`prerender: ${markup.length} bytes of static markup written into dist/index.html`);
