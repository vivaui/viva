import { readFileSync, statSync } from 'node:fs';
import { resolve } from 'node:path';
import type { Locale } from './i18n';
import { apiPath } from './routes';

export interface DocInline { text: string; italic?: boolean; code?: boolean; href?: string }
export interface DocBlock { kind: 'paragraph' | 'heading' | 'code' | 'list'; text?: DocInline[]; items?: DocBlock[][]; code?: string; ordered?: boolean }
export interface APIValue { names: string[]; decl: string; doc: DocBlock[] }
export interface APIFunc { name: string; receiver?: string; decl: string; signature?: string; synopsis: string; doc: DocBlock[] }
export interface APIField { name: string; type: string; doc: DocBlock[] }
export interface APIType { name: string; decl: string; synopsis: string; doc: DocBlock[]; fields?: APIField[]; constants: APIValue[]; variables: APIValue[]; constructors: APIFunc[]; methods: APIFunc[] }
export interface APIPackage { name: string; importPath: string; synopsis: string; doc: DocBlock[]; constants: APIValue[]; variables: APIValue[]; functions: APIFunc[]; types: APIType[] }
export interface APIData { packages: APIPackage[] }

// api.json is parsed once per build, and again whenever the generator rewrites it,
// so a long static build does not parse megabytes of JSON once per page.
let cache: { modified: number; data: APIData } | undefined;
export function readAPI(): APIData {
  const filename = resolve(process.cwd(), '../build/docs-data/api.json');
  let modified: number;
  try {
    modified = statSync(filename).mtimeMs;
  } catch (error) {
    throw new Error(`Cannot read ${filename}. Run npm run generate first.`, { cause: error });
  }
  if (cache?.modified !== modified) {
    const data = JSON.parse(readFileSync(filename, 'utf8')) as APIData;
    if (!Array.isArray(data.packages) || !data.packages.length) throw new Error('The generated API data must contain packages.');
    cache = { modified, data };
  }
  return cache.data;
}

export function packageNamed(data: APIData, name: string): APIPackage {
  const found = data.packages.find(entry => entry.name === name);
  if (!found) throw new Error(`Unknown API package: ${name}`);
  return found;
}

export type APISymbolKind = 'type' | 'function' | 'constructor' | 'method';

export interface ResolvedAPISymbol {
  pkg: APIPackage;
  type?: APIType;
  func?: APIFunc;
  // owner is the type whose page anchors a constructor's or method's entry.
  owner?: APIType;
  kind: APISymbolKind;
}

// resolveAPISymbol finds "package.Name" in the generated API data. A name may
// be a type, a package function, or a constructor that go/doc attached to the
// type it returns (PaintToast belongs to ToastLayout).
export function resolveAPISymbol(data: APIData, symbol: string): ResolvedAPISymbol | undefined {
  const cut = symbol.indexOf('.');
  if (cut <= 0 || cut === symbol.length - 1) return undefined;
  const pkg = data.packages.find(entry => entry.name === symbol.slice(0, cut));
  if (!pkg) return undefined;
  const name = symbol.slice(cut + 1);
  const type = pkg.types.find(entry => entry.name === name);
  if (type) return { pkg, type, kind: 'type' };
  const fn = pkg.functions.find(entry => entry.name === name);
  if (fn) return { pkg, func: fn, kind: 'function' };
  for (const owner of pkg.types) {
    const constructor = owner.constructors.find(entry => entry.name === name);
    if (constructor) return { pkg, func: constructor, owner, kind: 'constructor' };
    const method = owner.methods.find(entry => entry.name === name);
    if (method) return { pkg, func: method, owner, kind: 'method' };
  }
  return undefined;
}

// typeHref links a field's Go type to its generated type page when it is a Viva
// type the API documents. Slices and pointers look through to their element
// type; primitives, time.Duration and other packages stay plain.
export function typeHref(data: APIData, pkgName: string, value: string, locale: Locale): string | undefined {
  let name = value.trim();
  while (name.startsWith('[]')) name = name.slice(2);
  while (name.startsWith('*')) name = name.slice(1);
  const parts = name.split('.');
  const targetPkg = parts.length === 2 ? parts[0] : pkgName;
  const typeName = parts.length === 2 ? parts[1] : name;
  if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(typeName)) return undefined;
  const pkg = data.packages.find(entry => entry.name === targetPkg);
  if (!pkg || !pkg.types.some(entry => entry.name === typeName)) return undefined;
  return apiPath(locale, targetPkg, typeName);
}

// A generated link such as "ui/Canvas#method-Hovered" or "app#func-RunCanvas"
// becomes a route under /<locale>/api/. External URLs pass through unchanged.
export function apiHref(locale: Locale, href: string): string {
  if (/^[a-z]+:\/\//i.test(href)) return href;
  const [page, anchor] = href.split('#');
  return apiPath(locale, ...page.split('/')) + (anchor ? `#${anchor}` : '');
}
