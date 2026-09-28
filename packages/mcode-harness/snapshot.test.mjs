import assert from 'node:assert/strict';
import test from 'node:test';
import { copyFile, mkdir, mkdtemp, readFile, rm, stat, symlink, writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { spawnSync } from 'node:child_process';

// This opt-in test requires an isolated Linux Runtime containing the built
// sandbox/worker artifact. It runs only native tools; no model is contacted.
const artifact = process.env.OAC_TEST_MCODE_NATIVE_ARTIFACT;
const isolated = process.env.OAC_TEST_MCODE_SNAPSHOT_ISOLATED === '1';
test('capability read exceptions preserve denied ancestors and immutable snapshots', {
  skip: !artifact || !isolated, timeout: 60_000,
}, async t => {
  const fixture = await mkdtemp(join(tmpdir(), 'oac-mcode-snapshot-'));
  t.after(() => rm(fixture, { recursive: true, force: true }));
  await copyFile(new URL('./launch.mjs', import.meta.url), join(fixture, 'launch.mjs'));
  await symlink(join(artifact, 'dist'), join(fixture, 'dist'));
  const quote = value => "'" + value.replaceAll("'", "'\\''") + "'";
  for (const [name, ancestor, root] of [
    ['standard', '/environment', '/environment/initialization/capabilities'],
    ['custom', join(fixture, 'private-layout'), join(fixture, 'private-layout', 'frozen', 'skills')],
  ]) {
    await t.test(name, async t => {
      const scratch = join(fixture, name + '-scratch');
      const secret = join(ancestor, 'private', 'snapshot-test-secret');
      await mkdir(root, { recursive: true });
      await mkdir(join(ancestor, 'private'), { recursive: true });
      await mkdir(scratch);
      const script = join(root, 'proof.sh'), attachment = join(root, 'attachment.txt');
      await writeFile(script, 'printf SNAPSHOT_SCRIPT_READABLE', { mode: 0o700 });
      await writeFile(attachment, 'SNAPSHOT_ATTACHMENT_READABLE', { mode: 0o600 });
      await writeFile(secret, 'PRIVATE_SNAPSHOT_CANARY', { mode: 0o600 });
      const profile = join(fixture, name + '.json');
      const config = { workspace: '/workspace', scratch, network: 'disabled',
        allowedDomains: [], protectedDirs: [ancestor], capabilityRoot: root, skills: true };
      await writeFile(profile, JSON.stringify(config));
      const executor = { execute(tool, input) {
        const child = spawnSync(process.execPath, [join(fixture, 'launch.mjs'), profile, '/workspace'], {
          input: JSON.stringify({ tool, input }), encoding: 'utf8', timeout: 15_000,
          env: { PATH: '/usr/local/bin:/usr/bin:/bin', HOME: '/tmp', LANG: 'C.UTF-8' },
        });
        assert.ok(child.stdout, child.stderr || String(child.error));
        return JSON.parse(child.stdout);
      } };
      const read = await executor.execute('bash', {
        command: 'sh ' + quote(script) + '; cat ' + quote(attachment),
      });
      assert.notEqual(read.isError, true, read.text);
      assert.ok(read.text.includes('SNAPSHOT_SCRIPT_READABLE'));
      assert.ok(read.text.includes('SNAPSHOT_ATTACHMENT_READABLE'));
      const protectedResult = await executor.execute('bash', {
        command: [
          'if printf changed >> ' + quote(attachment) + '; then exit 41; fi',
          'if chmod 0777 ' + quote(script) + '; then exit 42; fi',
          'if touch ' + quote(join(root, 'new-sibling')) + '; then exit 43; fi',
          'if cat ' + quote(secret) + '; then exit 44; fi',
          'printf SNAPSHOT_PROTECTION_OK',
        ].join('; '),
      });
      assert.notEqual(protectedResult.isError, true);
      assert.ok(protectedResult.text.includes('SNAPSHOT_PROTECTION_OK'));
      assert.ok(!protectedResult.text.includes('PRIVATE_SNAPSHOT_CANARY'));
      assert.equal(await readFile(attachment, 'utf8'), 'SNAPSHOT_ATTACHMENT_READABLE');
      assert.equal((await stat(script)).mode & 0o777, 0o700);
      await assert.rejects(stat(join(root, 'new-sibling')), { code: 'ENOENT' });
      // Negative control proves that the ancestor deny really masks this root.
      await writeFile(profile, JSON.stringify({ ...config, capabilityRoot: undefined, skills: false }));
      const hidden = await executor.execute('bash', { command: 'cat ' + quote(attachment) });
      assert.equal(hidden.isError, true);
      assert.ok(!hidden.text.includes('SNAPSHOT_ATTACHMENT_READABLE'));
    });
  }
});
