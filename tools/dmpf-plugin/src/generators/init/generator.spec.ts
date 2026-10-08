import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { existsSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import type { Tree } from '@nx/devkit';
import { readJson } from '@nx/devkit';
import { createTreeWithEmptyWorkspace } from '@nx/devkit/testing';
import { packageRoot } from '../../lib/paths';
import { readPluginVersion, readVersions } from '../../lib/versions';
import { initGenerator, renderedPaths } from './generator';

jest.mock('node:child_process', () => ({
  ...jest.requireActual<typeof import('node:child_process')>('node:child_process'),
  execFileSync: jest.fn(),
}));

jest.mock('@nx/devkit', () => ({
  ...jest.requireActual<typeof import('@nx/devkit')>('@nx/devkit'),
  installPackagesTask: jest.fn(),
}));

const KERNEL = 'github.com/mateusmacedo/dmpf';

const consumerTree = (): Tree => {
  const tree = createTreeWithEmptyWorkspace();
  tree.write('package.json', JSON.stringify({ name: '@acme/shop', devDependencies: {} }));
  return tree;
};

const snapshot = (tree: Tree, paths: readonly string[]): Record<string, string | null> =>
  Object.fromEntries(paths.map((path) => [path, tree.read(path, 'utf-8')]));

const ALWAYS_WRITTEN = ['dmpf.json', 'dmpf.rendered.json', 'go.work', 'nx.json', 'package.json'];

beforeEach(() => jest.mocked(execFileSync).mockClear());

describe('[generator] init — fresh consumer', () => {
  it('should write dmpf.json with the options and the defaults derived from the module prefix', async () => {
    const tree = consumerTree();

    await initGenerator(tree, { modulePrefix: 'github.com/acme/shop' });

    expect(readJson(tree, 'dmpf.json')).toEqual({
      schema: 'dmpf/workspace@1',
      modulePrefix: 'github.com/acme/shop',
      npmScope: '@acme',
      appsDir: 'apps/backend',
      edge: '',
      spiffeTrustDomain: 'dmpf',
      composeProfile: 'dmpf',
      composeProject: 'shop',
      imageRegistry: 'ghcr.io/acme',
      bufModule: 'buf.build/acme',
      tooling: { mode: 'version' },
    });
  });

  it('should reserve the module prefix when nx add runs init without options', async () => {
    const tree = consumerTree();

    await initGenerator(tree, {});

    expect(readJson(tree, 'dmpf.json').modulePrefix).toBe('example.com/change-me');
  });

  it('should take an empty module prefix, as an Enter at the prompt gives, as no prefix', async () => {
    const tree = consumerTree();
    await initGenerator(tree, { modulePrefix: '' });
    const initialized = consumerTree();
    await initGenerator(initialized, { modulePrefix: 'github.com/acme/shop' });

    await initGenerator(initialized, { modulePrefix: '' });

    expect(readJson(tree, 'dmpf.json').modulePrefix).toBe('example.com/change-me');
    expect(readJson(initialized, 'dmpf.json').modulePrefix).toBe('github.com/acme/shop');
  });

  it('should keep a go directive newer than the one of versions.json', async () => {
    const tree = consumerTree();
    tree.write('go.work', 'go 1.99.0\n');

    await initGenerator(tree, { modulePrefix: 'github.com/acme/shop' });

    expect(tree.read('go.work', 'utf-8')).toBe('go 1.99.0\n');
  });

  it('should allow the kernel and the module prefix in .golangci.yml and group the local imports', async () => {
    const tree = consumerTree();

    await initGenerator(tree, { modulePrefix: 'github.com/acme/shop' });
    const golangci = tree.read('.golangci.yml', 'utf-8') as string;

    expect(golangci.match(new RegExp(`- ${KERNEL}/\\n`, 'g'))).toHaveLength(4);
    expect(golangci.match(/- github\.com\/acme\/shop\/\n/g)).toHaveLength(4);
    expect(golangci).toContain('- prefix(github.com/acme/shop)');
  });

  it('should write the static infra skeleton and leave the files of infrasync to it', async () => {
    const tree = consumerTree();

    await initGenerator(tree, { modulePrefix: 'github.com/acme/shop', composeProfile: 'shop' });

    expect(tree.read('infra/test/compose.yml', 'utf-8')).toContain('name: shop-testinfra');
    expect(tree.read('infra/local/docker-compose.yml', 'utf-8')).toContain('name: shop-local');
    expect(tree.read('infra/local/compose/app-base.yml', 'utf-8')).toContain('shop-grpc-api:');
    expect(tree.read('infra/local/compose/app-base.yml', 'utf-8')).toContain(
      "GRPC_TRUSTED_CLIENTS: ''",
    );
    expect(tree.exists('tools/dmpf-baseline/units-baseline.json')).toBe(true);
    for (const generated of [
      'infra/local/compose/pki.yml',
      'infra/observability/alloy/alloy-kubernetes.alloy',
    ]) {
      expect(tree.exists(generated)).toBe(false);
    }
  });

  it('should pin the go directive, merge the Go setup into nx.json and add @nx-go/nx-go', async () => {
    const tree = consumerTree();
    const versions = readVersions();

    const callback = await initGenerator(tree, { modulePrefix: 'github.com/acme/shop' });
    const nxJson = readJson(tree, 'nx.json');

    expect(tree.read('go.work', 'utf-8')).toBe(`go ${versions.go.directive}\n`);
    expect(nxJson.namedInputs.go).toContain('{workspaceRoot}/go.work');
    expect(nxJson.plugins).toContain('@nx-go/nx-go');
    expect(Object.keys(nxJson.targetDefaults)).toEqual(
      expect.arrayContaining(['deploy-env', '@nx-go/nx-go:lint', '@nx-go/nx-go:test']),
    );
    expect(readJson(tree, 'package.json').devDependencies['@nx-go/nx-go']).toBe(versions.nxGo);
    expect(typeof callback).toBe('function');
  });

  it('should record the hash of each managed file it wrote and leave the seed files out', async () => {
    const tree = consumerTree();

    await initGenerator(tree, { modulePrefix: 'github.com/acme/shop' });
    const { schema, files } = readJson(tree, 'dmpf.rendered.json');

    expect(schema).toBe('dmpf/rendered@1');
    expect(files['.golangci.yml']).toBe(
      createHash('sha256')
        .update(tree.read('.golangci.yml', 'utf-8') as string)
        .digest('hex'),
    );
    expect(files).not.toHaveProperty(['infra/local/docker-compose.yml']);
  });

  it('should run infrasync by version after the flush, in version mode', async () => {
    const tree = consumerTree();
    const versions = readVersions();

    const callback = await initGenerator(tree, { modulePrefix: 'github.com/acme/shop' });
    await callback?.();

    expect(execFileSync).toHaveBeenCalledWith(
      'go',
      [
        'run',
        `${KERNEL}/tools/dmpf-conformance/cmd/infrasync@${versions.conformance}`,
        '--root',
        '.',
        '--write',
      ],
      expect.objectContaining({ cwd: tree.root }),
    );
  });
});

describe('[generator] init — AI assets', () => {
  it('should write the AI assets with the layout of the workspace and the platform sources of the release', async () => {
    const tree = consumerTree();
    const { conformance } = readVersions();

    await initGenerator(tree, { modulePrefix: 'github.com/acme/shop', appsDir: 'services' });
    const rule = tree.read('.claude/rules/dmpf-bounded-context.md', 'utf-8') as string;
    const goldenPath = tree.read(
      '.agents/skills/dmpf-bounded-context/references/golden-path.md',
      'utf-8',
    ) as string;

    expect(rule.split('---')[1]).toBe('\npaths:\n  - "services/**"\n');
    expect(goldenPath).toContain('services/<name>');
    expect(goldenPath).toContain('https://github.com/mateusmacedo/dmpf/blob/');
    expect(goldenPath).toContain(
      `go run ${KERNEL}/tools/dmpf-conformance/cmd/conformance@${conformance}`,
    );
    expect(goldenPath).toContain(
      'DMPF_APPS_DIR=services bash node_modules/@mateusmacedo/dmpf-plugin/scripts/dmpf-context-check.sh',
    );
    expect(goldenPath).not.toMatch(/`apps\/backend\/bookings/);
    expect(tree.read('.claude/agents/dmpf-context-author.md', 'utf-8')).not.toContain('skill-go');
  });
});

describe('[generator] init — CI caller', () => {
  it('should pin the caller of the reusable workflow to the workflowRef and the plugin version', async () => {
    const tree = consumerTree();
    const { workflowRef } = readVersions();

    await initGenerator(tree, { modulePrefix: 'github.com/acme/shop' });
    const caller = tree.read('.github/workflows/dmpf-ci.yml', 'utf-8') as string;

    expect(caller).toContain(
      `uses: mateusmacedo/dmpf/.github/workflows/dmpf-go-ci.yml@${workflowRef || 'master'}\n`,
    );
    expect(caller).toContain(`plugin-version: "${readPluginVersion()}"`);
  });

  it('should not write the caller in local mode, where ci.yml calls the reusable workflow', async () => {
    const source = consumerTree();
    await initGenerator(source, { modulePrefix: 'github.com/acme/shop' });
    const tree = consumerTree();
    tree.write(
      'dmpf.json',
      JSON.stringify({ ...readJson(source, 'dmpf.json'), tooling: { mode: 'local' } }),
    );

    await initGenerator(tree, {});

    expect(tree.exists('.github/workflows/dmpf-ci.yml')).toBe(false);
  });
});

describe('[generator] init — re-run', () => {
  it('should change nothing when run again with the same options', async () => {
    const tree = consumerTree();
    await initGenerator(tree, { modulePrefix: 'github.com/acme/shop' });
    const paths = [...renderedPaths(), ...ALWAYS_WRITTEN];
    const before = snapshot(tree, paths);

    await initGenerator(tree, { modulePrefix: 'github.com/acme/shop' });

    expect(snapshot(tree, paths)).toEqual(before);
  });

  it('should keep the values of dmpf.json for the options it is not given', async () => {
    const tree = consumerTree();
    await initGenerator(tree, { modulePrefix: 'github.com/acme/shop', edge: 'gateway' });

    await initGenerator(tree, {});

    expect(readJson(tree, 'dmpf.json')).toMatchObject({
      modulePrefix: 'github.com/acme/shop',
      edge: 'gateway',
    });
  });

  it('should move from the reserved prefix to the real one without --force', async () => {
    const tree = consumerTree();
    await initGenerator(tree, {});

    await initGenerator(tree, { modulePrefix: 'github.com/acme/shop' });

    expect(tree.read('.golangci.yml', 'utf-8')).toContain('- prefix(github.com/acme/shop)');
    expect(tree.read('.golangci.yml', 'utf-8')).not.toContain('example.com/change-me');
  });

  it('should refuse to overwrite an edited file without --force, listing it and writing nothing', async () => {
    const tree = consumerTree();
    await initGenerator(tree, { modulePrefix: 'github.com/acme/shop' });
    tree.write('infra/local/compose/redis.yml', 'services: {}\n');
    const before = snapshot(tree, ['dmpf.json', 'infra/local/compose/postgres.yml']);

    await expect(
      initGenerator(tree, { modulePrefix: 'github.com/acme/shop', composeProfile: 'shop' }),
    ).rejects.toThrow('infra/local/compose/redis.yml');
    expect(snapshot(tree, ['dmpf.json', 'infra/local/compose/postgres.yml'])).toEqual(before);

    await initGenerator(tree, {
      modulePrefix: 'github.com/acme/shop',
      composeProfile: 'shop',
      force: true,
    });
    expect(tree.read('infra/local/compose/redis.yml', 'utf-8')).toContain('image: redis:7-alpine');
  });

  it('should never overwrite a seed file, which other generators extend', async () => {
    const tree = consumerTree();
    await initGenerator(tree, { modulePrefix: 'github.com/acme/shop' });
    tree.write('infra/local/docker-compose.yml', 'name: edited\n');

    await initGenerator(tree, { modulePrefix: 'github.com/acme/shop', composeProject: 'other' });

    expect(tree.read('infra/local/docker-compose.yml', 'utf-8')).toBe('name: edited\n');
  });
});

describe('[generator] init — platform', () => {
  const root = join(packageRoot(), '..', '..');

  it('should leave the platform byte for byte as it is: its generic infra is the output of init', async () => {
    const tree = createTreeWithEmptyWorkspace();
    for (const path of [...renderedPaths(), ...ALWAYS_WRITTEN]) {
      if (existsSync(join(root, path))) {
        tree.write(path, readFileSync(join(root, path), 'utf-8'));
      }
    }
    const before = snapshot(tree, [...renderedPaths(), ...ALWAYS_WRITTEN]);

    await initGenerator(tree, {});

    expect(snapshot(tree, [...renderedPaths(), ...ALWAYS_WRITTEN])).toEqual(before);
  });
});
