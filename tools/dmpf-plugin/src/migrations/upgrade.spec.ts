import { existsSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import type { Tree } from '@nx/devkit';
import { initGenerator, renderedPaths } from '../generators/init/generator';
import { packageRoot } from '../lib/paths';
import { type DmpfVersions, readPluginVersion, readVersions } from '../lib/versions';
import { consumerTree, renderedByAnOlderVersion, snapshot } from '../testing/rendered';
import syncInit from './sync-init/sync-init';
import updateVersions from './update-versions/update-versions';

jest.mock('../lib/versions', () => {
  const actual = jest.requireActual<typeof import('../lib/versions')>('../lib/versions');
  return {
    ...actual,
    readVersions: jest.fn(actual.readVersions),
    readPluginVersion: jest.fn(actual.readPluginVersion),
  };
});

const OPTIONS = { modulePrefix: 'github.com/acme/shop' };
const CONTEXT = ['apps/backend/shop/go.mod', 'apps/backend/shop/project.json'];
const ALWAYS_WRITTEN = ['dmpf.json', 'dmpf.rendered.json', 'go.work', 'nx.json', 'package.json'];

const addContext = (tree: Tree, versions: DmpfVersions): void => {
  tree.write(
    'apps/backend/shop/go.mod',
    `module github.com/acme/shop/apps/backend/shop\n\ngo 1.26\n\nrequire github.com/mateusmacedo/dmpf/libs/backend/go/domain ${versions.kernel}\n`,
  );
  tree.write(
    'apps/backend/shop/project.json',
    `${JSON.stringify({ command: `go run golang.org/x/vuln/cmd/govulncheck@${versions.govulncheck} ./...` }, null, 2)}\n`,
  );
};

describe('[migrations] upgrade from an older version', () => {
  const current = jest.requireActual<typeof import('../lib/versions')>('../lib/versions');
  const older: DmpfVersions = {
    ...current.readVersions(),
    kernel: 'v0.9.0',
    conformance: 'v0.9.0',
    go: { directive: '1.25.0', image: 'golang:1.25.0-alpine' },
    golangciLint: 'v2.0.0',
    govulncheck: 'v1.0.0',
    nxGo: '3.0.0',
    workflowRef: 'a'.repeat(40),
  };

  const released: DmpfVersions = { ...current.readVersions(), workflowRef: 'b'.repeat(40) };

  it('should leave the workspace equal to the output of init in this version', async () => {
    const paths = [...renderedPaths(), ...ALWAYS_WRITTEN, ...CONTEXT];
    jest.mocked(readVersions).mockReturnValue(released);
    const expected = consumerTree();
    await initGenerator(expected, OPTIONS);
    addContext(expected, released);
    jest.mocked(readVersions).mockReturnValue(older);
    jest.mocked(readPluginVersion).mockReturnValue('0.9.0');
    const tree = consumerTree();
    await initGenerator(tree, OPTIONS);
    addContext(tree, older);
    renderedByAnOlderVersion(tree, '.golangci.yml', 'version: "2"\n');
    jest.mocked(readVersions).mockReturnValue(released);
    jest.mocked(readPluginVersion).mockImplementation(current.readPluginVersion);
    expect(snapshot(tree, paths)).not.toEqual(snapshot(expected, paths));

    await syncInit(tree);
    const nextSteps = await updateVersions(tree);

    expect(snapshot(tree, paths)).toEqual(snapshot(expected, paths));
    expect(nextSteps).toEqual(['pnpm install', 'pnpm nx run-many -t tidy']);
  });
});

describe('[migrations] manifest', () => {
  const manifest = JSON.parse(readFileSync(join(packageRoot(), 'migrations.json'), 'utf-8'));
  const entries = Object.entries<{ version: string; implementation: string }>(manifest.generators);

  it('should register sync-init and update-versions at the kernel version of versions.json', () => {
    const version = readVersions().kernel.replace(/^v/, '');

    expect(entries.map(([name]) => name).sort()).toEqual(['sync-init', 'update-versions']);
    for (const [, entry] of entries) {
      expect(entry.version).toBe(version);
    }
  });

  it.each(entries)('should point %s to a migration compiled from src', (_name, entry) => {
    const source = entry.implementation.replace(/^\.\/dist\//, 'src/');

    expect(existsSync(join(packageRoot(), `${source}.ts`))).toBe(true);
  });
});
