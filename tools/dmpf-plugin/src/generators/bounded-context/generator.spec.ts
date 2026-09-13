import type { Tree } from '@nx/devkit';
import { logger, output } from '@nx/devkit';
import { createTreeWithEmptyWorkspace } from '@nx/devkit/testing';
import { boundedContextGenerator } from './generator';
import type { BoundedContextGeneratorSchema } from './schema';

const GO_VERSION = '1.26.6';
const DIRECTORY = 'libs/backend/go';
const MODULE_PREFIX = 'github.com/mateusmacedo/dmpf';
const NX_PROJECT_SCHEMA = '../../../../../node_modules/nx/schemas/project-schema.json';
const GOFMT_COMMAND = String.raw`saida="$(gofmt -l . 2>&1)"; status=$?; [ $status -eq 0 ] || { printf '%s\n' "$saida" >&2; exit $status; }; [ -z "$saida" ] || { printf '%s\n' "$saida" >&2; exit 1; }`;

const GO_WORK = [
  `go ${GO_VERSION}`,
  '',
  'use (',
  '\t./libs/backend/go/dmpf-app',
  '\t./libs/backend/go/dmpf-domain',
  '\t./libs/backend/go/dmpf-ports',
  ')',
  '',
].join('\n');

type BlockLayout = {
  block: string;
  suffix: string;
  dirName: string;
  layer: string;
  integration: boolean;
};

const LAYOUT: readonly BlockLayout[] = [
  { block: 'domain', suffix: 'domain', dirName: 'domain', layer: 'domain', integration: false },
  { block: 'port', suffix: 'ports', dirName: 'ports', layer: 'domain', integration: false },
  {
    block: 'application',
    suffix: 'application',
    dirName: 'application',
    layer: 'services',
    integration: false,
  },
  {
    block: 'provider',
    suffix: 'provider-postgres',
    dirName: 'provider',
    layer: 'providers',
    integration: true,
  },
  { block: 'app', suffix: 'app', dirName: 'app', layer: 'apps', integration: true },
];

const ALL_BLOCKS: readonly string[] = LAYOUT.map((entry) => entry.block);

const MODULE_FILES: readonly string[] = [
  'README.md',
  'dmpf-units.json',
  'doc.go',
  'go.mod',
  'package.json',
  'project.json',
];

const FULL_OPTIONS: BoundedContextGeneratorSchema = {
  name: 'checkout',
  boundedContext: 'sales',
  blocks: [...ALL_BLOCKS],
  directory: DIRECTORY,
};

type ProjectConfig = {
  name: string;
  $schema: string;
  projectType: string;
  sourceRoot: string;
  tags: string[];
  targets: Record<string, unknown>;
};

type Unit = {
  id: string;
  block: string;
  bounded_context: string;
  public_integration_surface: boolean;
  include: string[];
};

type ExternalDependency = {
  package: string;
  capability: string;
};

type Manifest = {
  schema: string;
  units: Unit[];
  external: ExternalDependency[];
  exceptions: unknown[];
};

type PackageManifest = {
  name: string;
  version: string;
  private: boolean;
};

type Changes = Record<string, string>;

const changesOf = (tree: Tree): Changes =>
  Object.fromEntries(
    tree.listChanges().map((change) => [change.path, change.content?.toString('utf-8') ?? '']),
  );

const treeWithGoWork = (goWork: string = GO_WORK): Tree => {
  const tree = createTreeWithEmptyWorkspace();
  tree.write('go.work', goWork);
  return tree;
};

const readText = (tree: Tree, path: string): string => {
  const content = tree.read(path, 'utf-8');
  if (content === null) {
    throw new Error(`expected ${path} to exist in the tree`);
  }
  return content;
};

const readJsonFile = <T>(tree: Tree, path: string): T => JSON.parse(readText(tree, path)) as T;

