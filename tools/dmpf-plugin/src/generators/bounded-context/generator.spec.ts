import * as childProcess from 'node:child_process';
import type { Tree } from '@nx/devkit';
import { logger, output } from '@nx/devkit';
import { createTreeWithEmptyWorkspace } from '@nx/devkit/testing';
import { boundedContextGenerator } from './generator';
import type { BoundedContextGeneratorSchema } from './schema';

jest.mock('node:child_process', () => ({
  ...jest.requireActual<typeof import('node:child_process')>('node:child_process'),
  execFileSync: jest.fn(),
}));

const GO_VERSION = '1.26.6';
const DIRECTORY = 'apps/backend';
const MODULE_PREFIX = 'github.com/mateusmacedo/dmpf';
const NX_PROJECT_SCHEMA = '../../../node_modules/nx/schemas/project-schema.json';
const GOFMT_COMMAND = String.raw`saida="$(gofmt -l . 2>&1)"; status=$?; [ $status -eq 0 ] || { printf '%s\n' "$saida" >&2; exit $status; }; [ -z "$saida" ] || { printf '%s\n' "$saida" >&2; exit 1; }`;

const GO_WORK = [
  `go ${GO_VERSION}`,
  '',
  'use (',
  '\t./libs/backend/go/app',
  '\t./libs/backend/go/domain',
  '\t./libs/backend/go/ports',
  ')',
  '',
].join('\n');

type CompanionUnit = {
  unitSuffix: string;
  dirName: string;
  summary: string;
};

type BlockLayout = {
  block: string;
  unitSuffix: string;
  dirName: string;
  layer: string;
  companions?: readonly CompanionUnit[];
};

const LAYOUT: readonly BlockLayout[] = [
  { block: 'domain', unitSuffix: 'domain', dirName: 'domain', layer: 'domain' },
  { block: 'port', unitSuffix: 'ports', dirName: 'ports', layer: 'domain' },
  { block: 'application', unitSuffix: 'application', dirName: 'application', layer: 'services' },
  { block: 'provider', unitSuffix: 'provider-postgres', dirName: 'provider', layer: 'providers' },
  {
    block: 'app',
    unitSuffix: 'app',
    dirName: 'app',
    layer: 'apps',
    companions: [
      { unitSuffix: 'appkit', dirName: 'appkit', summary: '' },
      { unitSuffix: 'distkit', dirName: 'distkit', summary: '' },
    ],
  },
];

const ALL_BLOCKS: readonly string[] = LAYOUT.map((entry) => entry.block);
const BLOCK_DIRS: readonly string[] = LAYOUT.map((entry) => entry.dirName);

const MODULE_FILES: readonly string[] = [
  'README.md',
  'dmpf-units.json',
  'go.mod',
  'package.json',
  'project.json',
];

// The binary lives beside the blocks and only exists when the context has an
// app block: there is nothing to serve without one.
const CMD_DIR = 'cmd';
const APPKIT_DIR = 'appkit';
const DISTKIT_DIR = 'distkit';
const DOCKERFILE = 'Dockerfile';

const BLOCK_FILES: Record<string, readonly string[]> = {
  domain: ['doc.go'],
  ports: ['doc.go'],
  application: ['doc.go'],
  provider: ['doc.go', 'schema.go', 'schema.sql'],
  app: ['catalog.go', 'config.go', 'config_test.go', 'doc.go', 'rpc', 'telemetry.go', 'wiring.go'],
};

const FULL_OPTIONS: BoundedContextGeneratorSchema = {
  name: 'checkout',
  boundedContext: 'sales',
  blocks: [...ALL_BLOCKS],
  directory: DIRECTORY,
};

const MODULE_DIR = `${DIRECTORY}/${FULL_OPTIONS.name}`;

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

