import assert from 'node:assert/strict';
import { existsSync, readdirSync, readFileSync, statSync } from 'node:fs';
import test from 'node:test';

const catalog = JSON.parse(readFileSync(new URL('../build/docs-data/catalog.json', import.meta.url), 'utf8'));
const output = new URL('../build/docs-site/', import.meta.url);
const readPage = path => readFileSync(new URL(`${path}index.html`, output), 'utf8');

const api = JSON.parse(readFileSync(new URL('../build/docs-data/api.json', import.meta.url), 'utf8'));
const guideDirectory = new URL('./content/guides/en/', import.meta.url);
const guideSlugs = readdirSync(guideDirectory).filter(name => name.endsWith('.md')).map(name => name.slice(0, -3));
const lang = locale => (locale === 'zh' ? 'zh-CN' : 'en');

test('the root redirects to the Chinese home page', () => {
  // Pages answers this at the edge; the page with the meta refresh is only a
  // fallback for hosts that ignore _redirects.
  const redirects = readFileSync(new URL('./_redirects', output), 'utf8');
  assert.match(redirects, /^\/\s+\/zh\/\s+302$/m, 'no edge redirect for the root');
  assert.ok(readPage('').includes('/zh/'));
});

test('navigation is client-routed and the stored theme survives it', () => {
  const html = readPage('zh/docs/getting-started/');
  // Without the client router every click loads a new document: measured at
  // 0.3-1.1 s per move against about 30 ms for a DOM swap on the same machine.
  assert.ok(html.includes('ClientRouter'), 'no client router script');
  // The router swaps in the new page's markup, which carries data-theme="light",
  // so a dark-theme reader would see the page flash white without this hook.
  assert.ok(html.includes('astro:after-swap'), 'no theme restore after a swap');
});

