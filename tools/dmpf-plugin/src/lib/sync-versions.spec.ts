import { execFileSync } from 'node:child_process';
import { mkdirSync, mkdtempSync, readFileSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { packageRoot } from './paths';

const SCRIPT = join(packageRoot(), 'scripts', 'sync-versions.mjs');
const SHA = 'b'.repeat(40);

const write = (root: string, path: string, content: string): void => {
  mkdirSync(dirname(join(root, path)), { recursive: true });
  writeFileSync(join(root, path), content);
};

const workspace = (): string => {
  const root = mkdtempSync(join(tmpdir(), 'sync-versions-'));
  write(root, 'go.work', 'go 1.26.6\n\nuse ./libs/backend/go/domain\n');
  write(root, 'apps/backend/bookings/Dockerfile', 'FROM golang:1.26.6-alpine AS build\n');
  write(
    root,
    'libs/backend/go/contracts/buf.gen.yaml',
    '      - google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12\n',
  );
  write(
    root,
    'nx.json',
    JSON.stringify({
      args: ['run', 'github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2'],
    }),
  );
  write(
    root,
    'libs/backend/go/domain/project.json',
    JSON.stringify({ command: 'go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...' }),
  );
  write(root, 'pnpm-workspace.yaml', 'catalog:\n  "@nx-go/nx-go": 4.0.0\n');
  write(
    root,
    'tools/dmpf-plugin/migrations.json',
    JSON.stringify({
      generators: {
        'sync-init': {
          version: '1.0.0-rc.1',
          implementation: './dist/migrations/sync-init/sync-init',
        },
      },
    }),
  );
  write(
    root,
    'tools/dmpf-plugin/versions.json',
    `${JSON.stringify(
      {
        schema: 'dmpf/versions@1',
        kernel: 'v1.0.0-rc.1',
        conformance: 'v1.0.0-rc.1',
        go: { directive: '1.26.5', image: 'golang:1.26.5-alpine' },
        buf: 'v1.70.0',
        protocGenGo: 'v1.36.11',
        golangciLint: 'v2.13.0',
        govulncheck: 'v1.6.0',
        nxGo: '3.0.0',
        workflowRef: '',
      },
      null,
      2,
    )}\n`,
  );
  return root;
};

const run = (root: string, ...args: string[]): { status: number; output: string } => {
  try {
    const output = execFileSync('node', [SCRIPT, '--root', root, ...args], {
      encoding: 'utf-8',
      stdio: 'pipe',
    });
    return { status: 0, output };
  } catch (error) {
    const failure = error as { status: number; stdout: string; stderr: string };
    return { status: failure.status, output: `${failure.stdout}${failure.stderr}` };
  }
};

const migrationsOf = (root: string): { generators: Record<string, { version: string }> } =>
  JSON.parse(readFileSync(join(root, 'tools/dmpf-plugin/migrations.json'), 'utf-8'));

const versionsOf = (root: string): Record<string, unknown> =>
  JSON.parse(readFileSync(join(root, 'tools/dmpf-plugin/versions.json'), 'utf-8'));

describe('[script] sync-versions', () => {
  it('should write the target version, the source commit and every pin read from its source', () => {
    const root = workspace();

    expect(run(root, '--target', '1.0.0-rc.2', '--workflow-ref', SHA).status).toBe(0);
    expect(versionsOf(root)).toEqual({
      schema: 'dmpf/versions@1',
      kernel: 'v1.0.0-rc.2',
      conformance: 'v1.0.0-rc.2',
      go: { directive: '1.26.6', image: 'golang:1.26.6-alpine' },
      buf: 'v1.70.0',
      protocGenGo: 'v1.36.12',
      golangciLint: 'v2.13.2',
      govulncheck: 'v1.7.0',
      nxGo: '4.0.0',
      workflowRef: SHA,
    });
  });

  it('should move every migration to the target version, so an upgrade runs each one once', () => {
    const root = workspace();

    expect(run(root, '--target', '1.0.0-rc.2').status).toBe(0);
    expect(migrationsOf(root).generators['sync-init'].version).toBe('1.0.0-rc.2');
  });

  it('should fail the check, naming the migration, when it is not at the kernel version', () => {
    const root = workspace();
    run(root, '--target', '1.0.0-rc.2');
    write(
      root,
      'tools/dmpf-plugin/migrations.json',
      JSON.stringify({ generators: { 'sync-init': { version: '1.0.0-rc.1' } } }),
    );
    const check = run(root, '--check');

    expect(check.status).toBe(1);
    expect(check.output).toContain('sync-init');
  });

  it('should write nothing when migrations.json is not JSON', () => {
    const root = workspace();
    const before = readFileSync(join(root, 'tools/dmpf-plugin/versions.json'), 'utf-8');
    write(root, 'tools/dmpf-plugin/migrations.json', '<<<<<<< HEAD\n');

    expect(run(root, '--target', '1.0.0-rc.2').status).toBe(2);
    expect(readFileSync(join(root, 'tools/dmpf-plugin/versions.json'), 'utf-8')).toBe(before);
  });

  it('should keep the kernel version and the source commit when given no target', () => {
    const root = workspace();

    expect(run(root).status).toBe(0);
    expect(versionsOf(root)).toMatchObject({
      kernel: 'v1.0.0-rc.1',
      workflowRef: '',
      buf: 'v1.70.0',
      govulncheck: 'v1.7.0',
    });
  });

  it('should pass the check after a write and fail it, naming the pin, when a source moves', () => {
    const root = workspace();
    run(root);

    expect(run(root, '--check').status).toBe(0);
    write(root, 'go.work', 'go 1.26.7\n\nuse ./libs/backend/go/domain\n');
    const check = run(root, '--check');

    expect(check.status).toBe(1);
    expect(check.output).toContain('go.directive');
  });

  it.each([
    [['--target', 'latest']],
    [['--workflow-ref', 'main']],
    [['--check', '--target', '1.0.0']],
  ])('should refuse %j without touching versions.json', (args) => {
    const root = workspace();
    const before = readFileSync(join(root, 'tools/dmpf-plugin/versions.json'), 'utf-8');

    expect(run(root, ...args).status).toBe(2);
    expect(readFileSync(join(root, 'tools/dmpf-plugin/versions.json'), 'utf-8')).toBe(before);
  });
});