const dirNameOf = (suffix: string): string =>
  LAYOUT.find((layout) => layout.suffix === suffix)?.dirName ??
  (() => {
    throw new Error(`unknown suffix ${suffix}`);
  })();

const moduleDir = ({
  suffix,
  name = FULL_OPTIONS.name,
  directory = DIRECTORY,
}: {
  suffix: string;
  name?: string;
  directory?: string;
}): string => `${directory}/${name}/${dirNameOf(suffix)}`;

const dirOf = (suffix: string): string => moduleDir({ suffix });

const generate = async (
  overrides: Partial<BoundedContextGeneratorSchema> = {},
  prepare?: (tree: Tree) => void,
): Promise<Tree> => {
  const tree = treeWithGoWork();
  prepare?.(tree);
  await boundedContextGenerator(tree, { ...FULL_OPTIONS, ...overrides });
  return tree;
};

const goTarget = ({
  command,
  cache,
  withInputs = true,
}: {
  command: string;
  cache?: boolean;
  withInputs?: boolean;
}): Record<string, unknown> => {
  const target: Record<string, unknown> = { executor: 'nx:run-commands' };
  if (cache !== undefined) {
    target.cache = cache;
  }
  if (withInputs) {
    target.inputs = ['go', '^go'];
  }
  target.options = { command, cwd: '{projectRoot}' };
  return target;
};

const expectedTargets = ({
  integration,
  dependsOnProject,
}: {
  integration: boolean;
  dependsOnProject?: string;
}): Record<string, unknown> => {
  const testRace = goTarget({
    command: integration
      ? 'go test -race -count=1 -p 1 -tags=integration ./...'
      : 'go test -race ./...',
    cache: !integration,
  });
  if (dependsOnProject !== undefined) {
    testRace.dependsOn = [{ projects: [dependsOnProject], target: 'test-race' }];
  }
  return {
    'fmt-check': goTarget({ command: GOFMT_COMMAND, cache: true }),
    vet: goTarget({ command: 'go vet ./...', cache: true }),
    build: goTarget({ command: 'go build ./...' }),
    'test-race': testRace,
    govulncheck: goTarget({
      command: 'go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...',
      cache: false,
      withInputs: false,
    }),
  };
};

const useEntries = (goWork: string): string[] => {
  const block = goWork.match(/use \(\n([\s\S]*?)\n\)/);
  if (block === null) {
    throw new Error('expected a "use ( ... )" block in go.work');
  }
  return block[1].split('\n').map((line) => line.trim());
};

const captureOutput = (): (() => string) => {
  const parts: string[] = [];
  const record = (...args: unknown[]): void => {
    parts.push(args.map((arg) => (typeof arg === 'string' ? arg : JSON.stringify(arg))).join(' '));
  };
  jest.spyOn(logger, 'info').mockImplementation(record);
  jest.spyOn(logger, 'log').mockImplementation(record);
  jest.spyOn(output, 'log').mockImplementation(record);
  return () => parts.join('\n');
};

const expectRefusal = async ({
  overrides,
  message,
  prepare,
}: {
  overrides: Partial<BoundedContextGeneratorSchema>;
  message: RegExp;
  prepare?: (tree: Tree) => void;
}): Promise<void> => {
  const tree = treeWithGoWork();
  prepare?.(tree);
  const before = changesOf(tree);
  await expect(boundedContextGenerator(tree, { ...FULL_OPTIONS, ...overrides })).rejects.toThrow(
    message,
  );
  expect(changesOf(tree)).toEqual(before);
};

afterEach(() => {
  jest.restoreAllMocks();
});

