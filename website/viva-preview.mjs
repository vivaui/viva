import { spawn } from 'node:child_process';
import { createReadStream, statSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const root = fileURLToPath(new URL('../', import.meta.url));
const directories = ['app', 'ui', 'gfx', 'components', 'internal', 'cmd/component-preview', 'cmd/viva/templates', 'tools/docs'];
const previewDirectory = path.join(root, 'build/docs-data/public/preview');

export function previewAssetPath(requestURL, directory = previewDirectory) {
  let pathname;
  try {
    pathname = decodeURIComponent(new URL(requestURL, 'http://127.0.0.1').pathname);
  } catch {
    return null;
  }
  if (!pathname.startsWith('/preview/')) return null;
  const relative = pathname.slice('/preview/'.length);
  if (!relative || relative.includes('\0')) return null;
  const resolved = path.resolve(directory, relative);
  return resolved.startsWith(`${path.resolve(directory)}${path.sep}`) ? resolved : null;
}

function serveGeneratedPreview(request, response, next) {
  if (request.method !== 'GET' && request.method !== 'HEAD') return next();
  const filename = previewAssetPath(request.url ?? '');
  if (!filename) return next();
  let info;
  try {
    info = statSync(filename);
  } catch {
    return next();
  }
  if (!info.isFile()) return next();
  const contentTypes = { '.css': 'text/css; charset=utf-8', '.html': 'text/html; charset=utf-8', '.js': 'text/javascript; charset=utf-8', '.wasm': 'application/wasm' };
  response.statusCode = 200;
  response.setHeader('Cache-Control', 'no-cache');
  response.setHeader('Content-Length', String(info.size));
  response.setHeader('Content-Type', contentTypes[path.extname(filename)] ?? 'application/octet-stream');
  if (request.method === 'HEAD') return response.end();
  createReadStream(filename).pipe(response);
}

export function isPreviewSource(filename) {
  const relative = path.relative(root, filename).split(path.sep).join('/');
  if (relative.endsWith('_test.go')) return false;
  return relative === 'go.mod' || relative === 'go.sum' || directories.some(directory => relative.startsWith(`${directory}/`));
}

export function createPreviewRebuilder({ rebuild, reload, reportError, delay = 180 }) {
  let timer;
  let running = false;
  let pending = false;
  let closed = false;
  const controller = new AbortController();
  const drain = async () => {
    if (running || closed || !pending) return;
    running = true;
    pending = false;
    try {
      await rebuild(controller.signal);
      if (!closed && !pending) reload();
    } catch (error) {
      if (!closed && !pending) reportError(error);
    } finally {
      running = false;
      if (!closed && pending) timer = setTimeout(drain, delay);
    }
  };
  return {
    changed() {
      if (closed) return;
      pending = true;
      clearTimeout(timer);
      timer = setTimeout(drain, delay);
    },
    close() {
      closed = true;
      clearTimeout(timer);
      controller.abort();
    },
  };
}

function generate(signal) {
  return new Promise((resolve, reject) => {
    const child = spawn('go', ['run', './tools/docs'], { cwd: root, stdio: ['ignore', 'pipe', 'pipe'], detached: process.platform !== 'win32' });
    let output = '';
    const capture = chunk => { output = (output + chunk.toString()).slice(-16000); };
    child.stdout.on('data', capture);
    child.stderr.on('data', capture);
    const abort = () => {
      if (!child.pid) return;
      if (process.platform === 'win32') {
        spawn('taskkill', ['/PID', String(child.pid), '/T', '/F'], { stdio: 'ignore' }).on('error', () => child.kill());
      } else {
        try { process.kill(-child.pid, 'SIGTERM'); } catch (error) { if (error.code !== 'ESRCH') child.kill(); }
      }
    };
    signal.addEventListener('abort', abort, { once: true });
    if (signal.aborted) abort();
    child.on('error', error => {
      signal.removeEventListener('abort', abort);
      reject(error);
    });
    child.on('close', code => {
      signal.removeEventListener('abort', abort);
      if (code === 0) resolve();
      else reject(new Error(output || `Go preview generation exited with ${code}`));
    });
  });
}

export default function vivaPreview() {
  return {
    name: 'viva-source-preview',
    apply: 'serve',
    configureServer(server) {
      server.middlewares.use(serveGeneratedPreview);
      const rebuilder = createPreviewRebuilder({
        rebuild: generate,
        reload() {
          server.moduleGraph.invalidateAll();
          server.ws.send({ type: 'full-reload' });
          server.config.logger.info('Go preview rebuilt; component source and WASM updated.');
        },
        reportError(error) {
          server.config.logger.error(`Go preview build failed; previous preview retained.\n${error.message}`);
          server.ws.send({ type: 'error', err: { message: `Go preview build failed. The last successful preview is unchanged.\n${error.message}`, stack: '', plugin: 'viva-source-preview' } });
        },
      });
      const changed = filename => { if (isPreviewSource(filename)) rebuilder.changed(); };
      server.watcher.add([...directories, 'go.mod', 'go.sum'].map(filename => path.join(root, filename)));
      server.watcher.on('add', changed).on('change', changed).on('unlink', changed);
      const close = () => {
        rebuilder.close();
        server.watcher.off('add', changed).off('change', changed).off('unlink', changed);
      };
      server.httpServer?.once('close', close);
      server.watcher.once('close', close);
    },
  };
}