const generate = async (
  overrides: Partial<BoundedContextGeneratorSchema> = {},
  prepare?: (tree: Tree) => void,
): Promise<Tree> => {
  const tree = treeWithGoWork();
  prepare?.(tree);
  await boundedContextGenerator(tree, { ...FULL_OPTIONS, ...overrides });
  return tree;
};

const projectOf = (tree: Tree): ProjectConfig =>
  readJsonFile<ProjectConfig>(tree, `${MODULE_DIR}/project.json`);

const manifestOf = (tree: Tree): Manifest =>
  readJsonFile<Manifest>(tree, `${MODULE_DIR}/dmpf-units.json`);

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

const serveTarget = (role: string): Record<string, unknown> => ({
  executor: 'nx:run-commands',
  options: { command: `go run ./cmd --role ${role}`, cwd: '{projectRoot}' },
});

const expectedTargets = ({
  integration,
  app = true,
}: {
  integration: boolean;
  app?: boolean;
}): Record<string, unknown> => {
  const testRace = goTarget({
    command: integration
      ? 'go test -race -count=1 -p 1 -tags=integration ./...'
      : 'go test -race ./...',
    cache: !integration,
  });
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
    ...(app
      ? {
          'serve-api': serveTarget('api'),
          'serve-relay': serveTarget('relay'),
          'test-distributed': {
            executor: 'nx:run-commands',
            cache: false,
            inputs: ['go', '^go'],
            dependsOn: [{ projects: ['postgres', 'app'], target: 'test-race' }, 'test-race'],
            options: {
              command: 'go test -race -count=1 -p 1 -tags=integration,distributed ./distkit/...',
              cwd: '{projectRoot}',
            },
          },
        }
      : {}),
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
  it('should create one Go module holding one directory per requested block', async () => {
    const tree = await generate();

    expect(tree.children(MODULE_DIR).sort()).toEqual(
      [...MODULE_FILES, ...BLOCK_DIRS, CMD_DIR, APPKIT_DIR, DISTKIT_DIR, DOCKERFILE].sort(),
    );
  });

  it('should give every block its doc.go, the provider its schema and the app its canonical files', async () => {
    const tree = await generate();

    for (const { dirName } of LAYOUT) {
      expect(tree.children(`${MODULE_DIR}/${dirName}`).sort()).toEqual(BLOCK_FILES[dirName]);
    }
    expect(tree.children(`${MODULE_DIR}/app/rpc`).sort()).toEqual([
      'errors.go',
      'errors_internal_test.go',
      'service.go',
    ]);
  });

  it('should give the app block a composition root the binary enters by', async () => {
    const tree = await generate();
    const config = readText(tree, `${MODULE_DIR}/app/config.go`);
    const wiring = readText(tree, `${MODULE_DIR}/app/wiring.go`);
    const main = readText(tree, `${MODULE_DIR}/${CMD_DIR}/main.go`);

    expect(wiring).toContain('func Run(ctx context.Context, cfg Config, out io.Writer) error');
    expect(config).toContain('func Defaults(role Role) Config');
    expect(config).toContain('func FromEnv(role Role, lookup func(string) string) (Config, error)');
    expect(config).toContain('func (c Config) Validate() error');
    expect(main).toContain('app.FromEnv(app.Role(o.role), o.lookup)');
    expect(main).toContain('--role api|relay');
  });

  it('should serve gRPC only, with the kernel chain and the migrate of the outbox and the context schema', async () => {
    const tree = await generate();
    const wiring = readText(tree, `${MODULE_DIR}/app/wiring.go`);

    expect(wiring).toContain('kernelgrpc.ServerInterceptors(rpc.ServiceName');
    expect(wiring).toContain('server.RegisterService(&rpc.ServiceDesc, rpc.Server{})');
    expect(wiring).toContain(
      'postgres.Migrate(ctx, pool, []postgres.Capability{postgres.Outbox}, provider.Schema)',
    );
    expect(wiring).not.toContain('net/http');
    expect(tree.exists(`${MODULE_DIR}/app/http`)).toBe(false);
  });

  it('should name the service after the context unless serviceName is given', async () => {
    const derived = await generate();
    const given = await generate({ serviceName: 'company.sales.service.v1.CheckoutService' });

    expect(readText(derived, `${MODULE_DIR}/app/rpc/service.go`)).toContain(
      'const ServiceName = "company.checkout.service.v1.CheckoutService"',
    );
    expect(readText(given, `${MODULE_DIR}/app/rpc/service.go`)).toContain(
      'const ServiceName = "company.sales.service.v1.CheckoutService"',
    );
  });

  it('should read the configuration without the DMPF_ prefix, with the topic named after the context', async () => {
    const tree = await generate({ name: 'order-fulfillment' });
    const config = readText(tree, `${DIRECTORY}/order-fulfillment/app/config.go`);

    expect(config).toMatch(/envDSN\s+= "PG_DSN"/);
    expect(config).toContain('"KAFKA_ORDER_FULFILLMENT_TOPIC"');
    expect(config).toContain('OrderFulfillmentTopic string');
    expect(config).not.toContain('DMPF_');
  });

  it('should embed the context schema in the provider for the composition root to migrate', async () => {
    const tree = await generate();

    expect(readText(tree, `${MODULE_DIR}/provider/schema.go`)).toContain('//go:embed schema.sql');
    expect(readText(tree, `${MODULE_DIR}/provider/schema.sql`)).toContain('snapshot');
  });

  it('should build the image of the binary from the workspace root', async () => {
    const tree = await generate();
    const dockerfile = readText(tree, `${MODULE_DIR}/${DOCKERFILE}`);

    expect(dockerfile).toContain(`./${MODULE_DIR}/cmd`);
    expect(dockerfile).toContain('EXPOSE 9090');
  });

  it('should leave no binary and no serve target when the context has no app block', async () => {
    const tree = await generate({ blocks: ['domain', 'port', 'application'] });

    expect(tree.children(MODULE_DIR)).not.toContain(CMD_DIR);
    expect(Object.keys(projectOf(tree).targets)).not.toContain('serve-api');
    expect(tree.children(MODULE_DIR)).not.toContain(DOCKERFILE);
  });

  it('should identify the project by the bare context name', async () => {
    const tree = await generate();
    const project = projectOf(tree);

    expect(project.name).toBe('checkout');
    expect(project.$schema).toBe(NX_PROJECT_SCHEMA);
    expect(project.projectType).toBe('application');
    expect(project.sourceRoot).toBe(MODULE_DIR);
  });

  it('should tag the project with the three dimensions plus the highest layer of its blocks', async () => {
    const cases: readonly [string[], string][] = [
      [['domain', 'port'], 'domain'],
      [['domain', 'port', 'application'], 'services'],
      [['domain', 'port', 'application', 'provider'], 'providers'],
      [[...ALL_BLOCKS], 'apps'],
    ];

    for (const [blocks, layer] of cases) {
      const tree = await generate({ blocks });

      expect(projectOf(tree).tags).toEqual([
        'type:app',
        'scope:backend',
        'stack:go',
        `layer:${layer}`,
      ]);
    }
  });

  it('should declare the five Go targets, the two serve targets, the distributed one and no lint target', async () => {
    const tree = await generate();

    expect(Object.keys(projectOf(tree).targets).sort()).toEqual([
      'build',
      'fmt-check',
      'govulncheck',
      'serve-api',
      'serve-relay',
      'test-distributed',
      'test-race',
      'vet',
    ]);
  });

  it('should serve each role by the single binary of the context', async () => {
    const tree = await generate();
    const targets = projectOf(tree).targets as Record<string, { options: { command: string } }>;

    expect(targets['serve-api'].options.command).toBe('go run ./cmd --role api');
    expect(targets['serve-relay'].options.command).toBe('go run ./cmd --role relay');
  });

  it('should run test-race with the integration tag and no serialization, the database being its own', async () => {
    const tree = await generate();

    expect(projectOf(tree).targets).toEqual(expectedTargets({ integration: true }));
  });

  it('should run a cached, tag-free test-race when no block touches infrastructure', async () => {
    const tree = await generate({ blocks: ['domain', 'port', 'application'] });

    expect(projectOf(tree).targets).toEqual(expectedTargets({ integration: false, app: false }));
  });

  it('should declare one unit per block and one per companion, in block order', async () => {
    const tree = await generate();
    const manifest = manifestOf(tree);
    const expected = LAYOUT.flatMap((entry) => [
      { block: entry.block, unitSuffix: entry.unitSuffix, dirName: entry.dirName },
      ...(entry.companions ?? []).map((companion) => ({
        block: entry.block,
        unitSuffix: companion.unitSuffix,
        dirName: companion.dirName,
      })),
    ]);

    expect(manifest.schema).toBe('dmpf/units@1');
    expect(manifest.exceptions).toEqual([]);
    expect(manifest.units).toHaveLength(expected.length);
    manifest.units.forEach((unit, index) => {
      const { block, unitSuffix, dirName } = expected[index];
      expect(unit.id).toBe(`sales/${unitSuffix}`);
      expect(unit.block).toBe(block);
      expect(unit.bounded_context).toBe('sales');
      expect(unit.public_integration_surface).toBe(false);
      // The binary belongs to the unit of the app block: it is a composition
      // root, not a unit of its own.
      const include =
        dirName === 'app'
          ? [
              `${MODULE_PREFIX}/${MODULE_DIR}/app`,
              `${MODULE_PREFIX}/${MODULE_DIR}/app/rpc`,
              `${MODULE_PREFIX}/${MODULE_DIR}/cmd`,
            ]
          : [`${MODULE_PREFIX}/${MODULE_DIR}/${dirName}`];
      expect(unit.include).toEqual(include);
    });
  });

  it('should give the app block both test kits, each in a directory of its own', async () => {
    const tree = await generate();

    expect(tree.children(`${MODULE_DIR}/${APPKIT_DIR}`).sort()).toEqual(['doc.go', 'pool.go']);
    expect(readText(tree, `${MODULE_DIR}/${APPKIT_DIR}/doc.go`)).toContain('package appkit');
    expect(readText(tree, `${MODULE_DIR}/${APPKIT_DIR}/pool.go`)).toContain(
      'Project:      "checkout"',
    );
    expect(readText(tree, `${MODULE_DIR}/${APPKIT_DIR}/pool.go`)).toContain(
      'var Tables = []string{}',
    );
    expect(tree.children(`${MODULE_DIR}/${DISTKIT_DIR}`)).toEqual(['doc.go']);
    expect(readText(tree, `${MODULE_DIR}/${DISTKIT_DIR}/doc.go`)).toContain('package distkit');
  });

  it('should run the distributed vector by its own target, after the shared infrastructure', async () => {
    const tree = await generate();
    const target = (projectOf(tree).targets as Record<string, Record<string, unknown>>)[
      'test-distributed'
    ];

    expect(target.cache).toBe(false);
    expect(target.dependsOn).toEqual([
      { projects: ['postgres', 'app'], target: 'test-race' },
      'test-race',
    ]);
    expect((target.options as { command: string }).command).toBe(
      'go test -race -count=1 -p 1 -tags=integration,distributed ./distkit/...',
    );
  });

  it('should leave no test kit when the context has no app block', async () => {
    const tree = await generate({ blocks: ['domain', 'port', 'application'] });

    expect(tree.children(MODULE_DIR)).not.toContain(APPKIT_DIR);
    expect(tree.children(MODULE_DIR)).not.toContain(DISTKIT_DIR);
    const ids = manifestOf(tree).units.map((unit) => unit.id);
    expect(ids).not.toContain('sales/appkit');
    expect(ids).not.toContain('sales/distkit');
  });

  it('should declare the pgx external dependency only when provider is generated', async () => {
    const withProvider = manifestOf(await generate());
    const withoutProvider = manifestOf(
      await generate({ blocks: ['domain', 'port', 'application'] }),
    );

    expect(withProvider.external).toContainEqual(
      expect.objectContaining({ package: 'github.com/jackc/pgx/v5', capability: 'io.storage' }),
    );
    expect(withoutProvider.external).toEqual([]);
  });

  it('should write a go.mod with only module and go: a fresh context imports no siblings', async () => {
    const tree = await generate();
    const goMod = readText(tree, `${MODULE_DIR}/go.mod`);

    expect(goMod).toContain(`module ${MODULE_PREFIX}/${MODULE_DIR}`);
    expect(goMod).toContain(`go ${GO_VERSION}`);
    expect(goMod).not.toContain('require');
    expect(goMod).not.toContain('replace');
  });

  it('should read the Go version of the generated go.mod from go.work', async () => {
    const tree = treeWithGoWork(GO_WORK.replace(`go ${GO_VERSION}`, 'go 1.27.0'));

    await boundedContextGenerator(tree, FULL_OPTIONS);

    expect(readText(tree, `${MODULE_DIR}/go.mod`)).toContain('go 1.27.0');
  });

  it('should register the single module in go.work in sorted order', async () => {
    const tree = await generate();

    expect(useEntries(readText(tree, 'go.work'))).toEqual([
      './apps/backend/checkout',
      './libs/backend/go/app',
      './libs/backend/go/domain',
      './libs/backend/go/ports',
    ]);
    expect(readText(tree, 'go.work').startsWith(`go ${GO_VERSION}`)).toBe(true);
  });

  it('should write a private, unpublished package.json named after the context', async () => {
    const tree = await generate();

    expect(readJsonFile<PackageManifest>(tree, `${MODULE_DIR}/package.json`)).toEqual({
      name: '@mateusmacedo/checkout',
      version: '0.0.0',
      private: true,
    });
  });

  it('should create only the requested block directories when a subset is asked for', async () => {
    const withoutApp = await generate({
      blocks: ['domain', 'port', 'application', 'provider'],
    });

    expect(withoutApp.children(MODULE_DIR).sort()).toEqual(
      [...MODULE_FILES, 'application', 'domain', 'ports', 'provider'].sort(),
    );
  });

  it('should fall back to the schema defaults when blocks and directory are omitted', async () => {
    const tree = treeWithGoWork();

    await boundedContextGenerator(tree, {
      name: FULL_OPTIONS.name,
      boundedContext: FULL_OPTIONS.boundedContext,
    });

    expect(tree.children(MODULE_DIR).sort()).toEqual(
      [...MODULE_FILES, ...BLOCK_DIRS, CMD_DIR, APPKIT_DIR, DISTKIT_DIR, DOCKERFILE].sort(),
    );
  });
});