describe('[generator] bounded-context — generation', () => {
  it('should create one Go module directory per requested block', async () => {
    const tree = await generate();

    expect(tree.children(`${DIRECTORY}/checkout`).sort()).toEqual([
      'app',
      'application',
      'domain',
      'ports',
      'provider',
    ]);
  });

  it('should fill every module with the six skeleton files and no other source', async () => {
    const tree = await generate();

    for (const { suffix } of LAYOUT) {
      expect(tree.children(dirOf(suffix)).sort()).toEqual(MODULE_FILES);
    }
  });

  it('should identify every project by its module directory', async () => {
    const tree = await generate();

    for (const { suffix } of LAYOUT) {
      const project = readJsonFile<ProjectConfig>(tree, `${dirOf(suffix)}/project.json`);

      expect(project.name).toBe(`checkout-${suffix}-go`);
      expect(project.$schema).toBe(NX_PROJECT_SCHEMA);
      expect(project.projectType).toBe('library');
      expect(project.sourceRoot).toBe(dirOf(suffix));
    }
  });

  it('should tag every project with the three taxonomy dimensions plus its layer', async () => {
    const tree = await generate();

    for (const { suffix, layer } of LAYOUT) {
      const project = readJsonFile<ProjectConfig>(tree, `${dirOf(suffix)}/project.json`);

      expect(project.tags).toEqual(['type:lib', 'scope:backend', 'stack:go', `layer:${layer}`]);
    }
  });

  it('should declare the five Go targets and no lint target', async () => {
    const tree = await generate();

    for (const { suffix } of LAYOUT) {
      const project = readJsonFile<ProjectConfig>(tree, `${dirOf(suffix)}/project.json`);

      expect(Object.keys(project.targets).sort()).toEqual([
        'build',
        'fmt-check',
        'govulncheck',
        'test-race',
        'vet',
      ]);
    }
  });

  it('should shape every target like dmpf-domain, with integration test-race on provider and app', async () => {
    const tree = await generate();

    for (const { suffix, integration } of LAYOUT) {
      const project = readJsonFile<ProjectConfig>(tree, `${dirOf(suffix)}/project.json`);

      expect(project.targets).toEqual(
        expectedTargets({
          integration,
          dependsOnProject: suffix === 'app' ? 'checkout-provider-postgres-go' : undefined,
        }),
      );
    }
  });

  it('should declare one unit per module carrying the block of that module', async () => {
    const tree = await generate();

    for (const { suffix, block } of LAYOUT) {
      const manifest = readJsonFile<Manifest>(tree, `${dirOf(suffix)}/dmpf-units.json`);

      expect(manifest.schema).toBe('dmpf/units@1');
      expect(manifest.exceptions).toEqual([]);
      expect(manifest.units).toHaveLength(1);
      expect(manifest.units[0].block).toBe(block);
      expect(manifest.units[0].public_integration_surface).toBe(false);
      expect(manifest.units[0].id.startsWith('sales/')).toBe(true);
      expect(manifest.units[0].include.length).toBeGreaterThan(0);
      for (const included of manifest.units[0].include) {
        expect(included.startsWith(`${MODULE_PREFIX}/${dirOf(suffix)}`)).toBe(true);
      }
    }
  });

  it('should declare the pgx external dependency on the provider manifest only', async () => {
    const tree = await generate();

    for (const { suffix } of LAYOUT) {
      const manifest = readJsonFile<Manifest>(tree, `${dirOf(suffix)}/dmpf-units.json`);

      if (suffix === 'provider-postgres') {
        expect(manifest.external).toContainEqual(
          expect.objectContaining({
            package: 'github.com/jackc/pgx/v5',
            capability: 'io.storage',
          }),
        );
      } else {
        expect(manifest.external).toEqual([]);
      }
    }
  });

  it('should write a workspace-only go.mod for every module', async () => {
    const tree = await generate();

    for (const { suffix } of LAYOUT) {
      const goMod = readText(tree, `${dirOf(suffix)}/go.mod`);

      expect(goMod).toContain(`module ${MODULE_PREFIX}/${dirOf(suffix)}`);
      expect(goMod).toContain(`go ${GO_VERSION}`);
      expect(goMod).not.toContain('require');
      expect(goMod).not.toContain('replace');
    }
  });

  it('should read the Go version of the generated go.mod from go.work', async () => {
    const tree = treeWithGoWork(GO_WORK.replace(`go ${GO_VERSION}`, 'go 1.27.0'));

    await boundedContextGenerator(tree, FULL_OPTIONS);

    expect(readText(tree, `${dirOf('domain')}/go.mod`)).toContain('go 1.27.0');
  });

  it('should register the new modules in go.work in sorted order', async () => {
    const tree = await generate();

    expect(useEntries(readText(tree, 'go.work'))).toEqual([
      './libs/backend/go/checkout/app',
      './libs/backend/go/checkout/application',
      './libs/backend/go/checkout/domain',
      './libs/backend/go/checkout/ports',
      './libs/backend/go/checkout/provider',
      './libs/backend/go/dmpf-app',
      './libs/backend/go/dmpf-domain',
      './libs/backend/go/dmpf-ports',
    ]);
    expect(readText(tree, 'go.work').startsWith(`go ${GO_VERSION}`)).toBe(true);
  });

  it('should write a private, unpublished package.json for every module', async () => {
    const tree = await generate();

    for (const { suffix } of LAYOUT) {
      const manifest = readJsonFile<PackageManifest>(tree, `${dirOf(suffix)}/package.json`);

      expect(manifest).toEqual({
        name: `@mateusmacedo/checkout-${suffix}-go`,
        version: '0.0.0',
        private: true,
      });
    }
  });

  it('should create only the requested blocks when a subset is asked for', async () => {
    const withoutApp = await generate({
      blocks: ['domain', 'port', 'application', 'provider'],
    });

    expect(withoutApp.children(`${DIRECTORY}/checkout`).sort()).toEqual([
      'application',
      'domain',
      'ports',
      'provider',
    ]);
  });

  it('should fall back to the schema defaults when blocks and directory are omitted', async () => {
    const tree = treeWithGoWork();

    await boundedContextGenerator(tree, {
      name: FULL_OPTIONS.name,
      boundedContext: FULL_OPTIONS.boundedContext,
    });

    expect(tree.children(`${DIRECTORY}/checkout`).sort()).toEqual([
      'app',
      'application',
      'domain',
      'ports',
      'provider',
    ]);
  });
});

