// Drives the browser verification fixture through Playwright.
//
//   viva build --target web ./tools/webcheck/fixture -o build/webcheck
//   node tools/webcheck/check.mjs --dir build/webcheck --browser chromium
//
// The checks cover the complete browser interaction path, not just painting:
// a pointer click, Enter on the focused button, Tab traversal, a captured
// slider drag delivered through Canvas.PointerOn, a screen-reader range change,
// a pass-through backdrop, real keyboard text entry, and a synthetic IME
// composition. The same script runs against Chromium, Firefox and WebKit, the
// three engines Playwright ships; it does not drive a physical input method.
//
// The Playwright package lives in tools/webcheck/package.json so it cannot
// enter a native or browser application artifact. Run `npm ci --prefix
// tools/webcheck` and `npx --prefix tools/webcheck playwright install ...`
// once before using it.
import { createServer } from 'node:http';
import { existsSync } from 'node:fs';
import { readFileSync } from 'node:fs';
import { extname, join, normalize } from 'node:path';
import { chromium, firefox, webkit } from 'playwright';

const args = Object.fromEntries(process.argv.slice(2).map((a, i, all) => a.startsWith('--') ? [a.slice(2), all[i + 1]?.startsWith('--') || all[i + 1] === undefined ? true : all[i + 1]] : []).filter(e => e.length));
const width = +(args.width ?? 640), height = +(args.height ?? 480);
const engines = { chromium, firefox, webkit };
const requested = args.browser ?? 'chromium';
const names = requested === 'all' ? Object.keys(engines) : [requested];

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

function check(ok, message) {
  if (!ok) throw new Error(message);
  console.log(`ok  ${message}`);
}

