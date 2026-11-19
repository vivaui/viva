// Drives the browser verification fixture in the system Safari through
// safaridriver, which is the only way to exercise Apple's own browser rather
// than Playwright's WebKit build.
//
// One-time setup (requires an administrator):
//
//   sudo safaridriver --enable
//
// Then:
//
//   viva build --target web ./tools/webcheck/fixture -o build/webcheck
//   node tools/webcheck/safari.mjs --dir build/webcheck
//
// This is a smoke gate: it checks the renderer the page chose, a non-blank
// canvas, the bitmap pixel, and that a trusted pointer click reaches the
// application. The full interaction and IME suite remains in check.mjs; a real
// operating-system IME candidate window still needs a human on the target
// machine.
import { createServer } from 'node:http';
import { readFileSync } from 'node:fs';
import { extname, join, normalize } from 'node:path';
import { spawn } from 'node:child_process';
import { PNG } from 'pngjs';

const args = Object.fromEntries(process.argv.slice(2).map((a, i, all) => a.startsWith('--') ? [a.slice(2), all[i + 1]?.startsWith('--') || all[i + 1] === undefined ? true : all[i + 1]] : []).filter(e => e.length));
if (!args.dir) throw new Error('--dir is required');
const renderer = args.renderer ?? 'auto';
const port = +(args.port ?? 4444);
const root = normalize(args.dir);
const types = { '.html': 'text/html; charset=utf-8', '.js': 'text/javascript; charset=utf-8', '.wasm': 'application/wasm', '.json': 'application/json', '.png': 'image/png' };

const server = createServer((request, response) => {
  const pathname = decodeURIComponent(new URL(request.url, 'http://x').pathname);
  const path = normalize(join(root, pathname));
  if (!path.startsWith(root)) { response.writeHead(403).end(); return; }
  const file = path.endsWith('/') ? join(path, 'index.html') : path;
  try {
    const body = readFileSync(file);
    response.writeHead(200, { 'content-type': types[extname(file)] ?? 'application/octet-stream', 'cache-control': 'no-store' });
    response.end(body);
  } catch {
    response.writeHead(404).end('not found');
  }
});
await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
const pageURL = `http://127.0.0.1:${server.address().port}/?viva-renderer=${renderer}`;

const driver = spawn('safaridriver', ['-p', String(port)], { stdio: ['ignore', 'pipe', 'pipe'] });
let driverLog = '';
driver.stdout.on('data', chunk => { driverLog += chunk; });
driver.stderr.on('data', chunk => { driverLog += chunk; });
const cleanup = () => { server.close(); driver.kill('SIGTERM'); };
process.on('exit', cleanup);

async function waitForDriver() {
  for (let attempt = 0; attempt < 60; attempt++) {
    try {
      const response = await fetch(`http://127.0.0.1:${port}/status`);
      if (response.ok) return;
    } catch {}
    await new Promise(resolve => setTimeout(resolve, 250));
  }
  throw new Error('safaridriver did not start: ' + driverLog.trim());
}

async function webdriver(method, path, body) {
  const response = await fetch(`http://127.0.0.1:${port}${path}`, {
    method,
    headers: { 'content-type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const data = await response.json();
  if (!response.ok || data.value?.error) {
    throw new Error(`${method} ${path}: ${data.value?.error ?? response.status} ${data.value?.message ?? ''}`.trim());
  }
  return data.value;
}

function check(ok, message) {
  if (!ok) throw new Error(message);
  console.log(`ok  ${message}`);
}

async function execute(session, script) {
  return webdriver('POST', `/session/${session}/execute/sync`, { script, args: [] });
}

async function waitFor(session, script, message) {
  for (let attempt = 0; attempt < 120; attempt++) {
    if (await execute(session, script)) { check(true, message); return; }
    await new Promise(resolve => setTimeout(resolve, 100));
  }
  throw new Error('timed out: ' + message);
}

try {
  await waitForDriver();
  let session;
  try {
    const created = await webdriver('POST', '/session', { capabilities: { alwaysMatch: { browserName: 'safari' } } });
    session = created.sessionId;
  } catch (error) {
    console.error('Safari remote automation is not enabled. Run "sudo safaridriver --enable" once, then retry.');
    console.error(String(error.message || error));
    process.exit(1);
  }
  try {
    await webdriver('POST', `/session/${session}/url`, { url: pageURL });
    await waitFor(session, `return !!document.querySelector('[role=button][aria-label^="Count:"]')`, 'the page mounted the application');
    const chosen = await execute(session, `return document.documentElement.dataset.vivaRenderer`);
    check(renderer === 'auto' ? chosen === 'webgl2' || chosen === 'canvas2d' : chosen === renderer, `the page chose the ${renderer} renderer`);

    // A trusted click through WebDriver actions, not a DOM-dispatched event.
    await webdriver('POST', `/session/${session}/actions`, {
      actions: [{
        type: 'pointer', id: 'mouse', parameters: { pointerType: 'mouse' },
        actions: [
          { type: 'pointerMove', duration: 0, x: 110, y: 40 },
          { type: 'pointerDown', button: 0 },
          { type: 'pointerUp', button: 0 },
        ],
      }],
    });
    await waitFor(session, `return document.querySelector('[role=button][aria-label^="Count:"]')?.getAttribute('aria-label') === 'Count: 1'`, 'a trusted pointer click reaches the application');

    const base64 = await webdriver('GET', `/session/${session}/screenshot`);
    const shot = PNG.sync.read(Buffer.from(base64, 'base64'));
    const background = [shot.data[0], shot.data[1], shot.data[2]];
    let painted = 0;
    for (let index = 0; index < shot.data.length; index += 4) {
      if (Math.abs(shot.data[index] - background[0]) + Math.abs(shot.data[index + 1] - background[1]) + Math.abs(shot.data[index + 2] - background[2]) > 30) painted++;
    }
    check(painted > (shot.width * shot.height) / 50, 'the canvas is not blank');
    const imagePixel = (420 * 4) + (320 * shot.width * 4);
    check(shot.data[imagePixel] > 200 && shot.data[imagePixel + 1] < 80 && shot.data[imagePixel + 2] > 200, 'the bitmap image is painted');
    console.log(`Safari smoke check passed (${chosen})`);
  } finally {
    await webdriver('DELETE', `/session/${session}`).catch(() => {});
  }
} finally {
  cleanup();
}
