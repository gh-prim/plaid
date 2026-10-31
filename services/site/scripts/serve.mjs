// Local preview of dist/. With --watch, rebuilds when src/, locales/ or public/ change.
// Usage: node scripts/serve.mjs [--watch] [--port 4321]

import { watch } from 'node:fs';
import { readFile, stat } from 'node:fs/promises';
import { createServer } from 'node:http';
import { dirname, extname, join, normalize } from 'node:path';
import { fileURLToPath } from 'node:url';

import { build } from './build.mjs';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');
const dist = join(root, 'dist');
const args = process.argv.slice(2);
const port = Number(args[args.indexOf('--port') + 1]) || 4321;
const TYPES = {
  '.html': 'text/html; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.woff2': 'font/woff2',
  '.xml': 'application/xml',
  '.txt': 'text/plain; charset=utf-8',
};

build();
if (args.includes('--watch')) {
  let timer;
  for (const dir of ['src', 'locales', 'public']) {
    watch(join(root, dir), { recursive: true }, () => {
      clearTimeout(timer);
      timer = setTimeout(() => {
        try {
          build();
          console.log('Rebuilt.');
        } catch (err) {
          console.error(err.message);
        }
      }, 100);
    });
  }
}

async function resolve(urlPath) {
  const clean = normalize(decodeURIComponent(urlPath.split('?')[0])).replace(/^(\.\.[/\\])+/, '');
  let file = join(dist, clean);
  if (!file.startsWith(dist)) return null;
  try {
    if ((await stat(file)).isDirectory()) file = join(file, 'index.html');
    await stat(file);
    return file;
  } catch {
    return null;
  }
}

createServer(async (req, res) => {
  let file = await resolve(req.url);
  let code = 200;
  if (!file) {
    code = 404;
    file = join(dist, req.url.startsWith('/fr/') ? 'fr/404.html' : '404.html');
  }
  res.writeHead(code, { 'Content-Type': TYPES[extname(file)] ?? 'application/octet-stream' });
  res.end(await readFile(file));
}).listen(port, () => console.log(`Serving dist/ on http://localhost:${port}`));
