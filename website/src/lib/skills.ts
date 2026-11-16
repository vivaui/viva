import { readdirSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';

// The agent skills live in docs/skills of the repository, where a checkout uses
// them directly. The website serves each file as it is and keeps no copy that
// could drift from it.
const root = () => resolve(process.cwd(), '../docs/skills');

// skillPath is where a skill file is served; file defaults to the briefing.
export const skillPath = (name: string, file = 'SKILL.md'): string => `/skills/${name}/${file}`;

export function skillNames(): string[] {
  return readdirSync(root(), { withFileTypes: true }).filter(entry => entry.isDirectory()).map(entry => entry.name).sort();
}

// skillFiles are the Markdown files a skill ships: SKILL.md and the pages it
// links to, such as recipes.md. The install step downloads all of them.
export function skillFiles(name: string): string[] {
  return readdirSync(resolve(root(), name), { withFileTypes: true })
    .filter(entry => entry.isFile() && entry.name.endsWith('.md'))
    .map(entry => entry.name)
    .sort();
}

export function readSkillFile(name: string, file: string): string {
  if (!skillFiles(name).includes(file)) throw new Error(`skill ${name} has no ${file}`);
  return readFileSync(resolve(root(), name, file), 'utf8');
}

export function readSkill(name: string): string {
  return readSkillFile(name, 'SKILL.md');
}

// A skill file opens with its own title, and the guide that shows it already has
// one; its sections then sit one level below the guide's "The skill in full".
export function nestSkillHtml(html: string): string {
  return html.replace(/<h1[^>]*>[\s\S]*?<\/h1>\s*/, '').replace(/<(\/?)h2(?=[\s>])/g, '<$1h3');
}
