import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { categoryLabel, componentPath, componentsPath, defaultLocale, isLocale, languagePath, locales, localizePage, messages } from './i18n.ts';
import type { Catalog } from './types.ts';

const catalog: Catalog = JSON.parse(readFileSync(new URL('../../../build/docs-data/catalog.json', import.meta.url), 'utf8'));

test('Chinese is the default and only explicitly supported locales are accepted', () => {
  assert.equal(defaultLocale, 'zh');
  assert.deepEqual(locales, ['zh', 'en']);
  for (const locale of locales) assert.ok(isLocale(locale));
  for (const value of ['', 'fr', 'zh-CN', 'EN', null, undefined, {}]) assert.equal(isLocale(value), false);
});

test('language links retain the component index or the current component', () => {
  for (const locale of locales) {
    assert.equal(componentPath(locale), componentsPath(locale));
    assert.equal(componentsPath(locale), `/${locale}/components/`);
    assert.equal(languagePath(locale, 'components'), componentsPath(locale));
    for (const page of catalog.pages) assert.equal(languagePath(locale, page.ID), `/${locale}/ui/${page.ID}/`);
  }
});

test('both message dictionaries have exactly the same nonempty keys', () => {
  assert.deepEqual(Object.keys(messages.zh).sort(), Object.keys(messages.en).sort());
  for (const locale of locales) {
    for (const [key, value] of Object.entries(messages[locale])) assert.ok(value.trim(), `${locale}/${key}`);
  }
});

test('every generated component, example, section and platform has both translations', () => {
  // Coverage lists the components package's source files; a page-layer control
  // (ui.List) has a page of its own without one.
  const componentPages = catalog.pages.filter(page => page.Source.startsWith('components/')).length;
  assert.equal(componentPages, catalog.coverage.length);
  assert.ok(catalog.pages.length > 60);
  assert.ok(catalog.pages.reduce((count, page) => count + page.Examples.length, 0) >= catalog.pages.length);
  for (const page of catalog.pages) {
    const snapshot = structuredClone(page);
    for (const locale of locales) {
      const translated = localizePage(page, locale);
      assert.ok(translated.Summary.trim());
      assert.equal(translated.Examples.length, page.Examples.length);
      assert.equal(translated.Sections.length, page.Sections.length);
      assert.equal(translated.Capabilities.length, page.Capabilities.length);
      for (const example of translated.Examples) {
        assert.ok(example.Title.trim());
        assert.ok(example.Description.trim());
      }
      for (const section of translated.Sections) {
        assert.ok(section.Title.trim());
        assert.ok(section.Text.trim());
      }
      for (const capability of translated.Capabilities) {
        assert.ok(capability.Status.trim());
        assert.ok(capability.Detail.trim());
      }
      assert.notEqual(categoryLabel('zh', page.Category), categoryLabel('en', page.Category));
    }
    assert.deepEqual(page, snapshot);
  }
});

test('translations retain identities, ordering and executable commands', () => {
  for (const page of catalog.pages) {
    for (const locale of locales) {
      const translated = localizePage(page, locale);
      for (const field of ['ID', 'Title', 'Revision', 'InstallCommand'] as const) {
        assert.equal(translated[field], page[field]);
      }
      assert.deepEqual(translated.Examples.map(example => example.ID), page.Examples.map(example => example.ID));
      assert.deepEqual(translated.Sections.map(section => section.ID), page.Sections.map(section => section.ID));
    }
  }
});

test('English explanations contain no untranslated Chinese prose', () => {
  for (const page of catalog.pages) {
    const translated = localizePage(page, 'en');
    const prose = [translated.Summary,
      ...translated.Examples.flatMap(example => [example.Title, example.Description]),
      ...translated.Sections.flatMap(section => [section.Title, section.Text, ...section.Items ?? []]),
      ...translated.Capabilities.flatMap(capability => [capability.Status, capability.Detail])];
    for (const value of prose) assert.doesNotMatch(value, /\p{Script=Han}/u);
  }
});

test('Chinese explanations are localized rather than silently falling back to English', () => {
  for (const page of catalog.pages) {
    const translated = localizePage(page, 'zh');
    const prose = [translated.Summary,
      ...translated.Examples.flatMap(example => [example.Title, example.Description]),
      ...translated.Sections.flatMap(section => [section.Title, section.Text, ...section.Items ?? []]),
      ...translated.Capabilities.flatMap(capability => [capability.Status, capability.Detail])];
    for (const value of prose) assert.match(value, /\p{Script=Han}/u);
  }
});

test('new component and example entries receive bounded generic English copy', () => {
  const page = catalog.pages[0];
  assert.doesNotMatch(localizePage({ ...page, ID: 'new-component' }, 'en').Summary, /\p{Script=Han}/u);
  assert.doesNotMatch(localizePage({ ...page, Examples: [{ ...page.Examples[0], ID: 'new-example' }] }, 'en').Examples[0].Title, /\p{Script=Han}/u);
  assert.throws(() => localizePage({ ...page, Sections: [{ ...page.Sections[0], ID: 'new-section' }] }, 'zh'), /Missing section translation/);
  assert.throws(() => localizePage({ ...page, Capabilities: [{ ...page.Capabilities[0], Platform: 'new-platform' }] }, 'zh'), /Missing platform translation/);
  assert.throws(() => categoryLabel('zh', 'new-category'), /Missing category translation/);
});
