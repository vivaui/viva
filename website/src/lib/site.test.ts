import assert from 'node:assert/strict';
import { existsSync, readdirSync, readFileSync } from 'node:fs';
import test from 'node:test';
import { site } from './site.ts';
import { home } from './home.ts';
import { showcase } from './showcase.ts';
import { skillFiles, skillNames, skillPath } from './skills.ts';

const guides = new URL('../../content/guides/', import.meta.url);
const slugs = (locale: string) => readdirSync(new URL(`${locale}/`, guides)).filter(name => name.endsWith('.md')).map(name => name.slice(0, -3)).sort();
const read = (locale: string, slug: string) => readFileSync(new URL(`${locale}/${slug}.md`, guides), 'utf8');
const han = /\p{Script=Han}/u;

function walk(value: unknown, visit: (text: string) => void) {
  if (typeof value === 'string') visit(value);
  else if (Array.isArray(value)) value.forEach(item => walk(item, visit));
  else if (value && typeof value === 'object') Object.values(value).forEach(item => walk(item, visit));
}

test('site and home dictionaries have the same keys in both languages', () => {
  assert.deepEqual(Object.keys(site.zh).sort(), Object.keys(site.en).sort());
  assert.deepEqual(Object.keys(home.zh).sort(), Object.keys(home.en).sort());
  assert.equal(home.zh.principles.length, home.en.principles.length);
  assert.equal(home.zh.platforms.length, home.en.platforms.length);
  assert.deepEqual(home.zh.platforms.map(entry => entry.name), home.en.platforms.map(entry => entry.name));
});

test('showcase cases exist in both languages with the same links and facts', () => {
  assert.deepEqual(Object.keys(showcase.zh).sort(), Object.keys(showcase.en).sort());
  assert.deepEqual(showcase.zh.cases.map(entry => entry.slug), showcase.en.cases.map(entry => entry.slug));
  assert.ok(showcase.en.cases.length >= 2);
  for (const [index, english] of showcase.en.cases.entries()) {
    const chinese = showcase.zh.cases[index];
    assert.equal(chinese.facts.length, english.facts.length, `${english.slug}: facts`);
    assert.deepEqual(chinese.links.map(link => link.href), english.links.map(link => link.href), `${english.slug}: links`);
    // The schema is language-neutral: if the two dictionaries disagree, the page
    // publishes a different structured-data description per locale.
    assert.deepEqual(chinese.schema, english.schema, `${english.slug}: schema`);
  }
});

test('English showcase copy has no Chinese prose and the Chinese copy is Chinese', () => {
  walk(showcase.en, text => assert.doesNotMatch(text, han, text));
  assert.match(showcase.zh.summary, han);
  for (const entry of showcase.zh.cases) assert.match(entry.summary, han);
});

test('English chrome has no Chinese prose and Chinese chrome is Chinese', () => {
  walk(site.en, text => assert.doesNotMatch(text, han, text));
  walk(home.en, text => assert.doesNotMatch(text, han, text));
  for (const key of ['docsSummary', 'apiSummary'] as const) assert.match(site.zh[key], han);
  for (const key of ['title', 'summary', 'heading', 'lead', 'measuredIntro'] as const) assert.match(home.zh[key], han);
});