describe('[generator] bounded-context — identifiers', () => {
  const name = 'order-fulfillment';

  it('should slug the module directory from the name', async () => {
    const tree = await generate({ name });

    expect(tree.children(DIRECTORY)).toEqual(['order-fulfillment']);
    expect(tree.children(`${DIRECTORY}/order-fulfillment`).sort()).toEqual(
      [...MODULE_FILES, ...BLOCK_DIRS, CMD_DIR, APPKIT_DIR, DISTKIT_DIR, DOCKERFILE].sort(),
    );
  });

  it('should name every Go package after its block directory, with no prefix', async () => {
    const tree = await generate({ name });

    for (const { dirName } of LAYOUT) {
      const doc = readText(tree, `${DIRECTORY}/${name}/${dirName}/doc.go`);

      expect(doc).toContain(`package ${dirName}\n`);
      expect(doc).not.toContain('orderfulfillment');
    }
  });

  it('should keep the declared bounded context independent from the name', async () => {
    const tree = await generate();
    const manifest = manifestOf(tree);

    expect(new Set(manifest.units.map((unit) => unit.bounded_context))).toEqual(new Set(['sales']));
    expect(readText(tree, `${MODULE_DIR}/dmpf-units.json`)).not.toContain(
      '"bounded_context": "checkout"',
    );
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

  it('should refuse a serviceName that is not a qualified proto service name', async () => {
    await expectRefusal({
      overrides: { serviceName: 'company.sales.checkoutService' },
      message: /option serviceName .* must match/,
    });
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
      message: /checkout/,
      prepare: (tree) => {
        tree.write(`${MODULE_DIR}/go.mod`, `module ${MODULE_PREFIX}/${MODULE_DIR}\n`);
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
            '\t./libs/backend/go/app\n',
            '\t./apps/backend/checkout\n\t./libs/backend/go/app\n',
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
    expect(manifestOf(tree).units[0].bounded_context).toBe(unsafe);
  });
});

describe('[generator] bounded-context — release groups and module sync', () => {
  const MODSYNC_ARGS = ['run', './tools/dmpf-conformance/cmd/modsync', '--root', '.', '--write'];

  const NX_JSON = JSON.stringify(
    {
      release: {
        groups: {
          'go-libs': {
            projects: ['directory:libs/backend/go/*'],
            releaseTag: { pattern: 'libs/backend/go/{projectName}/v{version}' },
          },
          'go-tools': {
            projects: ['conformance'],
            releaseTag: { pattern: 'tools/dmpf-conformance/v{version}' },
          },
          npm: {
            projects: ['tag:type:lib', '!tag:stack:go'],
            releaseTag: { pattern: '{projectName}@{version}' },
          },
        },
      },
    },
    null,
    2,
  );

  const modsync = childProcess.execFileSync as jest.MockedFunction<
    typeof childProcess.execFileSync
  >;

  beforeEach(() => {
    modsync.mockClear();
  });

  it('should never write to nx.json, so no release group is edited during generation', async () => {
    const tree = treeWithGoWork();
    tree.write('nx.json', NX_JSON);

    await boundedContextGenerator(tree, FULL_OPTIONS);

    expect(projectOf(tree).tags).toContain('type:app');
    expect(readText(tree, 'nx.json')).toBe(NX_JSON);
  });

  it('should defer the modsync run to the post-flush callback', async () => {
    const tree = treeWithGoWork();

    const callback = await boundedContextGenerator(tree, FULL_OPTIONS);

    expect(typeof callback).toBe('function');
    expect(modsync).not.toHaveBeenCalled();
  });

  it('should write the sibling requires by running dmpf-modsync from the workspace root', async () => {
    const tree = treeWithGoWork();

    const callback = await boundedContextGenerator(tree, FULL_OPTIONS);
    await callback();

    expect(modsync).toHaveBeenCalledTimes(1);
    expect(modsync).toHaveBeenCalledWith('go', MODSYNC_ARGS, {
      cwd: tree.root,
      stdio: 'inherit',
    });
  });

  it('should point to the manual recovery when the modsync run fails', async () => {
    const cause = new Error('spawnSync go ENOENT');
    modsync.mockImplementation(() => {
      throw cause;
    });
    const tree = treeWithGoWork();

    const callback = await boundedContextGenerator(tree, FULL_OPTIONS);

    expect(callback).toThrow(MODULE_DIR);
    expect(callback).toThrow(`go ${MODSYNC_ARGS.join(' ')}`);
    try {
      callback();
    } catch (error) {
      expect((error as Error).cause).toBe(cause);
    }
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

    expect(printed()).toContain('1 módulo gerado');
    expect(printed()).toContain('--write-baseline');
    expect(printed()).toContain('AUT-01');
  });
});
