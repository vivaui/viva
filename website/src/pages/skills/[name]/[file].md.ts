import type { APIRoute } from 'astro';
import { readSkillFile, skillFiles, skillNames } from '../../../lib/skills';

export function getStaticPaths() {
  return skillNames().flatMap(name => skillFiles(name).map(file => ({ params: { name, file: file.slice(0, -3) } })));
}

// The raw file, so `curl` can drop it into a project's skills directory. The
// route keeps /skills/<name>/SKILL.md and adds the pages beside it.
export const GET: APIRoute = ({ params }) =>
  new Response(readSkillFile(params.name!, `${params.file}.md`), { headers: { 'content-type': 'text/markdown; charset=utf-8' } });