async function run(name, url) {
  const engine = engines[name];
  if (!engine) throw new Error(`unknown browser ${name}; use chromium, firefox, webkit or all`);
  const browser = await engine.launch({ headless: true, ...(name === 'chromium' ? { chromiumSandbox: false } : {}) }).catch(error => {
    throw new Error(`${name} could not start (${error.message}); run "npx --prefix tools/webcheck playwright install ${name}"`);
  });
  const context = await browser.newContext({ viewport: { width, height }, deviceScaleFactor: 1 });
  const page = await context.newPage();
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  page.on('console', message => { if (message.type() === 'error') errors.push(message.text()); });

  const wait = async (fn, message, arg, timeout = 10000) => {
    try {
      await page.waitForFunction(fn, arg, { timeout });
    } catch {
      throw new Error(`timed out: ${message}`);
    }
    check(true, message);
  };
  const waitLabel = (selector, expected, message) => wait(({ selector, expected }) =>
    document.querySelector(selector)?.getAttribute('aria-label') === expected, message, { selector, expected });
  const waitAttribute = (selector, name, expected, message) => wait(({ selector, name, expected }) =>
    document.querySelector(selector)?.getAttribute(name) === expected, message, { selector, name, expected });
  const waitValue = (selector, expected, message) => wait(({ selector, expected }) =>
    document.querySelector(selector)?.value === expected, message, { selector, expected });
  const waitText = (selector, expected, message) => wait(({ selector, expected }) =>
    document.querySelector(selector)?.textContent === expected, message, { selector, expected });

  try {
    console.log(`# ${name} ${browser.version()}`);
    await page.goto(url);
    await wait(() => !!document.querySelector('canvas'), 'the page mounted a canvas');
    await page.waitForSelector('[role=button][aria-label^="Count:"]');
    check(await page.evaluate(() => {
      const canvas = document.querySelector('canvas');
      const data = canvas.getContext('2d').getImageData(0, 0, canvas.width, canvas.height).data;
      let painted = 0;
      for (let i = 0; i < data.length; i += 64) if (data[i] || data[i + 1] || data[i + 2] || data[i + 3]) painted++;
      return painted > (data.length / 64) / 10;
    }), 'the canvas is not blank');

    await waitLabel('[role=button][aria-label^="Count:"]', 'Count: 0', 'the counter starts at zero');
    await page.mouse.click(110, 40);
    await waitLabel('[role=button][aria-label^="Count:"]', 'Count: 1', 'a pointer click runs the click pass');
    await page.keyboard.press('Enter');
    await waitLabel('[role=button][aria-label^="Count:"]', 'Count: 2', 'Enter activates the focused control');

    await page.keyboard.press('Tab');
    await page.keyboard.press('Tab');
    await page.keyboard.press('ArrowRight');
    await waitAttribute('[role=slider]', 'aria-valuenow', '21', 'Tab reaches the slider and an arrow key changes it');

    await page.mouse.move(90, 116);
    await page.mouse.down();
    await page.mouse.move(260, 116, { steps: 5 });
    await page.mouse.up();
    await wait(() => +(document.querySelector('[role=slider]')?.getAttribute('aria-valuenow') ?? 0) > 60,
      'a captured drag through Canvas.PointerOn changed the slider');

    await page.evaluate(() => {
      const element = document.querySelector('[role=slider]');
      element.value = '80';
      element.dispatchEvent(new Event('input', { bubbles: true }));
    });
    await waitAttribute('[role=slider]', 'aria-valuenow', '80', 'a screen-reader range change reaches application state');

    // The text field is a real editable hit backed by a text.Editor, wired with
    // web.EditorInputs exactly as flex.Fields wires an application field.
    await page.mouse.click(100, 198);
    await wait(() => document.activeElement?.tagName === 'INPUT' &&
      document.activeElement?.getAttribute('aria-label') === 'Name',
      'clicking the text field focuses its browser editor');
    await page.keyboard.type('typed');
    await waitText('[role=status]', 'Text: typed', 'typed text reaches the Go-owned editor');

    const composition = await page.evaluate(() => {
      const element = document.querySelector('input[aria-label="Name"]');
      const base = element.value;
      const start = element.selectionStart ?? base.length;
      element.dispatchEvent(new CompositionEvent('compositionstart', { bubbles: true }));
      element.value = base.slice(0, start) + 'にほ' + base.slice(start);
      element.setSelectionRange(start, start + 2);
      element.dispatchEvent(new CompositionEvent('compositionupdate', { bubbles: true, data: 'にほ' }));
      return { base, start };
    });
    await waitText('[role=status]', 'Text: typedにほ', 'composition updates publish the marked text');
    await waitValue('input[aria-label="Name"]', 'typedにほ', 'the browser keeps its own preedit while composing');

    await page.evaluate(({ base, start }) => {
      const element = document.querySelector('input[aria-label="Name"]');
      const committed = '日本';
      element.value = base.slice(0, start) + committed + base.slice(start);
      element.setSelectionRange(start + committed.length, start + committed.length);
      element.dispatchEvent(new CompositionEvent('compositionend', { bubbles: true, data: committed }));
      // Browsers follow compositionend with an input event; the host must not
      // apply the committed text twice.
      element.dispatchEvent(new InputEvent('input', {
        bubbles: true, inputType: 'insertCompositionText', data: committed, isComposing: false,
      }));
    }, composition);
    await waitText('[role=status]', 'Text: typed日本', 'composition commits exactly once to the Go editor');
    await waitValue('input[aria-label="Name"]', 'typed日本', 'the editor remains the authority after composition');

    await page.keyboard.type('!');
    await waitText('[role=status]', 'Text: typed日本!', 'typing resumes after composition ends');

    // Canceling a composition removes its marked text and leaves the committed
    // text before it intact.
    const canceled = await page.evaluate(() => {
      const element = document.querySelector('input[aria-label="Name"]');
      const base = element.value;
      const start = element.selectionStart ?? base.length;
      element.dispatchEvent(new CompositionEvent('compositionstart', { bubbles: true }));
      element.value = base.slice(0, start) + 'か' + base.slice(start);
      element.setSelectionRange(start, start + 1);
      element.dispatchEvent(new CompositionEvent('compositionupdate', { bubbles: true, data: 'か' }));
      return { base };
    });
    await waitText('[role=status]', 'Text: typed日本!か', 'a second composition publishes its preedit');
    await page.evaluate(({ base }) => {
      const element = document.querySelector('input[aria-label="Name"]');
      element.value = base;
      element.setSelectionRange(base.length, base.length);
      element.dispatchEvent(new CompositionEvent('compositionend', { bubbles: true, data: '' }));
      element.dispatchEvent(new InputEvent('input', {
        bubbles: true, inputType: 'insertCompositionText', data: '', isComposing: false,
      }));
    }, canceled);
    await waitText('[role=status]', 'Text: typed日本!', 'canceling a composition removes its marked text');

    await page.mouse.click(310, 40);
    await waitLabel('[role=button][aria-label^="Menu:"]', 'Menu: true', 'the menu opened');
    await page.waitForSelector('[role=group][aria-label="Dismiss"]');
    check(true, 'the backdrop is published');
    await page.mouse.click(110, 40);
    await waitLabel('[role=button][aria-label^="Count:"]', 'Count: 3', 'a pass-through backdrop dismisses and reaches the control beneath');
    await page.waitForSelector('[role=group][aria-label="Dismiss"]', { state: 'detached' });
    check(true, 'the pass-through backdrop closed');

    check(errors.length === 0, `the page reported no errors${errors.length ? ': ' + errors.join('; ') : ''}`);
    console.log(`web check passed (${name} ${browser.version()})`);
  } finally {
    await browser.close();
  }
}

async function main() {
  if (!names.every(name => engines[name])) {
    throw new Error(`unknown browser ${requested}; use chromium, firefox, webkit or all`);
  }
  if (!args.dir && !args.url) throw new Error('--dir is required unless --url is given');
  if (args.dir && !existsSync(args.dir)) throw new Error(`--dir ${args.dir} does not exist`);
  const served = args.url ? { url: args.url, server: null } : await serve(args.dir);
  try {
    for (const name of names) await run(name, served.url);
  } finally {
    served.server?.close();
  }
}

main().catch(error => { console.error(`web check failed: ${error.message}`); process.exit(1); });
