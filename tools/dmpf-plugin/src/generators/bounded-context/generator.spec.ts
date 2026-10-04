import * as childProcess from 'node:child_process';
import * as fs from 'node:fs';
import * as path from 'node:path';
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
const CONTRACT_DIR = 'contract';
const DEPLOY_DIR = 'deploy';
const TEST_ENV = 'bash ../../../tools/test-env.sh';
const GO_TIDY = 'bash ../../../tools/go-tidy.sh';
const TEST_INFRA = { projects: ['testkit'], target: 'test-infra-up' };
const APPKIT_DIR = 'appkit';
const DISTKIT_DIR = 'distkit';
const DOCKERFILE = 'Dockerfile';

const BLOCK_FILES: Record<string, readonly string[]> = {
  domain: ['doc.go'],
  ports: ['doc.go'],
  application: ['commands.go', 'doc.go'],
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
const CONTRACT_MODULE_DIR = `${MODULE_DIR}/${CONTRACT_DIR}`;
const DEPLOY_MODULE_DIR = `${MODULE_DIR}/${DEPLOY_DIR}`;

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

const resourceOf = (role: string): string =>
  `\${OTEL_RESOURCE_ATTRIBUTES:+$OTEL_RESOURCE_ATTRIBUTES,}service.instance.id=checkout-local-${role},dmpf.process.role=${role}`;
const serveOf = (role: string): string =>
  `set -a; [ ! -f deploy/.env ] || . deploy/.env; OTEL_RESOURCE_ATTRIBUTES="${resourceOf(role)}"; set +a; exec go run ./cmd --role ${role}`;
const SERVE_API = serveOf('api');
const SERVE_RELAY = serveOf('relay');
const DOCKER_RUN_RELAY = `[ ! -f deploy/.env ] || . deploy/.env; exec docker run --rm --name checkout-relay --network host --env-file deploy/.env -e OTEL_RESOURCE_ATTRIBUTES="${resourceOf('relay')}" apps-backend-checkout --role relay`;

const serveTarget = (command: string, dependsOn: unknown[]): Record<string, unknown> => ({
  executor: 'nx:run-commands',
  options: { command, cwd: '{projectRoot}', envFile: '{projectRoot}/deploy/.env.example' },
  continuous: true,
  dependsOn,
});

const expectedTargets = ({
  integration,
  app = true,
}: {
  integration: boolean;
  app?: boolean;
}): Record<string, unknown> => {
  const testRace = {
    ...goTarget({
      command: integration
        ? `${TEST_ENV} go test -race -count=1 -p 1 -tags=integration ./...`
        : 'go test -race ./...',
      cache: !integration,
    }),
    ...(integration ? { dependsOn: [TEST_INFRA] } : {}),
  };
  return {
    tidy: {
      executor: 'nx:run-commands',
      cache: false,
      options: { command: GO_TIDY, cwd: '{projectRoot}' },
    },
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
          'nx-release-publish': { executor: 'nx:noop' },
          'serve-api': serveTarget(SERVE_API, [
            { projects: ['bff'], target: 'infra-up' },
            { projects: ['bff'], target: 'infra-session' },
          ]),
          'serve-relay': serveTarget(SERVE_RELAY, ['serve-api']),
          'deploy-env': {},
          'docker:run-relay': {
            executor: 'nx:run-commands',
            options: { command: DOCKER_RUN_RELAY, cwd: '{projectRoot}' },
            continuous: true,
            dependsOn: ['docker:run'],
          },
          'test-distributed': {
            executor: 'nx:run-commands',
            cache: false,
            inputs: ['go', '^go'],
            dependsOn: [
              TEST_INFRA,
              { projects: ['postgres', 'app'], target: 'test-race' },
              'test-race',
            ],
            options: {
              command: `${TEST_ENV} go test -race -count=1 -p 1 -tags=integration,distributed ./distkit/...`,
              cwd: '{projectRoot}',
            },
          },
          e2e: {
            executor: 'nx:run-commands',
            cache: false,
            inputs: ['go', '^go'],
            dependsOn: [TEST_INFRA],
            options: {
              command: `${TEST_ENV} go test -race -count=1 -p 1 -tags=integration,distributed ./distkit/...`,
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
  it('should create one Go module holding one directory per requested block, the contract and the deploy', async () => {
    const tree = await generate();

    expect(tree.children(MODULE_DIR).sort()).toEqual(
      [
        ...MODULE_FILES,
        ...BLOCK_DIRS,
        CMD_DIR,
        APPKIT_DIR,
        DISTKIT_DIR,
        DOCKERFILE,
        CONTRACT_DIR,
        DEPLOY_DIR,
      ].sort(),
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

    expect(wiring).toContain('func Run(ctx context.Context, cfg Config) error');
    expect(config).toContain('func Defaults(role Role) Config');
    expect(config).toContain('func FromEnv(role Role, lookup func(string) string) (Config, error)');
    expect(config).toContain('func (c Config) Validate() error');
    expect(main).toContain('app.FromEnv(app.Role(o.role), o.lookup)');
    expect(main).toContain('--role api|relay');
  });

  it('should declare the role of the process to the telemetry, with no log fields of its own', async () => {
    const tree = await generate();
    const telemetry = readText(tree, `${MODULE_DIR}/app/telemetry.go`);

    expect(telemetry).toContain('Role:     string(cfg.Role),');
    expect(telemetry).not.toContain('Fields');
    expect(telemetry).not.toContain('requestFields');
  });

  it('should leave the identity and the export of the telemetry to the OTEL_* environment', async () => {
    const tree = await generate();
    const config = readText(tree, `${MODULE_DIR}/app/config.go`);
    const telemetry = readText(tree, `${MODULE_DIR}/app/telemetry.go`);
    const configTest = readText(tree, `${MODULE_DIR}/app/config_test.go`);

    for (const legacy of [
      '"OTLP_ENDPOINT"',
      '"OTLP_INSECURE"',
      '"SERVICE"',
      '"SERVICE_VERSION"',
      '"INSTANCE_ID"',
      'OTLPEndpoint',
      'OTLPInsecure',
    ]) {
      expect(config).not.toContain(legacy);
      expect(telemetry).not.toContain(legacy);
    }
    expect(config).not.toContain('Instance');
    expect(config).not.toContain('Version');
    expect(telemetry).not.toContain('Instance:');
    expect(telemetry).not.toContain('Version:');
    expect(configTest).toContain(
      'func TestTheIdentityOfTheProcessIsLeftToTheEnvironment(t *testing.T) {',
    );
    expect(config).toContain('if cfg.Signals, err = boot.SignalsFromEnv(lookup); err != nil {');
    expect(telemetry).toContain('Signals:  cfg.Signals,');
    expect(telemetry).not.toContain('Endpoint:');
    expect(telemetry).not.toContain('Insecure:');
  });

  it('should declare the effective configuration of each role to the process configured record', async () => {
    const tree = await generate();
    const telemetry = readText(tree, `${MODULE_DIR}/app/telemetry.go`);

    expect(telemetry).toContain('Settings: settings(cfg),');
    expect(telemetry).toContain('slog.Any("postgres", postgres.DescribeDSN(cfg.DSN))');
    expect(telemetry).toContain('slog.String("grpc_tls_key_file", presence(cfg.GRPCKeyFile)),');
    expect(telemetry).toContain('slog.Int("metric_tenants", len(cfg.MetricTenants)),');
    expect(telemetry).toContain('slog.Any("kafka", cfg.KafkaAuth),');
    expect(telemetry).toContain('slog.String("kafka_checkout_topic", cfg.CheckoutTopic),');
    expect(telemetry).toContain('slog.String("kafka_checkout_dlq", cfg.CheckoutDLQ),');
    expect(telemetry).not.toContain('cfg.Relay');
  });

  it('should hand the relay the telemetry of the runtime and the physical topic of each channel', async () => {
    const tree = await generate();
    const wiring = readText(tree, `${MODULE_DIR}/app/wiring.go`);

    expect(wiring).toContain(
      'func RelayConfig(cfg Config, rt *otelboot.Runtime, catalog channel.Catalog) relay.Config {',
    );
    for (const line of [
      'config.Tracer = rt.Tracer()',
      'config.System = semconv.MessagingSystemKafka.Value.AsString()',
      'config.LoggerProvider = rt.LoggerProvider()',
      'config.MeterProvider = rt.MeterProvider()',
      'config.Address = topicOf(catalog)',
      'ch, err := catalog.Resolve(destination)',
      'return ch.Address',
    ]) {
      expect(wiring).toContain(line);
    }
    expect(wiring).toContain(
      'relay.NewOverPostgres(pool, publisher, "checkout", RelayConfig(cfg, rt, catalog))',
    );
    expect(wiring).not.toContain('cfg.Relay)');
  });

  it('should hand every kernel library the logger provider of the runtime and log its own lines under its package', async () => {
    const tree = await generate();
    const wiring = readText(tree, `${MODULE_DIR}/app/wiring.go`);

    for (const line of [
      'idclock.SystemClock{}, rt.LoggerProvider(), fn)',
      'kernelgrpc.ServerInterceptors(rpc.ServiceName, ctrl, rt.Instruments(), rt.LoggerProvider(), kernelgrpc.WithCommands(rpc.Commands()...))',
      'LoggerProvider: rt.LoggerProvider(),',
      'kernelgrpc.HealthServices(rpc.ServiceName), ready, rt.LoggerProvider())',
      'postgres.WaitForTables(ctx, pool, time.Second, rt.LoggerProvider(), postgres.Tables(postgres.Outbox)...)',
      'rt.LoggerFor(reflect.TypeFor[Config]().PkgPath()).LogAttrs(ctx, slog.LevelInfo, "relay draining",',
      'slog.String(string(semconv.MessagingSystemKey), semconv.MessagingSystemKafka.Value.AsString()),',
      'slog.String(string(semconv.MessagingDestinationNameKey), cfg.CheckoutTopic))',
    ]) {
      expect(wiring).toContain(line);
    }
    expect(wiring).not.toMatch(/\.Logger\(\)/);
  });

  it('should meter the pool of each role with the provider of the runtime', async () => {
    const tree = await generate();
    const wiring = readText(tree, `${MODULE_DIR}/app/wiring.go`);
    const pools = wiring.match(/postgres\.NewPool\(/g) ?? [];
    const metered =
      wiring.match(
        /postgres\.NewPool\(ctx, cfg\.DSN, rt\.Tracer\(\), postgres\.WithMeterProvider\(rt\.MeterProvider\(\)\)\)/g,
      ) ?? [];

    expect(pools).toHaveLength(2);
    expect(metered).toHaveLength(pools.length);
  });

  it('should serve gRPC only, with the kernel chain and the migrate of the outbox, the inbox and the context schema', async () => {
    const tree = await generate();
    const wiring = readText(tree, `${MODULE_DIR}/app/wiring.go`);

    expect(wiring).toContain('kernelgrpc.ServerInterceptors(rpc.ServiceName');
    expect(wiring).toContain('server.RegisterService(&rpc.ServiceDesc, rpc.Server{})');
    expect(wiring).toContain(
      'postgres.Migrate(ctx, pool, []postgres.Capability{postgres.Outbox, postgres.Inbox}, provider.Schema)',
    );
    expect(wiring).not.toContain('net/http');
    expect(tree.exists(`${MODULE_DIR}/app/http`)).toBe(false);
  });

  it('should route the commands through the inbox and host the purge of each role', async () => {
    const tree = await generate();
    const wiring = readText(tree, `${MODULE_DIR}/app/wiring.go`);
    const config = readText(tree, `${MODULE_DIR}/app/config.go`);
    const service = readText(tree, `${MODULE_DIR}/app/rpc/service.go`);
    const errors = readText(tree, `${MODULE_DIR}/app/rpc/errors.go`);
    const commands = readText(tree, `${MODULE_DIR}/application/commands.go`);

    expect(commands).toContain('const CommandConsumer = "checkout.commands"');
    expect(service).toContain('func Commands() []string');
    expect(wiring).toContain('kernelgrpc.WithCommands(rpc.Commands()...)');
    expect(wiring).toContain(
      'postgres.PurgeExpiredInbox(ctx, pool, application.CommandConsumer, cutoff, batch)',
    );
    expect(wiring).toContain('postgres.PurgePublished(ctx, pool, cutoff, batch)');
    expect(errors).toContain('kernelgrpc.IdempotencyStatus(err)');
    expect(config).toContain('IdempotencyRetention: 24 * time.Hour');
    expect(config).toContain('OutboxRetention:      168 * time.Hour');
    expect(config).toContain('ErrInvalidPolicy');
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

  it('should declare the six Go targets, the two serve targets, the env file, the relay container, the distributed one, the e2e over it and no lint target', async () => {
    const tree = await generate();

    expect(Object.keys(projectOf(tree).targets).sort()).toEqual([
      'build',
      'deploy-env',
      'docker:run-relay',
      'e2e',
      'fmt-check',
      'govulncheck',
      'nx-release-publish',
      'serve-api',
      'serve-relay',
      'test-distributed',
      'test-race',
      'tidy',
      'vet',
    ]);
  });

  it('should serve each role by the single binary of the context', async () => {
    const tree = await generate();
    const targets = projectOf(tree).targets as Record<string, { options: { command: string } }>;

    expect(targets['serve-api'].options.command).toBe(SERVE_API);
    expect(targets['serve-relay'].options.command).toBe(SERVE_RELAY);
    expect(targets['docker:run-relay'].options.command).toBe(DOCKER_RUN_RELAY);
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
      TEST_INFRA,
      { projects: ['postgres', 'app'], target: 'test-race' },
      'test-race',
    ]);
    expect((target.options as { command: string }).command).toBe(
      `${TEST_ENV} go test -race -count=1 -p 1 -tags=integration,distributed ./distkit/...`,
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

  it('should register the context and its contract in go.work in sorted order', async () => {
    const tree = await generate();

    expect(useEntries(readText(tree, 'go.work'))).toEqual([
      './apps/backend/checkout',
      './apps/backend/checkout/contract',
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
      [...MODULE_FILES, 'application', 'domain', 'ports', 'provider', CONTRACT_DIR].sort(),
    );
  });

  it('should fall back to the schema defaults when blocks and directory are omitted', async () => {
    const tree = treeWithGoWork();

    await boundedContextGenerator(tree, {
      name: FULL_OPTIONS.name,
      boundedContext: FULL_OPTIONS.boundedContext,
    });

    expect(tree.children(MODULE_DIR).sort()).toEqual(
      [
        ...MODULE_FILES,
        ...BLOCK_DIRS,
        CMD_DIR,
        APPKIT_DIR,
        DISTKIT_DIR,
        DOCKERFILE,
        CONTRACT_DIR,
        DEPLOY_DIR,
      ].sort(),
    );
  });
});

describe('[generator] bounded-context — identifiers', () => {
  const name = 'order-fulfillment';

  it('should slug the module directory from the name', async () => {
    const tree = await generate({ name });

    expect(tree.children(DIRECTORY)).toEqual(['order-fulfillment']);
    expect(tree.children(`${DIRECTORY}/order-fulfillment`).sort()).toEqual(
      [
        ...MODULE_FILES,
        ...BLOCK_DIRS,
        CMD_DIR,
        APPKIT_DIR,
        DISTKIT_DIR,
        DOCKERFILE,
        CONTRACT_DIR,
        DEPLOY_DIR,
      ].sort(),
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

  it('should accept the contract block, which is always the contract module of the context', async () => {
    const tree = await generate({ blocks: [...ALL_BLOCKS, 'contract'] });

    expect(tree.exists(`${CONTRACT_MODULE_DIR}/go.mod`)).toBe(true);
    expect(tree.children(MODULE_DIR)).not.toContain('contract.go');
  });

  it('should refuse a contract module already registered in go.work', async () => {
    await expectRefusal({
      overrides: {},
      message: /go\.work/,
      prepare: (tree) => {
        tree.write(
          'go.work',
          GO_WORK.replace(
            '\t./libs/backend/go/app\n',
            '\t./apps/backend/checkout/contract\n\t./libs/backend/go/app\n',
          ),
        );
      },
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

  it('should refuse a directory other than apps/backend, which the deploy and infrasync read', async () => {
    await expectRefusal({ overrides: { directory: 'services' }, message: /directory/i });
  });

  it('should take the next free gRPC port after the contexts declared', async () => {
    const tree = await generate({}, (t) => {
      t.write('apps/backend/orders/deploy/.env.example', 'GRPC_ADDR=:9191\n');
      t.write('apps/backend/bookings/deploy/.env.example', 'GRPC_ADDR=127.0.0.1:9196\n');
    });

    expect(readText(tree, `${DEPLOY_MODULE_DIR}/.env.example`).split('\n')).toContain(
      'GRPC_ADDR=127.0.0.1:9197',
    );
  });

  it('should refuse a gRPC port another context declares', async () => {
    await expectRefusal({
      overrides: { grpcPort: 9191 },
      message: /grpcPort/,
      prepare: (t) => t.write('apps/backend/orders/deploy/.env.example', 'GRPC_ADDR=:9191\n'),
    });
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

  const INFRASYNC_ARGS = [
    'run',
    './tools/dmpf-conformance/cmd/infrasync',
    '--root',
    '.',
    '--write',
  ];

  it('should add the release group of the contract with the literal tag of its directory', async () => {
    const tree = treeWithGoWork();
    tree.write('nx.json', NX_JSON);

    await boundedContextGenerator(tree, FULL_OPTIONS);

    const groups = readJsonFile<{ release: { groups: Record<string, unknown> } }>(tree, 'nx.json')
      .release.groups;
    expect(groups['go-contract-checkout']).toEqual({
      projects: ['checkout-contract'],
      releaseTag: { pattern: 'apps/backend/checkout/contract/v{version}' },
    });
    expect(Object.keys(groups)).toEqual(['go-contract-checkout', 'go-libs', 'go-tools', 'npm']);
  });

  it('should keep every other line of nx.json as it was', async () => {
    const tree = treeWithGoWork();
    tree.write('nx.json', NX_JSON);

    await boundedContextGenerator(tree, FULL_OPTIONS);

    const inserted = [
      '      "go-contract-checkout": {',
      '        "projects": ["checkout-contract"],',
      '        "releaseTag": {',
      '          "pattern": "apps/backend/checkout/contract/v{version}"',
      '        }',
      '      },',
      '',
    ].join('\n');
    expect(readText(tree, 'nx.json')).toBe(
      NX_JSON.replace('    "groups": {\n', `    "groups": {\n${inserted}`),
    );
  });

  it('should leave nx.json alone when the release group already exists', async () => {
    const tree = treeWithGoWork();
    tree.write('nx.json', NX_JSON);
    await boundedContextGenerator(tree, FULL_OPTIONS);
    const once = readText(tree, 'nx.json');
    const again = treeWithGoWork();
    again.write('nx.json', once);

    await boundedContextGenerator(again, FULL_OPTIONS);

    expect(readText(again, 'nx.json')).toBe(once);
  });

  it('should include the deploy compose of the app in the local compose', async () => {
    const compose = [
      'name: local',
      '',
      'include:',
      '  - compose/postgres.yml',
      '  - ../../apps/backend/orders/deploy/compose.yml',
      '  - compose/swagger-ui.yml',
      '',
    ].join('\n');
    const tree = treeWithGoWork();
    tree.write('infra/local/docker-compose.yml', compose);

    await boundedContextGenerator(tree, FULL_OPTIONS);

    expect(readText(tree, 'infra/local/docker-compose.yml')).toBe(
      compose.replace(
        '  - ../../apps/backend/orders/deploy/compose.yml\n',
        '  - ../../apps/backend/orders/deploy/compose.yml\n  - ../../apps/backend/checkout/deploy/compose.yml\n',
      ),
    );
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

    expect(modsync).toHaveBeenCalledTimes(2);
    expect(modsync).toHaveBeenNthCalledWith(1, 'go', MODSYNC_ARGS, {
      cwd: tree.root,
      stdio: 'inherit',
    });
  });

  it('should regenerate the shared infra from the manifests after the module sync', async () => {
    const tree = treeWithGoWork();

    const callback = await boundedContextGenerator(tree, FULL_OPTIONS);
    await callback();

    expect(modsync).toHaveBeenNthCalledWith(2, 'go', INFRASYNC_ARGS, {
      cwd: tree.root,
      stdio: 'inherit',
    });
  });

  it('should not run infrasync for a context without deploy', async () => {
    const tree = treeWithGoWork();

    const callback = await boundedContextGenerator(tree, {
      ...FULL_OPTIONS,
      blocks: ['domain', 'port', 'application'],
    });
    await callback();

    expect(modsync).toHaveBeenCalledTimes(1);
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

  it('should print the baseline instruction naming --write-baseline', async () => {
    const printed = captureOutput();

    await generate();

    expect(printed()).toContain('2 módulos gerados');
    expect(printed()).toContain('--write-baseline');
  });
});

describe('[generator] bounded-context — contract module', () => {
  type ContractProject = ProjectConfig & {
    targets: Record<string, { options: { command: string; cwd?: string } }>;
  };

  const contractProjectOf = (tree: Tree): ContractProject =>
    readJsonFile<ContractProject>(tree, `${CONTRACT_MODULE_DIR}/project.json`);

  it('should create the contract as a Go module of its own beside the blocks', async () => {
    const tree = await generate();

    expect(tree.children(CONTRACT_MODULE_DIR).sort()).toEqual([
      'buf.gen.yaml',
      'buf.yaml',
      'dmpf-units.json',
      'doc.go',
      'go.mod',
      'package.json',
      'project.json',
      'proto',
    ]);
    expect(readText(tree, `${CONTRACT_MODULE_DIR}/go.mod`)).toBe(
      `module ${MODULE_PREFIX}/${CONTRACT_MODULE_DIR}\n\ngo ${GO_VERSION}\n`,
    );
  });

  it('should name the contract project <name>-contract with the contract layer', async () => {
    const project = contractProjectOf(await generate());

    expect(project.name).toBe('checkout-contract');
    expect(project.$schema).toBe(`../${NX_PROJECT_SCHEMA}`);
    expect(project.projectType).toBe('library');
    expect(project.sourceRoot).toBe(CONTRACT_MODULE_DIR);
    expect(project.tags).toEqual(['type:lib', 'scope:backend', 'stack:go', 'layer:contract']);
  });

  it('should run the Buf gates on the contract directory with the project name', async () => {
    const targets = contractProjectOf(await generate()).targets;

    for (const gate of ['warmup', 'lint', 'pins', 'generate-check', 'breaking']) {
      expect(targets[`buf-${gate}`].options.command).toBe(
        `bash tools/buf-gate.sh ${gate} ${CONTRACT_MODULE_DIR} --project checkout-contract`,
      );
    }
    expect(targets.tidy.options.command).toBe('bash ../../../../tools/go-tidy.sh');
    expect(Object.keys(targets).sort()).toEqual([
      'buf-breaking',
      'buf-generate-check',
      'buf-lint',
      'buf-pins',
      'buf-warmup',
      'build',
      'fmt-check',
      'govulncheck',
      'test-race',
      'tidy',
      'vet',
    ]);
  });

  it('should publish the contract under the go_package_prefix of its own module', async () => {
    const tree = await generate();

    expect(readText(tree, `${CONTRACT_MODULE_DIR}/buf.gen.yaml`)).toContain(
      `value: ${MODULE_PREFIX}/${CONTRACT_MODULE_DIR}/gen/go`,
    );
    expect(readText(tree, `${CONTRACT_MODULE_DIR}/buf.gen.yaml`)).toContain('out: gen/go');
    expect(readText(tree, `${CONTRACT_MODULE_DIR}/buf.yaml`)).toContain(
      'name: buf.build/mateusmacedo/checkout-contract',
    );
  });

  it('should declare the contract unit as public integration surface of the bounded context', async () => {
    const manifest = readJsonFile<Manifest>(
      await generate(),
      `${CONTRACT_MODULE_DIR}/dmpf-units.json`,
    );

    expect(manifest.units).toEqual([
      {
        id: 'sales/contract',
        block: 'contract',
        bounded_context: 'sales',
        public_integration_surface: true,
        include: [
          `${MODULE_PREFIX}/${CONTRACT_MODULE_DIR}`,
          `${MODULE_PREFIX}/${CONTRACT_MODULE_DIR}/gen/go/company/checkout/service/v1`,
        ],
      },
    ]);
  });

  it('should declare the protobuf runtime that the generated code imports as the contract external', async () => {
    const manifest = readJsonFile<{ external: { package: string; capability: string }[] }>(
      await generate(),
      `${CONTRACT_MODULE_DIR}/dmpf-units.json`,
    );

    expect(manifest.external.map(({ package: pkg, capability }) => ({ pkg, capability }))).toEqual([
      { pkg: 'google.golang.org/protobuf', capability: 'wire.codec' },
    ]);
  });

  it('should write a private package.json named after the contract project', async () => {
    expect(
      readJsonFile<PackageManifest>(await generate(), `${CONTRACT_MODULE_DIR}/package.json`),
    ).toEqual({ name: '@mateusmacedo/checkout-contract', version: '0.0.0', private: true });
  });
});

describe('[generator] bounded-context — deploy', () => {
  type InfraManifest = {
    schema: string;
    app: string;
    image: { env: string; local: string };
    database: { passwordEnv: string; local: string; dev: string };
    kafka: {
      passwordEnv: string;
      local: string;
      topics: { name: string; env: string }[];
      acls: { operations: string[]; topics: string[] }[];
    };
    grpc: { server: string };
    openapi: boolean;
  };

  it('should give the app the deploy of Kubernetes, Compose, environment and platform needs', async () => {
    const tree = await generate();

    expect(tree.children(DEPLOY_MODULE_DIR).sort()).toEqual([
      '.env.example',
      'compose.yml',
      'infra.json',
      'k8s',
    ]);
    expect(tree.children(`${DEPLOY_MODULE_DIR}/k8s/base`).sort()).toEqual([
      'configmap.yaml',
      'deployment-api.yaml',
      'deployment-relay.yaml',
      'kustomization.yaml',
      'networkpolicy.yaml',
      'pdb-api.yaml',
      'service-api.yaml',
      'serviceaccount.yaml',
    ]);
    expect(tree.children(`${DEPLOY_MODULE_DIR}/k8s/overlays/dev`).sort()).toEqual([
      'configmap-checkout-patch.yaml',
      'deployment-checkout-api-patch.yaml',
      'kustomization.yaml',
    ]);
    expect(tree.children(`${DEPLOY_MODULE_DIR}/k8s/overlays/hmg`).sort()).toEqual([
      'configmap-checkout-patch.yaml',
      'deployment-checkout-api-patch.yaml',
      'deployment-checkout-relay-patch.yaml',
      'kustomization.yaml',
      'secrets.example.yaml.tmpl',
    ]);
  });

  it('should declare the database, the topics and the workload the platform provisions', async () => {
    const infra = readJsonFile<InfraManifest>(await generate(), `${DEPLOY_MODULE_DIR}/infra.json`);

    expect(infra).toEqual({
      schema: 'dmpf/infra@1',
      app: 'checkout',
      image: { env: 'CHECKOUT_IMAGE', local: 'checkout:local' },
      database: {
        passwordEnv: 'CHECKOUT_PG_PASSWORD',
        local: 'checkout-local',
        dev: 'checkout-dev',
      },
      kafka: {
        passwordEnv: 'KAFKA_CHECKOUT_PASSWORD',
        local: 'checkout-local',
        topics: [
          { name: 'checkout.events', env: 'KAFKA_CHECKOUT_TOPIC' },
          { name: 'checkout.events.dlq', env: 'KAFKA_CHECKOUT_DLQ' },
        ],
        acls: [
          {
            operations: ['write', 'describe'],
            topics: ['checkout.events', 'checkout.events.dlq'],
          },
        ],
      },
      grpc: { server: 'dmpf-checkout-api' },
      openapi: false,
    });
  });

  it('should run the local processes with full sampling, debug logs and OTLP log export', async () => {
    const env = readText(await generate(), `${DEPLOY_MODULE_DIR}/.env.example`);

    for (const line of [
      'OTEL_SERVICE_NAME=checkout',
      'OTEL_RESOURCE_ATTRIBUTES=service.version=local,service.instance.id=checkout-local,deployment.environment.name=local',
      'PG_DSN=postgres://checkout:checkout-local@localhost:5432/checkout?sslmode=disable',
      'KAFKA_CHECKOUT_TOPIC=checkout.events',
      'KAFKA_CHECKOUT_DLQ=checkout.events.dlq',
      'OTEL_EXPORTER_OTLP_PROTOCOL=grpc',
      'OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317',
      'OTEL_TRACES_SAMPLER_ARG=1.0',
      'OTEL_LOGS_EXPORTER=otlp',
      'OTEL_PROPAGATORS=tracecontext',
      'OTEL_GO_X_OBSERVABILITY=true',
      'OTEL_BSP_EXPORT_TIMEOUT=3000',
      'OTEL_BLRP_EXPORT_TIMEOUT=3000',
      'LOG_LEVEL=debug',
    ]) {
      expect(env.split('\n')).toContain(line);
    }
  });

  it('should give each served role its own instance and its role over the env file that every role shares', async () => {
    const tree = await generate();
    const targets = projectOf(tree).targets as Record<
      string,
      { options: { command: string; envFile: string } }
    >;
    const instances = new Set<string>();

    expect(readText(tree, `${DEPLOY_MODULE_DIR}/.env.example`)).not.toContain('dmpf.process.role=');
    for (const role of ['api', 'relay']) {
      const { command, envFile } = targets[`serve-${role}`].options;
      const appended =
        /OTEL_RESOURCE_ATTRIBUTES="\$\{OTEL_RESOURCE_ATTRIBUTES:\+\$OTEL_RESOURCE_ATTRIBUTES,\}([^"]*)"/.exec(
          command,
        );
      const steps = [
        '. deploy/.env;',
        'OTEL_RESOURCE_ATTRIBUTES=',
        'set +a;',
        `exec go run ./cmd --role ${role}`,
      ].map((step) => command.indexOf(step));

      expect(envFile).toBe('{projectRoot}/deploy/.env.example');
      expect(appended?.[1].split(',')).toEqual([
        `service.instance.id=checkout-local-${role}`,
        `dmpf.process.role=${role}`,
      ]);
      expect(steps.every((at, step) => at >= 0 && (step === 0 || at > steps[step - 1]))).toBe(true);
      instances.add(appended?.[1].split(',')[0] ?? '');
    }
    expect(instances.size).toBe(2);
  });

  const LAUNCH_DIR = path.join(__dirname, '../../../test-output/launch');
  const resourceLineOf = (env: string): string =>
    /^OTEL_RESOURCE_ATTRIBUTES=(.*)$/m.exec(env)?.[1] ?? '';
  const envFileValues = (content: string): Record<string, string> =>
    Object.fromEntries(
      content
        .split('\n')
        .map((line) => line.trim())
        .filter((line) => line.includes('=') && !line.startsWith('#'))
        .map((line) => [line.slice(0, line.indexOf('=')), line.slice(line.indexOf('=') + 1)]),
    );
  const resourceAfterSetupOf = (setup: string, env: Record<string, string>): string => {
    const run = childProcess.spawnSync(
      'sh',
      ['-c', `${setup} printf %s "$OTEL_RESOURCE_ATTRIBUTES"`],
      {
        cwd: __dirname,
        env: { PATH: process.env.PATH ?? '', ...env },
        encoding: 'utf-8',
      },
    );
    expect(run.status).toBe(0);
    return run.stdout;
  };
  const containerEnvOf = (
    command: string,
    deployEnv: string,
    role: string,
  ): Record<string, string> => {
    const bin = path.join(LAUNCH_DIR, 'bin');
    fs.mkdirSync(bin, { recursive: true });
    fs.mkdirSync(path.join(LAUNCH_DIR, 'deploy'), { recursive: true });
    fs.writeFileSync(path.join(bin, 'docker'), '#!/bin/sh\nprintf \'%s\\0\' "$@"\n');
    fs.chmodSync(path.join(bin, 'docker'), 0o700);
    fs.writeFileSync(path.join(LAUNCH_DIR, 'deploy', '.env'), deployEnv);
    const run = childProcess.spawnSync('sh', ['-c', command], {
      cwd: LAUNCH_DIR,
      env: { PATH: `${bin}${path.delimiter}${process.env.PATH ?? ''}` },
      encoding: 'utf-8',
    });
    const args = run.stdout.replace(/\0$/, '').split('\0');
    const launch = ['apps-backend-checkout', '--role', role];
    const files: string[] = [];
    const overrides: Record<string, string> = {};
    for (let at = 1; at < args.length - launch.length; at++) {
      if (args[at] === '--env-file') {
        files.push(args[++at]);
      } else if (args[at] === '-e' || args[at] === '--env') {
        const pair = args[++at];
        overrides[pair.slice(0, pair.indexOf('='))] = pair.slice(pair.indexOf('=') + 1);
      }
    }

    expect(run.status).toBe(0);
    expect(args[0]).toBe('run');
    expect(args.slice(-launch.length)).toEqual(launch);
    expect(files).toEqual(['deploy/.env']);
    return {
      ...envFileValues(fs.readFileSync(path.join(LAUNCH_DIR, 'deploy', '.env'), 'utf-8')),
      ...overrides,
    };
  };

  it('should start each served role with its own instance and no leading comma when nothing declares the resource', async () => {
    const tree = await generate();
    const targets = projectOf(tree).targets as Record<string, { options: { command: string } }>;
    const shared = resourceLineOf(readText(tree, `${DEPLOY_MODULE_DIR}/.env.example`));

    for (const role of ['api', 'relay']) {
      const { command } = targets[`serve-${role}`].options;
      const launch = `exec go run ./cmd --role ${role}`;
      const setup = command.slice(0, command.length - launch.length);
      const own = `service.instance.id=checkout-local-${role},dmpf.process.role=${role}`;

      expect(command.endsWith(launch)).toBe(true);
      expect(resourceAfterSetupOf(setup, {})).toBe(own);
      expect(resourceAfterSetupOf(setup, { OTEL_RESOURCE_ATTRIBUTES: shared })).toBe(
        `${shared},${own}`,
      );
    }
  });

  it('should run the relay container with its own instance and role over the env file that every role shares', async () => {
    const tree = await generate();
    const targets = projectOf(tree).targets as Record<string, { options: { command: string } }>;
    const shared = readText(tree, `${DEPLOY_MODULE_DIR}/.env.example`);
    const own = 'service.instance.id=checkout-local-relay,dmpf.process.role=relay';
    const { command } = targets['docker:run-relay'].options;

    const env = containerEnvOf(command, shared, 'relay');
    expect(env.OTEL_RESOURCE_ATTRIBUTES).toBe(`${resourceLineOf(shared)},${own}`);
    expect(env.OTEL_SERVICE_NAME).toBe('checkout');
    expect(
      containerEnvOf(command, shared.replace(/^OTEL_RESOURCE_ATTRIBUTES=.*\n/m, ''), 'relay')
        .OTEL_RESOURCE_ATTRIBUTES,
    ).toBe(own);
  });

  it('should declare no legacy telemetry variable in any generated file', async () => {
    const tree = await generate();
    const legacy =
      /(^|[^_A-Z])(OTLP_ENDPOINT|OTLP_INSECURE|OTLP_LOGS|TRACE_SAMPLE_RATE|SERVICE|SERVICE_VERSION|INSTANCE_ID)\b/m;
    const offenders = Object.entries(changesOf(tree))
      .filter(([path]) => path.startsWith(`${MODULE_DIR}/`))
      .filter(([, content]) => legacy.test(content))
      .map(([path]) => path);

    expect(offenders).toEqual([]);
  });

  it('should hand every process of the cluster the OTEL_* environment of the platform', async () => {
    const tree = await generate();
    const configmap = readText(tree, `${DEPLOY_MODULE_DIR}/k8s/base/configmap.yaml`);

    for (const line of [
      '  OTEL_SERVICE_NAME: checkout',
      '  OTEL_EXPORTER_OTLP_PROTOCOL: grpc',
      '  OTEL_EXPORTER_OTLP_ENDPOINT: http://otel-collector:4317',
      "  OTEL_TRACES_SAMPLER_ARG: '1.0'",
      '  OTEL_LOGS_EXPORTER: otlp',
      '  OTEL_PROPAGATORS: tracecontext',
      "  OTEL_GO_X_OBSERVABILITY: 'true'",
      "  OTEL_BSP_EXPORT_TIMEOUT: '3000'",
      "  OTEL_BLRP_EXPORT_TIMEOUT: '3000'",
    ]) {
      expect(configmap.split('\n')).toContain(line);
    }
    for (const environment of ['dev', 'hmg']) {
      const patch = readText(
        tree,
        `${DEPLOY_MODULE_DIR}/k8s/overlays/${environment}/configmap-checkout-patch.yaml`,
      );
      expect(patch.split('\n')).toContain(
        `  OTEL_RESOURCE_ATTRIBUTES: service.version=${environment},deployment.environment.name=${environment}`,
      );
    }
  });

  it('should export OTLP over TLS in hmg, trusting the CA mounted in every workload, and in clear text in dev', async () => {
    const tree = await generate();
    const hmg = readText(
      tree,
      `${DEPLOY_MODULE_DIR}/k8s/overlays/hmg/configmap-checkout-patch.yaml`,
    ).split('\n');

    expect(hmg).toContain('  OTEL_EXPORTER_OTLP_ENDPOINT: https://otel-collector:4317');
    expect(hmg).toContain('  OTEL_EXPORTER_OTLP_CERTIFICATE: /etc/dmpf/otel/ca.crt');
    expect(
      readText(tree, `${DEPLOY_MODULE_DIR}/k8s/overlays/dev/configmap-checkout-patch.yaml`),
    ).not.toContain('OTEL_EXPORTER_OTLP_ENDPOINT');
    for (const role of ['api', 'relay']) {
      const patch = readText(
        tree,
        `${DEPLOY_MODULE_DIR}/k8s/overlays/hmg/deployment-checkout-${role}-patch.yaml`,
      );
      expect(patch).toContain(
        [
          '            - name: otel-collector-ca',
          '              mountPath: /etc/dmpf/otel',
          '              readOnly: true',
        ].join('\n'),
      );
      expect(patch).toContain(
        [
          '        - name: otel-collector-ca',
          '          secret:',
          '            secretName: otel-collector-ca',
        ].join('\n'),
      );
    }
    expect(
      readText(tree, `${DEPLOY_MODULE_DIR}/k8s/overlays/hmg/kustomization.yaml`).split('\n'),
    ).toContain('  - path: deployment-checkout-relay-patch.yaml');
  });

  it('should give every workload the time to flush its telemetry before it is killed', async () => {
    const tree = await generate();

    for (const role of ['api', 'relay']) {
      expect(
        readText(tree, `${DEPLOY_MODULE_DIR}/k8s/base/deployment-${role}.yaml`).split('\n'),
      ).toContain('      terminationGracePeriodSeconds: 30');
    }
    expect(
      readText(tree, `${DEPLOY_MODULE_DIR}/compose.yml`).match(/^ {4}stop_grace_period: 30s$/gm),
    ).toHaveLength(2);
  });

  it('should name the instance and the role of each workload in the resource', async () => {
    const tree = await generate();

    for (const role of ['api', 'relay']) {
      const deployment = readText(tree, `${DEPLOY_MODULE_DIR}/k8s/base/deployment-${role}.yaml`);
      expect(deployment).toContain(
        [
          '            - name: K8S_POD_NAME',
          '              valueFrom:',
          '                fieldRef:',
          '                  fieldPath: metadata.name',
          '            - name: OTEL_RESOURCE_ATTRIBUTES',
          `              value: $(OTEL_RESOURCE_ATTRIBUTES),service.instance.id=$(K8S_POD_NAME),dmpf.process.role=${role}`,
        ].join('\n'),
      );
    }
  });

  it('should name the service, the instance and the role of each local process', async () => {
    const compose = readText(await generate(), `${DEPLOY_MODULE_DIR}/compose.yml`);

    expect(compose).toContain('  OTEL_SERVICE_NAME: checkout');
    for (const role of ['api', 'relay']) {
      expect(compose).toContain(
        `      OTEL_RESOURCE_ATTRIBUTES: \${OTEL_RESOURCE_ATTRIBUTES:-service.version=local,deployment.environment.name=local},service.instance.id=checkout-${role},dmpf.process.role=${role}`,
      );
    }
  });

  it('should extend the shared services of the local compose for the api and the relay', async () => {
    const compose = readText(await generate(), `${DEPLOY_MODULE_DIR}/compose.yml`);

    expect(compose).toContain('  dmpf-checkout-api:');
    expect(compose).toContain('  dmpf-checkout-relay:');
    expect(compose).toContain('file: ../../../../infra/local/compose/app-base.yml');
    expect(compose).toContain('dockerfile: apps/backend/checkout/Dockerfile');
  });

  it('should leave no deploy when the context has no app block', async () => {
    const tree = await generate({ blocks: ['domain', 'port', 'application'] });

    expect(tree.children(MODULE_DIR)).not.toContain(DEPLOY_DIR);
  });
});
