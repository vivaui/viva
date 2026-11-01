// Measures where the browser backend spends a frame, in headless Chrome.
//
//   node tools/webperf/measure.mjs --url http://127.0.0.1:8080/ [--width 1800 --height 1047]
//        [--hover x0,y0,x1,y1] [--wheel x,y,deltaY] [--moves 120] [--chrome <path>] [--json]
//
// The page is loaded with window.vivaWebStats defined, which makes the runtime
// record each frame (see internal/platform/web/trace.go). The harness waits for
// the first frames, snapshots the running totals, then drives the pointer or wheel
// and reports the mean per frame since the snapshot. Numbers are local observations on the machine that
// ran them; always report the machine and Chrome version beside them.
import { spawn } from 'node:child_process';
import { mkdtempSync } from 'node:fs';
import { tmpdir, cpus } from 'node:os';
import { join } from 'node:path';

const args = Object.fromEntries(process.argv.slice(2).map((a, i, all) => a.startsWith('--') ? [a.slice(2), all[i + 1]?.startsWith('--') || all[i + 1] === undefined ? true : all[i + 1]] : []).filter(e => e.length));
const url = args.url;
if (!url) { console.error('usage: measure.mjs --url <page> [--hover x0,y0,x1,y1] [--wheel x,y,dy] [--moves N]'); process.exit(2); }
const width = +(args.width ?? 1800), height = +(args.height ?? 1047), moves = +(args.moves ?? 120);
const chromePath = args.chrome ?? '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const port = 9300 + Math.floor(Math.random() * 500);
const sleep = ms => new Promise(r => setTimeout(r, ms));

const chrome = spawn(chromePath, ['--headless=new', '--no-sandbox', `--user-data-dir=${mkdtempSync(join(tmpdir(), 'webperf-'))}`,
  `--remote-debugging-port=${port}`, `--window-size=${width},${height}`, 'about:blank'], { stdio: 'ignore' });
const finish = code => { chrome.kill(); process.exit(code); };

let targets;
for (let i = 0; i < 80; i++) {
  try { targets = await (await fetch(`http://127.0.0.1:${port}/json`)).json(); if (targets.find(t => t.type === 'page')) break; } catch {}
  await sleep(250);
}
const ws = new WebSocket(targets.find(t => t.type === 'page').webSocketDebuggerUrl);
await new Promise(r => { ws.onopen = r; });
let id = 0; const pending = new Map(); const errors = [];
ws.onmessage = e => {
  const m = JSON.parse(e.data);
  if (m.id && pending.has(m.id)) { pending.get(m.id)(m.result ?? m.error); pending.delete(m.id); }
  if (m.method === 'Runtime.exceptionThrown') errors.push(m.params.exceptionDetails.text);
};
const send = (method, params = {}) => new Promise(r => { const i = ++id; pending.set(i, r); ws.send(JSON.stringify({ id: i, method, params })); });
const evaluate = async expr => (await send('Runtime.evaluate', { expression: expr, returnByValue: true })).result?.value;

await send('Runtime.enable'); await send('Page.enable');
await send('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile: false });
await send('Page.addScriptToEvaluateOnNewDocument', { source: 'window.vivaWebStats = {};' });
await send('Page.navigate', { url });
await sleep(4000);

const keys = ['frames', 'totalMs', 'a11yMs', 'rectanglesMs', 'trianglesMs', 'textsMs'];
const read = () => evaluate('JSON.stringify(window.vivaWebStats)').then(JSON.parse);
let base = {};
const reset = async () => { base = await read(); };
const summarize = (name, now, wall) => {
  const s = { ...now };
  for (const k of keys) s[k] = (now[k] ?? 0) - (base[k] ?? 0);
  const n = s.frames || 1;
  return { scenario: name, frames: s.frames, perFrameMs: Object.fromEntries(keys.slice(1).map(k => [k.replace('Ms', ''), +(s[k] / n).toFixed(2)])),
    lastFrame: { rectangles: s.rectangles, triangles: s.triangles, texts: s.texts, hits: s.hits, pathsBuilt: s.pathsBuilt, pathsReused: s.pathsReused }, wallMs: wall };
};

const results = [];
const first = await read();
results.push(summarize('startup (all frames so far)', first, null));
const point = async (x, y) => { await send('Input.dispatchMouseEvent', { type: 'mouseMoved', x, y }); await sleep(8); };

if (args.hover) {
  const [x0, y0, x1, y1] = args.hover.split(',').map(Number);
  await reset(); const t0 = Date.now();
  for (let k = 0; k < moves; k++) { const f = (k % 40) / 40; await point(x0 + (x1 - x0) * ((k * 7 % 40) / 40), y0 + (y1 - y0) * f); }
  await sleep(500);
  results.push(summarize('hover sweep', await read(), Date.now() - t0));
}
if (args.wheel) {
  const [x, y, dy] = args.wheel.split(',').map(Number);
  await point(x, y); await reset(); const t0 = Date.now();
  for (let k = 0; k < moves; k++) { await send('Input.dispatchMouseEvent', { type: 'mouseWheel', x, y, deltaX: 0, deltaY: k % 20 < 10 ? dy : -dy }); await sleep(8); }
  await sleep(500);
  results.push(summarize('wheel scroll', await read(), Date.now() - t0));
}

const version = (await (await fetch(`http://127.0.0.1:${port}/json/version`)).json()).Browser;
const report = { machine: `${cpus()[0].model} x${cpus().length}`, chrome: version, viewport: `${width}x${height}`, errors, results };
if (args.json) console.log(JSON.stringify(report, null, 2));
else {
  console.log(`${report.machine} | ${version} | ${report.viewport}${errors.length ? ' | PAGE ERRORS: ' + errors.join('; ') : ''}`);
  for (const r of results) console.log(`${r.scenario}: ${r.frames} frames${r.wallMs ? ` in ${r.wallMs} ms wall` : ''}; per frame ${JSON.stringify(r.perFrameMs)}; ${JSON.stringify(r.lastFrame)}`);
}
finish(0);
