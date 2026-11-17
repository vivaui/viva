import { defineConfig } from 'astro/config';
import svelte from '@astrojs/svelte';
import vivaPreview from './viva-preview.mjs';
import tailwindcss from '@tailwindcss/vite';
import { fileURLToPath } from 'node:url';
import { readFile, writeFile } from 'node:fs/promises';
import { locales, defaultLocale } from './src/lib/i18n.ts';

// Every absolute URL the site publishes is built from this: canonical links,
// hreflang, Open Graph and the sitemap all need one origin.
const site = 'https://vivaui.dev';

// Cloudflare Pages answers this before it looks at the assets, so the root is a
// redirect at the edge instead of the meta-refresh document that
// src/pages/index.astro emits for hosts that ignore it. A temporary redirect on
// purpose: the default language is a product choice, and a 301 to it would stay
// cached in browsers if that choice ever changes.
const redirects = [
  '# A temporary redirect, not a permanent one: the default language is a',
  '# product choice, and a cached 301 would outlive a change of that choice.',
  `/  /${defaultLocale}/  302`,
  '',
].join('\n');

// The locale a page is written in, and the same page in every other locale, so
// a sitemap entry can name its translations.
const translationOf = pathname => {
  const match = pathname.match(/^([a-z]{2})\/(.*)$/);
  if (!match || !locales.includes(match[1])) return null;
  const [, locale, rest] = match;
  return { locale, alternates: Object.fromEntries(locales.map(other => [other, `${other}/${rest}`])) };
};

