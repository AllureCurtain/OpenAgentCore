import '@modelcontextprotocol/sdk/server/index.js';
import { accessSync, readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';

if (!['linux', 'darwin'].includes(process.platform)) throw new Error('MiniMax native execution requires Linux or macOS');
for (const file of ['bridge.mjs', 'launch.mjs', 'tool-executor.mjs', 'dist/worker.mjs',
  'subagent-snapshot.mjs',
  'native/cli.js', 'native-patch.json']) accessSync(new URL(file, import.meta.url));
for (const command of ['bash', 'rg']) execFileSync('which', [command], { stdio: 'ignore' });
const pin = JSON.parse(readFileSync(new URL('source.json', import.meta.url)));
const patch = JSON.parse(readFileSync(new URL('native-patch.json', import.meta.url)));
if (patch.revision !== pin.revision || !/^[a-f0-9]{64}$/.test(patch.files?.['patch-native.mjs'] ?? '') ||
    !/^[a-f0-9]{64}$/.test(patch.files?.['native-subagent-admission.mjs'] ?? '')) throw new Error('Native admission patch provenance mismatch');
const tools = JSON.parse(execFileSync(process.execPath,
  [fileURLToPath(new URL('dist/worker.mjs', import.meta.url)), process.cwd(), '--describe'], { maxBuffer: 1024 * 1024 }));
if (tools.map(t => t.name).sort().join(',') !== 'bash,edit,glob,grep,read,write') throw new Error('Native tool inventory mismatch');
process.stdout.write(JSON.stringify({ protocol: 1, native: pin.version, source: pin.revision }) + '\n');