for (const locale of ['zh', 'en']) {
  test(`${locale} header offers a site-wide search`, () => {
    const html = readPage(`${locale}/`);
    assert.ok(html.includes('search-trigger'), 'no search trigger in the header');
    assert.ok(html.includes('SearchDialog'), 'the search dialog island is not in the header');
  });

  test(`${locale} search index covers components, guides and API`, () => {
    const index = JSON.parse(readFileSync(new URL(`./search/${locale}.json`, output), 'utf8'));
    const kinds = [...new Set(index.entries.map(entry => entry.kind))].sort();
    assert.deepEqual(kinds, ['api', 'component', 'guide']);
    assert.ok(index.entries.some(entry => entry.kind === 'component' && entry.href === `/${locale}/ui/button/`), 'no Button component');
    assert.ok(index.entries.some(entry => entry.kind === 'guide' && entry.href === `/${locale}/docs/getting-started/`), 'no getting-started guide');
    for (const entry of index.entries) assert.match(entry.href, /^\/[a-z]{2}\//, `bad href ${entry.href}`);
  });

  test(`${locale} header links the showcase and marks it current on its own page`, () => {
    const navigation = html => html.slice(html.indexOf('class="header-navigation"'), html.indexOf('class="header-end"'));
    const home = navigation(readPage(`${locale}/`));
    assert.ok(home.includes(`href="/${locale}/showcase/"`), 'no showcase link in the header');
    assert.ok(home.includes(locale === 'zh' ? '案例' : 'Showcase'), 'no showcase label in the header');
    assert.ok(!home.includes(`href="/${locale}/showcase/" aria-current`), 'showcase is current on the home page');
    const page = navigation(readPage(`${locale}/showcase/`));
    assert.match(page, new RegExp(`href="/${locale}/showcase/" aria-current="page"`), 'showcase is not current on its own page');
  });

  test(`${locale} showcase shows both applications as cards with their screenshots and links`, () => {
    const html = readPage(`${locale}/showcase/`);
    assert.ok(html.includes(`<html lang="${lang(locale)}"`));
    for (const language of ['zh', 'en']) assert.ok(html.includes(`href="/${language}/showcase/"`), `${language} switch`);
    assert.equal([...html.matchAll(/<h1[ >]/g)].length, 1);
    for (const name of ['Motty', 'Tinglan']) assert.ok(html.includes(name), `missing ${name}`);
    for (const href of ['https://motty.page', 'https://github.com/t8y2/motty', 'https://github.com/t8y2/motty/releases', 'https://github.com/talongao/tinglan', 'https://github.com/talongao/tinglan/issues']) {
      assert.ok(html.includes(href), `missing ${href}`);
    }
    for (const image of ['motty-hero', 'tinglan-hero']) assert.ok(html.includes(image), `missing ${image}`);
    assert.equal([...html.matchAll(/class="showcase-card-shot"/g)].length, 2, 'one screenshot per card');
    assert.equal([...html.matchAll(/class="showcase-card"/g)].length, 2, 'one card per case');
  });
}

test('unknown paths fail with a 404 page instead of the root redirect', () => {
  // Pages treats a project without a top-level 404.html as a single-page
  // application: every unmatched path answers with the root document and a 200,
  // so a broken link looks like a working redirect. Astro emits this file.
  assert.ok(existsSync(new URL('404.html', output)), 'no 404.html for unknown paths');
  const html = readFileSync(new URL('404.html', output), 'utf8');
  assert.ok(html.includes('<html lang="zh-CN"'));
  assert.ok(html.includes('404'));
  assert.ok(!html.includes('http-equiv="refresh"'), 'the 404 page is the root redirect');
  for (const language of ['zh', 'en']) {
    assert.ok(html.includes(`href="/${language}/"`), `404 page has no ${language} home link`);
    assert.ok(html.includes(`href="/${language}/components/"`), `404 page has no ${language} catalog link`);
  }
});

test('theme tokens stay namespaced so an extension cannot repaint the page', () => {
  // A browser extension that themes scrollbars declares its own
  // `:root { --background: url(...) }`. Extension styles load after the page's,
  // both selectors are `:root` at the same specificity, so a bare `--background`
  // let it replace the whole page background with its artwork. Every token this
  // site declares carries the product prefix for that reason, and these names
  // are the ones a page-level stylesheet is most likely to collide with.
  const cssDirectory = new URL('_astro/', output);
  const css = readdirSync(cssDirectory)
    .filter(name => name.endsWith('.css'))
    .map(name => readFileSync(new URL(name, cssDirectory), 'utf8'))
    .join('\n');
  assert.ok(css.includes('--viva-background:'), 'no namespaced page background token');
  for (const name of ['background', 'text', 'border', 'surface', 'surface-hover', 'muted', 'radius', 'strong-border', 'doc-secondary', 'brand', 'accent', 'accent-soft', 'error', 'success', 'mono', 'header-height']) {
    assert.doesNotMatch(css, new RegExp(`(?<![\\w-])--${name}\\s*:`), `the bare token --${name} is declared again`);
    assert.ok(!css.includes(`var(--${name})`), `something still reads the bare token --${name}`);
  }
});

for (const locale of ['zh', 'en']) {
  test(`${locale} home page states its numbers, its platforms and where the rest lives`, () => {
    const html = readPage(`${locale}/`);
    assert.ok(html.includes(`<html lang="${lang(locale)}"`));
    for (const language of ['zh', 'en']) assert.ok(html.includes(`href="/${language}/"`));
    for (const text of ['1,697,506', 'macOS', 'Windows', 'Linux']) assert.ok(html.includes(text), `missing ${text}`);
    assert.ok(html.includes(`href="/${locale}/docs/getting-started/"`));
    // The count is derived from the catalog, so a hardcoded number or a missing
    // stat fails here instead of drifting.
    assert.ok(html.includes(`<dd>${catalog.pages.length}</dd>`), 'the component count is missing');
    // The figures that have not reached their report's target live in the
    // performance guide; the landing page still has to lead a reader to them.
    assert.ok(html.includes(`href="/${locale}/docs/performance/"`), 'no link to the measurements that are not on the page');
    assert.ok(!html.includes('26.3–26.8 MiB'), 'a performance-guide figure is on the landing page');
    assert.equal([...html.matchAll(/<h1[ >]/g)].length, 1);
  });

  test(`${locale} footer carries the site map, the repository and the licence`, () => {
    const footer = readPage(`${locale}/`).slice(readPage(`${locale}/`).indexOf('class="site-footer"'));
    assert.ok(footer.includes('https://github.com/vivaui/viva'), 'no repository link');
    assert.ok(footer.includes('blob/main/LICENSE'), 'no licence link');
    assert.ok(footer.includes(`href="/${locale}/docs/"`), 'no guides link');
    assert.ok(footer.includes(`href="/${locale}/components/"`), 'no catalog link');
  });

  test(`${locale} documentation has an index and every guide`, () => {
    const index = readPage(`${locale}/docs/`);
    for (const slug of guideSlugs) {
      assert.ok(index.includes(`href="/${locale}/docs/${slug}/"`), `index lacks ${slug}`);
      const html = readPage(`${locale}/docs/${slug}/`);
      assert.ok(html.includes(`<html lang="${lang(locale)}"`));
      for (const language of ['zh', 'en']) assert.ok(html.includes(`href="/${language}/docs/${slug}/"`), `${slug}: no ${language} switch`);
      assert.equal([...html.matchAll(/<h1[ >]/g)].length, 1, `${slug}: one h1`);
    }
  });

  test(`${locale} API reference covers every package and type`, () => {
    const index = readPage(`${locale}/api/`);
    for (const pkg of api.packages) {
      assert.ok(index.includes(`href="/${locale}/api/${pkg.name}/"`), `index lacks ${pkg.name}`);
      const page = readPage(`${locale}/api/${pkg.name}/`);
      for (const type of pkg.types) {
        assert.ok(page.includes(`href="/${locale}/api/${pkg.name}/${type.name}/"`), `${pkg.name} lacks a link to ${type.name}`);
        assert.ok(existsSync(new URL(`${locale}/api/${pkg.name}/${type.name}/index.html`, output)));
      }
    }
  });

}

function htmlFiles(directory) {
  const found = [];
  for (const entry of readdirSync(directory)) {
    const path = new URL(`${entry}${statSync(new URL(entry, directory)).isDirectory() ? '/' : ''}`, directory);
    if (statSync(path).isDirectory()) found.push(...htmlFiles(path));
    else if (entry.endsWith('.html')) found.push(path);
  }
  return found;
}

test('every site-local link and image on every page resolves, including its #anchor', () => {
  const cache = new Map();
  const ids = path => {
    if (!cache.has(path)) cache.set(path, new Set([...readFileSync(new URL(path, output), 'utf8').matchAll(/\sid="([^"]+)"/g)].map(match => match[1])));
    return cache.get(path);
  };
  let checked = 0;
  for (const file of htmlFiles(new URL('zh/', output)).concat(htmlFiles(new URL('en/', output)))) {
    const html = readFileSync(file, 'utf8');
    const source = file.pathname.slice(output.pathname.length);
    for (const match of html.matchAll(/(?:href|src)="(\/[^"]*)"/g)) {
      const [target, anchor] = match[1].split('#');
      const path = target.split('?')[0].slice(1);
      const resolved = path.endsWith('/') || path === '' ? `${path}index.html` : path;
      assert.ok(existsSync(new URL(resolved, output)), `${source}: missing ${match[1]}`);
      if (anchor && resolved.endsWith('.html')) assert.ok(ids(resolved).has(decodeURIComponent(anchor)), `${source}: no #${anchor} in ${resolved}`);
      checked++;
    }
  }
  assert.ok(checked > 10000);
});

test('no page repeats an id', () => {
  for (const file of htmlFiles(new URL('zh/', output)).concat(htmlFiles(new URL('en/', output)))) {
    const ids = [...readFileSync(file, 'utf8').matchAll(/\sid="([^"]+)"/g)].map(match => match[1]);
    const repeated = ids.filter((id, index) => ids.indexOf(id) !== index);
    assert.deepEqual(repeated, [], `${file.pathname.slice(output.pathname.length)} repeats ${repeated.slice(0, 3)}`);
  }
});

for (const locale of ['zh', 'en']) {
  for (const page of catalog.pages) {
    test(`${locale}/${page.ID} has localized links, every example and unique accessible IDs`, () => {
      const html = readPage(`${locale}/ui/${page.ID}/`);
      assert.ok(html.includes(`<html lang="${locale === 'zh' ? 'zh-CN' : 'en'}"`));
      for (const language of ['zh', 'en']) assert.ok(html.includes(`href="/${language}/ui/${page.ID}/"`));
      assert.doesNotMatch(html, /<select\b/);
      assert.equal([...html.matchAll(/class="example-section"/g)].length, page.Examples.length);
      assert.equal([...html.matchAll(/class="example-demo"/g)].length, page.Examples.length);
      assert.equal([...html.matchAll(/class="code-block usage-block"/g)].length, page.Examples.length);
      for (const example of page.Examples) {
        assert.ok(html.includes(`id="example-${example.ID}"`));
      }
      const ids = [...html.matchAll(/\sid="([^"]+)"/g)].map(match => match[1]);
      assert.equal(new Set(ids).size, ids.length);
      for (const match of html.matchAll(/href="(\/[^"?#]+)"/g)) {
        const path = match[1].slice(1);
        assert.ok(existsSync(new URL(path.endsWith('/') ? `${path}index.html` : path, output)), `Missing link: ${path}`);
      }
    });
  }

  test(`${locale} components index links every component and switches language`, () => {
    const html = readPage(`${locale}/components/`);
    assert.ok(html.includes(`<html lang="${lang(locale)}"`));
    for (const language of ['zh', 'en']) assert.ok(html.includes(`href="/${language}/components/"`));
    for (const page of catalog.pages) assert.ok(html.includes(`href="/${locale}/ui/${page.ID}/"`), `index lacks ${page.ID}`);
    assert.ok(html.includes('class="component-index-grid"'));
    assert.ok(html.includes(locale === 'zh' ? '按钮' : 'Button'));
  });

  test(`${locale} catalog index hands the sidebar navigation, not page bodies`, () => {
    const html = readPage(`${locale}/components/`);
    // The sidebar island needs stable IDs, names and categories. Passing whole
    // pages duplicated every example, section and capability into the HTML, and
    // each page also embeds the full component list, so the index shipped a
    // megabyte of payload that the browser had to parse to show its navigation.
    assert.equal([...html.matchAll(/&quot;Category&quot;/g)].length, catalog.pages.length);
    assert.ok(html.includes(`&quot;${catalog.pages[0].ID}&quot;`));
    for (const field of ['Summary', 'Examples', 'Sections', 'Capabilities', 'InstallCommand']) {
      assert.ok(!html.includes(`&quot;${field}&quot;`), `sidebar island receives ${field}`);
    }
  });
}

// A crawler or an assistant that has only one URL still needs to learn which URL
// it is: the canonical, the translations and the language-neutral default.
test('every page is its own canonical and names every translation', () => {
  let checked = 0;
  for (const file of htmlFiles(new URL('zh/', output)).concat(htmlFiles(new URL('en/', output)))) {
    const html = readFileSync(file, 'utf8');
    const path = file.pathname.slice(output.pathname.length).replace(/index\.html$/, '');
    const source = `/${path}`;
    assert.equal(html.match(/<link rel="canonical" href="([^"]+)"/)?.[1], `https://vivaui.dev${source}`, `${source}: wrong canonical`);
    for (const language of ['zh', 'en']) {
      const expected = `https://vivaui.dev/${language}/${path.replace(/^[a-z]{2}\//, '')}`;
      assert.ok(html.includes(`hreflang="${lang(language)}" href="${expected}"`), `${source}: no ${language} alternate`);
    }
    assert.ok(html.includes(`hreflang="x-default" href="https://vivaui.dev/zh/${path.replace(/^[a-z]{2}\//, '')}"`), `${source}: no x-default`);
    for (const property of ['og:title', 'og:description', 'og:url', 'og:locale']) assert.ok(html.includes(`property="${property}"`), `${source}: no ${property}`);
    assert.ok(html.includes('name="twitter:card"'), `${source}: no twitter card`);
    checked++;
  }
  assert.ok(checked > 900, `only ${checked} pages checked`);
});

const structuredData = html => {
  const block = html.match(/<script type="application\/ld\+json">([\s\S]*?)<\/script>/)?.[1];
  assert.ok(block, 'no JSON-LD block');
  return JSON.parse(block)['@graph'];
};

test('structured data names the product on the home page and each guide', () => {
  for (const locale of ['zh', 'en']) {
    const home = structuredData(readPage(`${locale}/`));
    const types = home.map(node => node['@type']);
    for (const type of ['Organization', 'WebSite', 'SoftwareApplication']) assert.ok(types.includes(type), `${locale} home lacks ${type}`);
    const software = home.find(node => node['@type'] === 'SoftwareApplication');
    assert.equal(software.applicationCategory, 'DeveloperApplication');
    assert.equal(software.operatingSystem, 'macOS, Windows, Linux');
    assert.ok(software.codeRepository.endsWith('/vivaui/viva'));

    const guide = structuredData(readPage(`${locale}/docs/getting-started/`));
    const article = guide.find(node => node['@type'] === 'TechArticle');
    assert.equal(article.headline, locale === 'zh' ? '开始使用' : 'Get started');
    const breadcrumb = guide.find(node => node['@type'] === 'BreadcrumbList');
    assert.deepEqual(breadcrumb.itemListElement.map(entry => entry.position), [1, 2, 3]);
    assert.equal(new Set(guide.map(node => node['@id'])).size, guide.length, 'duplicate @id in one graph');
  }
});

test('a guide leads with its h1, not with the sidebar labels', () => {
  for (const locale of ['zh', 'en']) {
    for (const slug of guideSlugs) {
      const html = readPage(`${locale}/docs/${slug}/`);
      assert.equal(html.match(/<h([1-6])[ >]/)?.[1], '1', `${locale}/${slug}: the first heading is not the h1`);
    }
  }
});

test('the showcase names every application it presents', () => {
  for (const locale of ['zh', 'en']) {
    const graph = structuredData(readPage(`${locale}/showcase/`));
    const applications = graph.filter(node => node['@type'] === 'SoftwareApplication');
    assert.deepEqual(applications.map(node => node.name), ['Motty', 'Tinglan']);
    assert.ok(applications[0].codeRepository.endsWith('/t8y2/motty'));
    assert.equal(applications[1].license, 'https://www.gnu.org/licenses/agpl-3.0.html');
    assert.equal(new Set(graph.map(node => node['@id'])).size, graph.length, 'duplicate @id in one graph');
  }
});

test('the home page defines the product in its body text, not only in metadata', () => {
  assert.ok(readPage('en/').includes('is a Go framework for native desktop applications'), 'the English lead never defines Viva');
  assert.ok(readPage('zh/').includes('是面向原生桌面应用的 Go 框架'), 'the Chinese lead never defines Viva');
});

test('every page opts out of Cloudflare email obfuscation around its body', () => {
  // The zone setting rewrites anything that looks like an address, and the
  // install command `go get github.com/vivaui/viva@v0.2.1` matches. The
  // documented marker keeps the setting on for the rest of the site.
  for (const locale of ['zh', 'en']) {
    for (const path of [`${locale}/`, `${locale}/docs/getting-started/`, `${locale}/api/`]) {
      const html = readPage(path);
      const start = html.indexOf('<!--email_off-->');
      const end = html.indexOf('<!--/email_off-->');
      assert.ok(start > 0 && end > start, `${path}: no email_off marker around the body`);
    }
  }
  // The version is the one the README names as current, so a release changes it in one place.
  const current = /The current release is `(v\d+\.\d+\.\d+)`/.exec(readFileSync(new URL('../README.md', import.meta.url), 'utf8'))?.[1];
  assert.ok(current, 'the README names no current release');
  assert.ok(readPage('en/docs/getting-started/').includes(`viva@${current}`), 'the install command lost its version');
});

test('the crawler files describe the site that was built', () => {
  const sitemap = readFileSync(new URL('sitemap.xml', output), 'utf8');
  for (const locale of ['zh', 'en']) {
    for (const slug of ['', ...guideSlugs]) {
      const path = slug ? `${locale}/docs/${slug}/` : `${locale}/`;
      assert.ok(sitemap.includes(`<loc>https://vivaui.dev/${path}</loc>`), `sitemap lacks ${path}`);
    }
  }
  assert.ok(sitemap.includes('hreflang="x-default"'), 'sitemap has no x-default alternate');
  assert.ok(sitemap.includes('<loc>https://vivaui.dev/en/showcase/</loc>'), 'sitemap lacks the showcase');

  const robots = readFileSync(new URL('robots.txt', output), 'utf8');
  assert.match(robots, /^User-agent: \*$/m, 'robots.txt allows no crawler explicitly');
  assert.match(robots, /^Allow: \/$/m, 'robots.txt does not allow the site');
  assert.match(robots, /^Sitemap: https:\/\/vivaui\.dev\/sitemap\.xml$/m, 'robots.txt names no sitemap');

  // llms.txt is the map an assistant reads instead of crawling every page, so it
  // lists the guides, the reference indexes and the showcase, and stops there.
  const llms = readFileSync(new URL('llms.txt', output), 'utf8');
  assert.ok(llms.startsWith('# Viva\n'), 'llms.txt has no title');
  assert.match(llms, /^> Viva is a Go framework/m, 'llms.txt has no one-line summary');
  for (const slug of guideSlugs) assert.ok(llms.includes(`https://vivaui.dev/en/docs/${slug}/`), `llms.txt lacks ${slug}`);
  assert.ok(llms.includes('https://vivaui.dev/en/components/') && llms.includes('https://vivaui.dev/en/api/'), 'llms.txt lacks the reference indexes');
  assert.ok(llms.includes('https://vivaui.dev/en/showcase/'), 'llms.txt lacks the showcase');
  assert.ok(!llms.includes('/en/ui/button/'), 'llms.txt lists every component page instead of the catalog');

  const headers = readFileSync(new URL('_headers', output), 'utf8');
  assert.ok(headers.includes('Strict-Transport-Security: max-age=31536000'), 'no HSTS header');
  assert.ok(headers.includes('X-Frame-Options: SAMEORIGIN'), 'no frame protection');
  assert.ok(headers.includes('max-age=31536000, immutable'), 'hashed assets are not cached for a year');
});
