import { mkdtempSync, readFileSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { runScript, scriptPath } from './run';

const scriptThat = (body: string): string => {
  const dir = mkdtempSync(join(tmpdir(), 'run-'));
  const path = join(dir, 'probe.sh');
  writeFileSync(path, `set -eu\n${body}\n`);
  return path;
};

describe('[lib] run', () => {
  it('should resolve a script published under the package scripts directory', () => {
    expect(scriptPath('sync-versions.mjs')).toMatch(
      /dmpf-plugin[\\/]scripts[\\/]sync-versions\.mjs$/,
    );
  });

  it('should succeed when the script exits zero, with the arguments, the cwd and the extra env', async () => {
    const cwd = mkdtempSync(join(tmpdir(), 'run-cwd-'));
    const out = join(cwd, 'out.txt');
    const script = scriptThat(`printf '%s|%s|%s' "$1" "$PWD" "$PROBE" > ${out}`);

    const result = await runScript({ script, args: ['first'], cwd, env: { PROBE: 'value' } });

    expect(result).toEqual({ success: true });
    expect(readFileSync(out, 'utf-8')).toBe(`first|${cwd}|value`);
  });

  it('should fail when the script exits non-zero', async () => {
    const result = await runScript({ script: scriptThat('exit 3'), cwd: tmpdir() });

    expect(result).toEqual({ success: false });
  });
});
