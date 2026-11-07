// Drives the browser verification fixture through Chrome DevTools Protocol.
//
//   viva build --target web ./tools/webcheck/fixture -o build/webcheck
//   node tools/webcheck/check.mjs --dir build/webcheck
//
// The checks cover the complete browser interaction path, not just painting:
// a pointer click, Enter on the focused button, Tab traversal, a captured
// slider drag delivered through Canvas.PointerOn, a screen-reader range change,
// and a pass-through backdrop that dismisses and then reaches the control
// beneath. Chrome is required locally; CI installs it on the runner.
import { spawn } from 'node:child_process';
import { createServer } from 'node:http';
import { existsSync } from 'node:fs';
import { mkdtempSync, readFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { extname, join, normalize } from 'node:path';

const args = Object.fromEntries(process.argv.slice(2).map((a, i, all) => a.startsWith('--') ? [a.slice(2), all[i + 1]?.startsWith('--') || all[i + 1] === undefined ? true : all[i + 1]] : []).filter(e => e.length));
const width = +(args.width ?? 640), height = +(args.height ?? 480);
const sleep = ms => new Promise(r => setTimeout(r, ms));

function findChrome() {
  const candidates = [
    args.chrome,
    process.env.CHROME,
    '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
    '/usr/bin/google-chrome',
    '/usr/bin/google-chrome-stable',
    '/usr/bin/chromium',
    '/usr/bin/chromium-browser',
  ].filter(Boolean);
  return candidates.find(p => typeof p === 'string' && existsSync(p));
}

function contentType(path) {
  switch (extname(path)) {
    case '.html': return 'text/html; charset=utf-8';
    case '.js': return 'text/javascript; charset=utf-8';
    case '.wasm': return 'application/wasm';
    case '.json': return 'application/json';
    case '.png': return 'image/png';
    default: return 'application/octet-stream';
  }
}

async function serve(dir) {
  if (!dir) throw new Error('--dir is required unless --url is given');
  const root = normalize(dir);
  const server = createServer((request, response) => {
    const pathname = decodeURIComponent(new URL(request.url, 'http://x').pathname);
    if (pathname === '/favicon.ico') { response.writeHead(204).end(); return; }
    const path = normalize(join(root, pathname));
    if (!path.startsWith(root)) { response.writeHead(403).end(); return; }
    const file = path.endsWith('/') || path === root ? join(path, 'index.html') : path;
    try {
      const body = readFileSync(file);
      response.writeHead(200, { 'content-type': contentType(file), 'cache-control': 'no-store' });
      response.end(body);
    } catch {
      response.writeHead(404).end('not found');
    }
  });
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  return { server, url: `http://127.0.0.1:${server.address().port}/` };
}

async function main() {
  const chromePath = findChrome();
  if (!chromePath) throw new Error('Google Chrome or Chromium was not found; set CHROME or pass --chrome');
  const served = args.url ? { url: args.url, server: null } : await serve(args.dir);
  const port = 9400 + Math.floor(Math.random() * 400);
  const chrome = spawn(chromePath, ['--headless=new', '--no-sandbox', '--disable-gpu',
    `--user-data-dir=${mkdtempSync(join(tmpdir(), 'viva-webcheck-'))}`,
    `--remote-debugging-port=${port}`, `--window-size=${width},${height}`, 'about:blank'], { stdio: 'ignore' });
  let ws;
  try {
    let targets;
    for (let i = 0; i < 80; i++) {
      try { targets = await (await fetch(`http://127.0.0.1:${port}/json`)).json(); if (targets.find(t => t.type === 'page')) break; } catch {}
      await sleep(200);
    }
    const target = targets?.find(t => t.type === 'page');
    if (!target) throw new Error('Chrome did not open a debugging target');
    ws = new WebSocket(target.webSocketDebuggerUrl);
    await new Promise((resolve, reject) => { ws.onopen = resolve; ws.onerror = reject; });
    let id = 0;
    const pending = new Map();
    const errors = [];
    ws.onmessage = event => {
      const message = JSON.parse(event.data);
      if (message.id && pending.has(message.id)) { pending.get(message.id)(message.result ?? message.error); pending.delete(message.id); }
      if (message.method === 'Runtime.exceptionThrown') errors.push(message.params.exceptionDetails.text);
      if (message.method === 'Log.entryAdded' && message.params.entry.level === 'error' && !message.params.entry.url?.endsWith('/favicon.ico')) errors.push(message.params.entry.text);
    };
    const send = (method, params = {}) => new Promise(resolve => { const i = ++id; pending.set(i, resolve); ws.send(JSON.stringify({ id: i, method, params })); });
    const evaluate = async expression => (await send('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true })).result?.value;
    const check = (ok, message) => { if (!ok) throw new Error(message); console.log(`ok  ${message}`); };
    const waitFor = async (expression, message, timeout = 5000) => {
      const started = Date.now();
      while (Date.now() - started < timeout) {
        if (await evaluate(expression)) { check(true, message); return; }
        await sleep(100);
      }
      throw new Error(`timed out: ${message}`);
    };
    const mouse = async (type, x, y, buttons) => send('Input.dispatchMouseEvent', { type, x, y, button: 'left', buttons: buttons ?? (type === 'mousePressed' || type === 'mouseMoved' ? 1 : 0), clickCount: 1 });
    const click = async (x, y) => { await mouse('mouseMoved', x, y, 0); await mouse('mousePressed', x, y); await mouse('mouseReleased', x, y, 0); await sleep(250); };
    const key = async (name, code, vk) => {
      await send('Input.dispatchKeyEvent', { type: 'keyDown', key: name, code, windowsVirtualKeyCode: vk, nativeVirtualKeyCode: vk });
      await send('Input.dispatchKeyEvent', { type: 'keyUp', key: name, code, windowsVirtualKeyCode: vk, nativeVirtualKeyCode: vk });
      await sleep(250);
    };
    const label = selector => evaluate(`document.querySelector(${JSON.stringify(selector)})?.getAttribute('aria-label') ?? ''`);
    const value = selector => evaluate(`document.querySelector(${JSON.stringify(selector)})?.getAttribute('aria-valuenow') ?? ''`);

    await send('Runtime.enable');
    await send('Log.enable');
    await send('Page.enable');
    await send('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile: false });
    await send('Page.navigate', { url: served.url });
    await waitFor(`!!document.querySelector('canvas')`, 'the page mounted a canvas', 10000);
    await waitFor(`[...document.querySelectorAll('[role=button]')].some(e => (e.getAttribute('aria-label') ?? '').startsWith('Count:'))`, 'the application published its controls');
    check(await evaluate(`(() => { const c = document.querySelector('canvas'); const d = c.getContext('2d').getImageData(0, 0, c.width, c.height).data; let painted = 0; for (let i = 0; i < d.length; i += 64) if (d[i] || d[i + 1] || d[i + 2] || d[i + 3]) painted++; return painted > (d.length / 64) / 10; })()`), 'the canvas is not blank');

    check(await label('[role=button][aria-label^="Count:"]') === 'Count: 0', 'the counter starts at zero');
    await click(110, 40);
    check(await label('[role=button][aria-label^="Count:"]') === 'Count: 1', 'a pointer click runs the click pass');
    await key('Enter', 'Enter', 13);
    check(await label('[role=button][aria-label^="Count:"]') === 'Count: 2', 'Enter activates the focused control');

    await key('Tab', 'Tab', 9);
    await key('Tab', 'Tab', 9);
    await key('ArrowRight', 'ArrowRight', 39);
    check(await value('[role=slider]') === '21', 'Tab reaches the slider and an arrow key changes it');

    await mouse('mouseMoved', 90, 116, 0);
    await mouse('mousePressed', 90, 116);
    await mouse('mouseMoved', 260, 116, 1);
    await mouse('mouseReleased', 260, 116, 0);
    await sleep(300);
    const dragged = +(await value('[role=slider]'));
    check(dragged > 60, `a captured drag through Canvas.PointerOn changed the value to ${dragged}`);

    await evaluate(`(() => { const e = document.querySelector('[role=slider]'); e.value = '80'; e.dispatchEvent(new Event('input', { bubbles: true })); return true; })()`);
    await sleep(400);
    check(await value('[role=slider]') === '80', 'a screen-reader range change reaches application state');

    await click(310, 40);
    check(await label('[role=button][aria-label^="Menu:"]') === 'Menu: true', 'the menu opened');
    check(await evaluate(`!!document.querySelector('[role=group][aria-label="Dismiss"]')`), 'the backdrop is published');
    await click(110, 40);
    check(await label('[role=button][aria-label^="Count:"]') === 'Count: 3', 'a pass-through backdrop dismisses and reaches the control beneath');
    check(!(await evaluate(`!!document.querySelector('[role=group][aria-label="Dismiss"]')`)), 'the pass-through backdrop closed');

    check(errors.length === 0, `the page reported no errors${errors.length ? ': ' + errors.join('; ') : ''}`);
    const version = (await (await fetch(`http://127.0.0.1:${port}/json/version`)).json()).Browser;
    console.log(`web check passed (${version})`);
  } finally {
    try { ws?.close(); } catch {}
    chrome.kill();
    served.server?.close();
  }
}

main().catch(error => { console.error(`web check failed: ${error.message}`); process.exit(1); });