test('every guide exists in both languages with the same shape', () => {
  assert.deepEqual(slugs('zh'), slugs('en'));
  assert.ok(slugs('en').length >= 8);
  for (const slug of slugs('en')) {
    const [english, chinese] = [read('en', slug), read('zh', slug)];
    const front = (text: string, key: string) => new RegExp(`^${key}: (.+)$`, 'm').exec(text)?.[1];
    assert.equal(front(english, 'order'), front(chinese, 'order'), `${slug}: order`);
    assert.doesNotMatch(front(english, 'title')!, han);
    assert.match(front(chinese, 'title')!, han);
    const body = (text: string) => text.split(/^---$/m).slice(2).join('---');
    const count = (text: string, pattern: RegExp) => [...body(text).matchAll(pattern)].length;
    assert.equal(count(english, /^## /gm), count(chinese, /^## /gm), `${slug}: sections`);
    assert.equal(count(english, /^```/gm), count(chinese, /^```/gm), `${slug}: code fences`);
    assert.doesNotMatch(body(english).replace(/```[\s\S]*?```/g, ''), han, `${slug}: English guide contains Chinese prose`);
  }
});

test('front matter values need no YAML quoting', () => {
  for (const locale of ['zh', 'en']) {
    for (const slug of slugs(locale)) {
      for (const key of ['title', 'summary']) {
        const value = new RegExp(`^${key}: (.+)$`, 'm').exec(read(locale, slug))![1];
        assert.doesNotMatch(value, /: |^[\[\]{}&*!|>'"%@`#]| #/, `${locale}/${slug} ${key} would be misread as YAML`);
      }
    }
  }
});

test('both languages file each guide under the same group', () => {
  for (const slug of slugs('en')) {
    const group = (locale: string) => /^group: (\w+)$/m.exec(read(locale, slug))?.[1];
    assert.ok(['start', 'concepts', 'guides', 'distribute', 'more'].includes(group('en')!), `${slug}: group`);
    assert.equal(group('en'), group('zh'), `${slug}: group`);
  }
});

const examples = new URL('../../../examples/guides/', import.meta.url);
const marked = (text: string) => [...text.matchAll(/<!-- example: (\S+) -->\n```go\n([\s\S]*?)\n```/g)].map(match => ({ path: match[1], code: match[2] }));

test('the code a guide shows is the code the examples run', () => {
  let blocks = 0;
  for (const locale of ['zh', 'en']) {
    for (const slug of slugs(locale)) {
      const text = read(locale, slug);
      assert.doesNotMatch(text, /\{\{example:/, `${locale}/${slug} has an unexpanded example; run npm run sync:examples`);
      for (const block of marked(text)) {
        blocks++;
        assert.ok(existsSync(new URL(block.path, examples)), `${locale}/${slug}: no examples/guides/${block.path}`);
        assert.equal(block.code, readFileSync(new URL(block.path, examples), 'utf8').replace(/\n+$/, ''), `${locale}/${slug}: ${block.path} differs; run npm run sync:examples`);
      }
    }
  }
  assert.ok(blocks >= 20);
});

test('both languages show the same example files', () => {
  for (const slug of slugs('en')) {
    assert.deepEqual(marked(read('zh', slug)).map(block => block.path), marked(read('en', slug)).map(block => block.path), slug);
  }
});

test('every example program is explained by some guide', () => {
  const shown = new Set(slugs('en').flatMap(slug => marked(read('en', slug)).map(block => block.path)));
  for (const directory of readdirSync(examples, { withFileTypes: true }).filter(entry => entry.isDirectory())) {
    assert.ok([...shown].some(path => path.startsWith(`${directory.name}/`)), `examples/guides/${directory.name} appears in no guide`);
  }
});

test('guide order numbers are unique so the navigation order is not left to chance', () => {
  for (const locale of ['zh', 'en']) {
    const orders = slugs(locale).map(slug => /^order: (\d+)$/m.exec(read(locale, slug))![1]);
    assert.equal(new Set(orders).size, orders.length);
  }
});

test('guides link only within their own language', () => {
  for (const locale of ['zh', 'en']) {
    const other = locale === 'zh' ? 'en' : 'zh';
    for (const slug of slugs(locale)) assert.doesNotMatch(read(locale, slug), new RegExp(`\\]\\(/${other}/`), `${locale}/${slug} links into /${other}/`);
  }
});

test('every skill is named like its directory and the coding-agent guide installs every file it ships', () => {
  const names = skillNames();
  assert.ok(names.length >= 1);
  for (const name of names) {
    const text = readFileSync(new URL(`../../../docs/skills/${name}/SKILL.md`, import.meta.url), 'utf8');
    assert.match(text, new RegExp(`^---\\nname: ${name}\\n`), `${name}: front matter name`);
    const files = skillFiles(name);
    assert.ok(files.includes('SKILL.md'), `${name}: no SKILL.md`);
    for (const locale of ['zh', 'en']) {
      const guide = read(locale, 'coding-agents');
      for (const file of files) {
        assert.ok(guide.includes(`https://vivaui.dev${skillPath(name, file)}`), `${locale}: install URL for ${name}/${file}`);
        assert.ok(guide.includes(`.claude/skills/${name}/${file}`), `${locale}: install target for ${name}/${file}`);
      }
    }
  }
});

test('a skill shown inside a guide gives up its title and nests its sections', async () => {
  const { nestSkillHtml } = await import('./skills.ts');
  assert.equal(nestSkillHtml('<h1 id="a">Title</h1>\n<p>x</p><h2 id="b">One</h2>'), '<p>x</p><h3 id="b">One</h3>');
});
