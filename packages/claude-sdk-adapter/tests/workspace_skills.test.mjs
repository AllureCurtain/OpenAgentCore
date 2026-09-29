import assert from 'node:assert/strict';
import test from 'node:test';
import { parseSkills } from '../dist/workspace_skills.js';

test('Runtime Skill paths remain inside the declared package; public metadata is not a path descriptor', () => {
  const skill = { metadata: { type: 'inline', name: 'proof', description: 'A proof.' }, relative_root: 'plugins/0/custom/proof', package_root: 'plugins/0' };
  assert.deepEqual(parseSkills([skill]), [skill]);
  for (const invalid of [
    { ...skill, relative_root: '/private' },
    { ...skill, relative_root: 'plugins/0/../../private' },
    { ...skill, relative_root: 'plugins/1/proof' },
    { ...skill, package_root: '.' },
    { ...skill, metadata: { ...skill.metadata, name: '../escape' } },
    skill.metadata,
  ]) assert.throws(() => parseSkills([invalid]), /invalid_request/);
  assert.throws(() => parseSkills([skill, skill]), /invalid_request/);
});

test('native Skill envelope stays under the resolved Runtime root', async t => {
  const { mkdtempSync, mkdirSync, writeFileSync, rmSync, statSync } = await import('node:fs');
  const { tmpdir } = await import('node:os');
  const { join } = await import('node:path');
  const { workspaceSkills } = await import('../dist/workspace_skills.js');
  const root = mkdtempSync(join(tmpdir(), 'oac-capability-root-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const source = join(root, 'plugins/0/custom/proof');
  mkdirSync(source, { recursive: true });
  writeFileSync(join(source, 'SKILL.md'), '---\nname: proof\ndescription: A proof.\n---\nUse this Skill.');
  const skill = { metadata: { type: 'inline', name: 'proof', description: 'A proof.' },
    relative_root: 'plugins/0/custom/proof', package_root: 'plugins/0' };
  const result = workspaceSkills([skill], root);
  assert.deepEqual(result.paths, [join(root, 'native/claude/environment-skills-0')]);
  const projected = join(result.paths[0], 'content/custom/proof/SKILL.md');
  assert.equal(statSync(projected).ino, statSync(join(source, 'SKILL.md')).ino);
  assert.deepEqual(workspaceSkills([skill], root), result);
});