describe('[generator] bounded-context — identifiers', () => {
  const name = 'order-fulfillment';

  it('should slug the module directories from the name', async () => {
    const tree = await generate({ name });

    expect(tree.children(DIRECTORY)).toEqual(['order-fulfillment']);
    expect(tree.children(`${DIRECTORY}/order-fulfillment`).sort()).toEqual([
      'app',
      'application',
      'domain',
      'ports',
      'provider',
    ]);
  });

  it('should derive the Go package identifier by dropping the hyphens', async () => {
    const tree = await generate({ name });

    expect(readText(tree, `${moduleDir({ suffix: 'ports', name })}/doc.go`)).toContain(
      'package orderfulfillmentports',
    );
  });

  it('should keep the declared bounded context independent from the name', async () => {
    const tree = await generate();

    for (const { suffix } of LAYOUT) {
      const manifest = readJsonFile<Manifest>(tree, `${dirOf(suffix)}/dmpf-units.json`);

      expect(manifest.units.map((unit) => unit.bounded_context)).toEqual(['sales']);
      expect(readText(tree, `${dirOf(suffix)}/dmpf-units.json`)).not.toContain(
        '"bounded_context": "checkout"',
      );
    }
  });
});

describe('[generator] bounded-context — refusals', () => {
  type OptionsWithoutContext = Omit<BoundedContextGeneratorSchema, 'boundedContext'>;

  const withoutBoundedContext = (
    options: OptionsWithoutContext,
  ): Partial<BoundedContextGeneratorSchema> =>
    options as unknown as Partial<BoundedContextGeneratorSchema>;

  it('should refuse a missing boundedContext citing ADR-012', async () => {
    const tree = treeWithGoWork();
    const before = changesOf(tree);
    const options = withoutBoundedContext({
      name: FULL_OPTIONS.name,
      blocks: [...ALL_BLOCKS],
      directory: DIRECTORY,
    });

    await expect(
      boundedContextGenerator(tree, options as BoundedContextGeneratorSchema),
    ).rejects.toThrow(/ADR-012/);
    expect(changesOf(tree)).toEqual(before);
  });

  it('should refuse an empty boundedContext citing ADR-012', async () => {
    await expectRefusal({ overrides: { boundedContext: '' }, message: /ADR-012/ });
  });

  it('should refuse the contract block pointing at contracts/', async () => {
    await expectRefusal({
      overrides: { blocks: [...ALL_BLOCKS, 'contract'] },
      message: /contracts\//,
    });
  });

  it('should refuse a block subset that leaves dependencies open, naming them', async () => {
    const tree = treeWithGoWork();
    const before = changesOf(tree);

    const run = boundedContextGenerator(tree, {
      ...FULL_OPTIONS,
      blocks: ['domain', 'app'],
    });

    await expect(run).rejects.toThrow(/port/);
    expect(changesOf(tree)).toEqual(before);

    const messages = await boundedContextGenerator(treeWithGoWork(), {
      ...FULL_OPTIONS,
      blocks: ['domain', 'app'],
    }).catch((error: unknown) => (error as Error).message);

    expect(messages).toMatch(/application/);
    expect(messages).toMatch(/provider/);
  });

  it('should refuse a directory that escapes the workspace root', async () => {
    await expectRefusal({ overrides: { directory: '../outside' }, message: /directory/i });
  });

  it('should refuse an absolute directory', async () => {
    await expectRefusal({ overrides: { directory: '/tmp/outside' }, message: /directory/i });
  });

  it('should refuse an existing module directory', async () => {
    await expectRefusal({
      overrides: {},
      message: /checkout\/domain/,
      prepare: (tree) => {
        tree.write(`${dirOf('domain')}/go.mod`, `module ${MODULE_PREFIX}/${dirOf('domain')}\n`);
      },
    });
  });

  it('should refuse a module already registered in go.work', async () => {
    await expectRefusal({
      overrides: {},
      message: /go\.work/,
      prepare: (tree) => {
        tree.write(
          'go.work',
          GO_WORK.replace(
            '\t./libs/backend/go/dmpf-app\n',
            '\t./libs/backend/go/checkout/domain\n\t./libs/backend/go/dmpf-app\n',
          ),
        );
      },
    });
  });

  it('should either refuse an unsafe boundedContext or keep every manifest parseable', async () => {
    const tree = treeWithGoWork();
    const before = changesOf(tree);
    const unsafe = 'sa"les\n/../x';
    const failure = await boundedContextGenerator(tree, {
      ...FULL_OPTIONS,
      boundedContext: unsafe,
    }).then(
      () => null,
      (error: unknown) => error as Error,
    );

    if (failure !== null) {
      expect(failure.message).toMatch(/boundedContext/);
      expect(changesOf(tree)).toEqual(before);
      return;
    }

    for (const [path, content] of Object.entries(changesOf(tree))) {
      if (!path.endsWith('.json')) {
        continue;
      }
      expect(() => JSON.parse(content)).not.toThrow();
    }
    expect(
      readJsonFile<Manifest>(tree, `${dirOf('domain')}/dmpf-units.json`).units[0].bounded_context,
    ).toBe(unsafe);
  });
});

describe('[generator] bounded-context — determinism and output', () => {
  it('should produce byte-identical output across two runs', async () => {
    const first = await generate();
    const second = await generate();

    expect(changesOf(second)).toEqual(changesOf(first));
  });

  it('should print the baseline instruction naming --write-baseline and AUT-01', async () => {
    const printed = captureOutput();

    await generate();

    expect(printed()).toContain('--write-baseline');
    expect(printed()).toContain('AUT-01');
  });
});
