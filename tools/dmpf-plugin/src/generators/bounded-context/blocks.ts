import type { Block } from './schema';

export type ExternalDependency = {
  package: string;
  versions: string;
  entrypoints: readonly string[];
  capability: string;
};

export type Layer = 'domain' | 'services' | 'contract' | 'providers' | 'apps';

export type BlockLayout = {
  block: Block;
  unitSuffix: string;
  dirName: string;
  layer: Layer;
  requires: readonly Block[];
  integration: boolean;
  dependsOnProjects: readonly string[];
  external: readonly ExternalDependency[];
  role: string;
  summary: string;
};

// Copied verbatim from libs/backend/go/postgres/dmpf-units.json:
// the generated module is workspace-only and resolves both through the very same
// go.work entries, so it must declare the very same capabilities and entrypoints.
const PGX: ExternalDependency = {
  package: 'github.com/jackc/pgx/v5',
  versions: '>=5.10 <6',
  entrypoints: ['.', 'github.com/jackc/pgx/v5/pgxpool', 'github.com/jackc/pgx/v5/pgconn'],
  capability: 'io.storage',
};

const PROTOBUF: ExternalDependency = {
  package: 'google.golang.org/protobuf',
  versions: '>=1.36.12 <2',
  entrypoints: ['google.golang.org/protobuf/proto'],
  capability: 'wire.codec',
};

export const CONTRACT_BLOCK = 'contract';

// The kernel Postgres provider truncates the same tables every context harness
// truncates, so the two test-race targets must not share the job's database at once.
const KERNEL_POSTGRES_PROJECT = 'postgres';

const LAYER_ORDER: readonly Layer[] = ['domain', 'services', 'contract', 'providers', 'apps'];

export const LAYOUTS: readonly BlockLayout[] = [
  {
    block: 'domain',
    unitSuffix: 'domain',
    dirName: 'domain',
    layer: 'domain',
    requires: [],
    integration: false,
    dependsOnProjects: [],
    external: [],
    role: 'the deterministic aggregates and the outcome of their UPRs',
    summary:
      'Agregados síncronos e determinísticos, com o desfecho da UPR — sem porta, relógio, contexto nem tipo de wire.',
  },
  {
    block: 'port',
    unitSuffix: 'ports',
    dirName: 'ports',
    layer: 'domain',
    requires: ['domain'],
    integration: false,
    dependsOnProjects: [],
    external: [],
    role: 'the declared boundary to state, outbox and inbox, with no realization here',
    summary:
      'Fronteira declarada — repositórios, outbox e inbox em tipos do domínio, sem nenhuma realização.',
  },
  {
    block: 'application',
    unitSuffix: 'application',
    dirName: 'application',
    layer: 'services',
    requires: ['domain', 'port'],
    integration: false,
    dependsOnProjects: [],
    external: [],
    role: 'the caller side of the UPR, from the write use cases to the disposition of consumption',
    summary:
      'Lado do chamador da UPR: casos de uso de escrita na sequência canônica e a disposição do consumo.',
  },
  {
    block: 'provider',
    unitSuffix: 'provider-postgres',
    dirName: 'provider',
    layer: 'providers',
    requires: ['domain', 'port', 'application'],
    integration: true,
    dependsOnProjects: [KERNEL_POSTGRES_PROJECT],
    external: [PGX, PROTOBUF],
    role: 'the PostgreSQL realization of the transactional ports of this context',
    summary:
      'Realização em PostgreSQL das portas transacionais do contexto, com esquema e mapeadores próprios.',
  },
  {
    block: 'app',
    unitSuffix: 'app',
    dirName: 'app',
    layer: 'apps',
    requires: ['domain', 'port', 'application', 'provider'],
    integration: true,
    dependsOnProjects: [],
    external: [],
    role: 'the composition between the transport delivery and the application service',
    summary:
      'Composição entre a entrega do transporte e o serviço de aplicação, com a borda HTTP do contexto.',
  },
];

export const BLOCK_NAMES: readonly Block[] = LAYOUTS.map((layout) => layout.block);

export const layoutOf = (block: Block): BlockLayout => {
  const layout = LAYOUTS.find((candidate) => candidate.block === block);
  if (layout === undefined) {
    throw new Error(`bounded-context: no layout declared for block "${block}"`);
  }
  return layout;
};

export const isBlock = (value: string): value is Block =>
  BLOCK_NAMES.some((block) => block === value);

export const orderBlocks = (blocks: readonly Block[]): Block[] =>
  BLOCK_NAMES.filter((block) => blocks.includes(block));

export const missingDependencies = (blocks: readonly Block[]): Block[] => {
  const required = new Set<Block>();
  for (const block of blocks) {
    for (const dependency of layoutOf(block).requires) {
      if (!blocks.includes(dependency)) {
        required.add(dependency);
      }
    }
  }
  return orderBlocks([...required]);
};

// A single module runs at one CI stage; the stage is the one of its highest block,
// because it is the first stage where every dependency of the module is available.
export const highestLayer = (layouts: readonly BlockLayout[]): Layer =>
  layouts.reduce<Layer>(
    (highest, layout) =>
      LAYER_ORDER.indexOf(layout.layer) > LAYER_ORDER.indexOf(highest) ? layout.layer : highest,
    LAYER_ORDER[0],
  );

export const externalOf = (layouts: readonly BlockLayout[]): ExternalDependency[] => {
  const seen = new Set<string>();
  const merged: ExternalDependency[] = [];
  for (const dependency of layouts.flatMap((layout) => layout.external)) {
    if (!seen.has(dependency.package)) {
      seen.add(dependency.package);
      merged.push(dependency);
    }
  }
  return merged;
};