const escape = value => value.replace(/[&<>"']/g, character => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[character]);
const text = value => value.replace(/&amp;/g, '&').replace(/&lt;/g, '<').replace(/&gt;/g, '>').replace(/&quot;/g, '"').replace(/&#39;/g, "'");

// The files an agent or a crawler reads before it reads a page. They are
// generated from the pages that were actually built, so a new guide cannot be
// missing from the sitemap and a renamed one cannot linger as a 404.
const siteFiles = () => ({
  name: 'site-files',
  hooks: {
    'astro:build:done': async ({ dir, pages }) => {
      const loc = path => new URL(path, site).href;
      const attribute = (html, name) => text(html.match(new RegExp(`<meta name="${name}" content="([^"]*)"`))?.[1] ?? '');
      // llms.txt is written in English, so its summary is the English page's own.
      const home = await readFile(new URL('en/index.html', dir), 'utf8').catch(() => '');
      const summary = attribute(home, 'description');

      // Pages a reader can land on: not the JSON search index, not the skill
      // markdown, not the 404 document.
      const indexable = pages
        .map(page => page.pathname.replace(/^\//, ''))
        .filter(pathname => /^[a-z]{2}\//.test(pathname) && pathname.endsWith('/'))
        .map(pathname => ({ pathname, ...translationOf(pathname) }))
        .filter(entry => entry.locale)
        .sort((left, right) => left.pathname.localeCompare(right.pathname));

      const sitemap = ['<?xml version="1.0" encoding="utf-8"?>', '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">'];
      for (const entry of indexable) {
        sitemap.push(`  <url><loc>${escape(loc(entry.pathname))}</loc>`);
        for (const [locale, path] of Object.entries(entry.alternates)) {
          sitemap.push(`    <xhtml:link rel="alternate" hreflang="${locale === 'zh' ? 'zh-CN' : locale}" href="${escape(loc(path))}"/>`);
        }
        sitemap.push(`    <xhtml:link rel="alternate" hreflang="x-default" href="${escape(loc(entry.alternates[defaultLocale]))}"/>`);
        sitemap.push('  </url>');
      }
      sitemap.push('</urlset>', '');
      await writeFile(new URL('sitemap.xml', dir), sitemap.join('\n'));

      await writeFile(new URL('robots.txt', dir), [
        '# The content-signal terms for this site live in the Cloudflare dashboard.',
        '# The directives below are the crawl rules those terms do not express.',
        '# Every AI crawler is allowed: this site exists to be read and quoted.',
        'User-agent: *',
        'Allow: /',
        '',
        `Sitemap: ${loc('sitemap.xml')}`,
        '',
      ].join('\n'));

      // llms.txt: the map an assistant reads instead of crawling every page. The
      // summaries are the pages' own descriptions, so the index says exactly what
      // the page says. Component detail pages are left to the catalog entry, and
      // the showcase is one entry: applications built on the framework.
      const entryOf = async pathname => {
        const html = await readFile(new URL(`${pathname}index.html`, dir), 'utf8');
        const title = text(html.match(/<title>([^<]*)<\/title>/)?.[1] ?? pathname).replace(/ · Viva$/, '');
        return `- [${title}](${loc(pathname)}): ${attribute(html, 'description')}`;
      };
      const english = indexable.filter(entry => entry.locale === 'en');
      const guides = english.filter(entry => entry.pathname.startsWith('en/docs/') && entry.pathname !== 'en/docs/');
      const catalog = english.filter(entry => entry.pathname === 'en/components/' || entry.pathname === 'en/api/');
      const showcase = english.filter(entry => entry.pathname === 'en/showcase/');
      const llms = [
        '# Viva',
        '',
        `> ${summary}`,
        '',
        'Viva is an open-source (Apache-2.0) Go framework for native desktop applications.',
        "It draws through each operating system's own window and GPU APIs, so an",
        'application links the system frameworks and the Go runtime and nothing else:',
        'no WebView, no browser engine, no bundled runtime, and nothing scheduled while',
        'it is idle. macOS, Windows and Linux are supported; a WebAssembly target exists',
        'as a secondary preview, not as part of the desktop runtime.',
        '',
        '- Repository: https://github.com/vivaui/viva',
        '- Module: github.com/vivaui/viva',
        `- Guides: ${loc('en/docs/')}`,
        '',
        '## Guides',
        '',
        ...await Promise.all(guides.map(entry => entryOf(entry.pathname))),
        '',
        '## Reference',
        '',
        ...await Promise.all(catalog.map(entry => entryOf(entry.pathname))),
        '',
        '## Showcase',
        '',
        ...await Promise.all(showcase.map(entry => entryOf(entry.pathname))),
        '',
        '## Agent skill',
        '',
        `- [building-with-viva skill](${loc('skills/building-with-viva/SKILL.md')}): the briefing an agent loads before changing a Viva application.`,
        `- [Recipe cheat sheet](${loc('skills/building-with-viva/recipes.md')}): which finished example to start from.`,
        '',
        '## Optional',
        '',
        `- [Documentation in Chinese](${loc('zh/docs/')}): the same guides, translated.`,
        '',
      ].join('\n');
      await writeFile(new URL('llms.txt', dir), llms);

      // Headers Pages applies at the edge. HTML stays revalidated on purpose: a
      // deploy must be visible immediately, and a page is about 10 KB compressed.
      // Hashed assets never change, so they never need revalidating.
      await writeFile(new URL('_headers', dir), [
        '/*',
        '  Strict-Transport-Security: max-age=31536000; includeSubDomains',
        '  X-Content-Type-Options: nosniff',
        '  X-Frame-Options: SAMEORIGIN',
        '  Referrer-Policy: strict-origin-when-cross-origin',
        '  Permissions-Policy: geolocation=(), microphone=(), camera=(), payment=()',
        '',
        '/_astro/*',
        '  Cache-Control: public, max-age=31536000, immutable',
        '',
      ].join('\n'));

      await writeFile(new URL('_redirects', dir), redirects);
    },
  },
});

export default defineConfig({
  site,
  output: 'static',
  trailingSlash: 'always',
  // Readers move between documentation pages constantly; a full document load
  // per click is what makes the site feel slower than a client-routed one.
  // Hover prefetching and the client router in BaseLayout turn a click into a
  // DOM swap instead of a new page load.
  prefetch: { prefetchAll: true, defaultStrategy: 'hover' },
  publicDir: '../build/docs-data/public',
  outDir: '../build/docs-site',
  markdown: { shikiConfig: { themes: { light: 'github-light', dark: 'github-dark' }, defaultColor: false } },
  integrations: [svelte(), siteFiles()],
  devToolbar: { enabled: false },
  vite: {
    plugins: [vivaPreview(), tailwindcss()],
    resolve: { alias: { $lib: fileURLToPath(new URL('./src/lib', import.meta.url)) } },
  },
});
