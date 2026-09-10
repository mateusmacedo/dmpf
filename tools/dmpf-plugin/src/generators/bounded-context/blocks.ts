import type { Block } from './schema';

export type ExternalDependency = {
  package: string;
  versions: string;
  entrypoints: readonly string[];
  capability: string;
};

export type BlockLayout = {
  block: Block;
  suffix: string;
  layer: string;
  packageSuffix: string;
  requires: readonly Block[];
  integration: boolean;
  dependsOnBlocks: readonly Block[];
  external: readonly ExternalDependency[];
  role: string;
  summary: string;
};

// Copied verbatim from libs/backend/go/dmpf-provider-postgres/dmpf-units.json:
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

export const LAYOUTS: readonly BlockLayout[] = [
  {
    block: 'domain',
    suffix: 'domain',
    layer: 'domain',
    packageSuffix: 'domain',
    requires: [],
    integration: false,
    dependsOnBlocks: [],
    external: [],
    role: 'the deterministic aggregates and the outcome of their UPRs',
    summary:
      'Agregados síncronos e determinísticos, com o desfecho da UPR — sem porta, relógio, contexto nem tipo de wire.',
  },
  {
    block: 'port',
    suffix: 'ports',
    layer: 'domain',
    packageSuffix: 'ports',
    requires: ['domain'],
    integration: false,
    dependsOnBlocks: [],
    external: [],
    role: 'the declared boundary to state, outbox and inbox, with no realization here',
    summary:
      'Fronteira declarada — repositórios, outbox e inbox em tipos do domínio, sem nenhuma realização.',
  },
  {
    block: 'application',
    suffix: 'application',
    layer: 'services',
    packageSuffix: 'application',
    requires: ['domain', 'port'],
    integration: false,
    dependsOnBlocks: [],
    external: [],
    role: 'the caller side of the UPR, from the write use cases to the disposition of consumption',
    summary:
      'Lado do chamador da UPR: casos de uso de escrita na sequência canônica e a disposição do consumo.',
  },
  {
    block: 'provider',
    suffix: 'provider-postgres',
    layer: 'providers',
    packageSuffix: 'postgres',
    requires: ['domain', 'port', 'application'],
    integration: true,
    dependsOnBlocks: [],
    external: [PGX, PROTOBUF],
    role: 'the PostgreSQL realization of the transactional ports of this context',
    summary:
      'Realização em PostgreSQL das portas transacionais do contexto, com esquema e mapeadores próprios.',
  },
  {
    block: 'app',
    suffix: 'app',
    layer: 'apps',
    packageSuffix: 'app',
    requires: ['domain', 'port', 'application', 'provider'],
    integration: true,
    dependsOnBlocks: ['provider'],
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
