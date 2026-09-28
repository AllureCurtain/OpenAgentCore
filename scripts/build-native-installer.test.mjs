import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { chmod, lstat, mkdir, mkdtemp, readFile, rm, symlink, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import { buildBundle, copyComponent } from './build-native-installer.mjs';

async function fixture(t) {
  const root = await mkdtemp(join(tmpdir(), 'oac-native-bundle-test-'));
  t.after(() => rm(root, { recursive: true, force: true }));
  const source = join(root, 'source'); await mkdir(source);
  return { root, source, output: join(root, 'output') };
}

test('component preserves bytes and executable metadata with contained links flattened', async t => {
  const { source, output } = await fixture(t);
  await mkdir(join(source, 'lib'));
  await writeFile(join(source, 'lib', 'native'), 'native bytes');
  await chmod(join(source, 'lib', 'native'), 0o755);
  await symlink(join(source, 'lib'), join(source, 'linked'), process.platform === 'win32' ? 'junction' : 'dir');
  const files = await copyComponent(source, output);
  assert.equal(await readFile(join(output, 'linked/native'), 'utf8'), 'native bytes');
  assert.equal((await lstat(join(output, 'linked'))).isSymbolicLink(), false);
  assert.equal(files['linked/native'].sha256, createHash('sha256').update('native bytes').digest('hex'));
  if (process.platform !== 'win32') assert.equal(files['linked/native'].executable, true);
  assert.deepEqual(Object.keys(files).sort(), ['lib/native', 'linked/native']);
});

test('component rejects escaping directory links', async t => {
  const { root, source, output } = await fixture(t);
  await mkdir(join(root, 'outside')); await writeFile(join(root, 'outside/secret'), 'not bundled');
  await symlink(join(root, 'outside'), join(source, 'external'), process.platform === 'win32' ? 'junction' : 'dir');
  await assert.rejects(copyComponent(source, output), /escapes/);
});

test('component rejects cyclic links without recursing forever', async t => {
  const { source, output } = await fixture(t);
  await symlink(source, join(source, 'cycle'), process.platform === 'win32' ? 'junction' : 'dir');
  await assert.rejects(copyComponent(source, output), /cycle/);
});

test('bundle does not replace an existing output or accept relative paths', async t => {
  const { source, output } = await fixture(t);
  await mkdir(output); await writeFile(join(output, 'keep'), 'existing install');
  await assert.rejects(buildBundle({ daemon: join(source, 'daemon'), node: source, codex: source, output }), /already exists/);
  assert.equal(await readFile(join(output, 'keep'), 'utf8'), 'existing install');
  await assert.rejects(buildBundle({ daemon: 'relative', node: source, codex: source, output }), /absolute/);
});

test('bundle requires a selected harness and rejects output inside sources', async t => {
  const { source, output } = await fixture(t);
  await assert.rejects(buildBundle({ daemon: join(source, 'daemon'), node: source, output }), /Harness source/);
  await assert.rejects(buildBundle({ daemon: join(source, 'daemon'), node: source, codex: source, output: join(source, 'out') }), /outside/);
});

test('component rejects nonportable paths', { skip: process.platform === 'win32' }, async t => {
  const { source, output } = await fixture(t);
  await writeFile(join(source, 'bad:name'), 'bad');
  await assert.rejects(copyComponent(source, output), /non-portable/);
});

test('bundle rejects output entering a component through an aliased parent', async t => {
  const { root, source } = await fixture(t);
  const alias = join(root, 'alias');
  await symlink(source, alias, process.platform === 'win32' ? 'junction' : 'dir');
  await assert.rejects(buildBundle({ daemon: join(source, 'daemon'), node: source, codex: source, output: join(alias, 'new-parent', 'out') }), /outside/);
});
