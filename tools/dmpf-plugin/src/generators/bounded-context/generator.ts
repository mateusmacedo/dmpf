import { execFileSync } from 'node:child_process';
import type { GeneratorCallback, Tree } from '@nx/devkit';
import { generateFiles, logger } from '@nx/devkit';
import { templatesDir } from '../../lib/paths';
import { readVersions } from '../../lib/versions';
import type { BlockLayout } from './blocks';
import {
  BLOCK_NAMES,
  CONTRACT_BLOCK,
  externalOf,
  highestLayer,
  isBlock,
  layoutOf,
  missingDependencies,
  orderBlocks,
} from './blocks';
import { parseGoWork, registerModules, useEntryOf } from './go-work';
import { IDENTIFIER_PATTERN, isIdentifier } from './identifiers';
import { externalFragment, unitsFragment } from './manifest';
import type { Block, BoundedContextGeneratorSchema } from './schema';

const MODULE_PREFIX = 'github.com/mateusmacedo/dmpf';
const DEFAULT_DIRECTORY = 'apps/backend';
const GO_WORK = 'go.work';
const NPM_SCOPE = '@mateusmacedo';
const MODSYNC_ARGS = ['run', './tools/dmpf-conformance/cmd/modsync', '--root', '.', '--write'];
const INFRASYNC_ARGS = ['run', './tools/dmpf-conformance/cmd/infrasync', '--root', '.', '--write'];
const NX_JSON = 'nx.json';
const LOCAL_COMPOSE = 'infra/local/docker-compose.yml';
const CONTRACT_DIRECTORY = 'contract';
const DEPLOY_DIRECTORY = 'deploy';
const DEFAULT_GRPC_PORT = 9194;
const SERVICE_NAME_PATTERN = /^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)*\.[A-Z][A-Za-z0-9]*$/;

const BASELINE_INSTRUCTION = [
  'Unidades novas mudam a classificação. Regrave o baseline:',
  '  go run ./tools/dmpf-conformance/cmd/conformance --root . --write-baseline',
].join('\n');

type Substitutions = Record<string, string | boolean>;

type PlannedBlock = {
  layout: BlockLayout;
  directory: string;
  substitutions: Substitutions;
};

type Plan = {
  directory: string;
  contractDirectory: string;
  useEntries: readonly string[];
  substitutions: Substitutions;
  blocks: readonly PlannedBlock[];
  goWork: string;
};

const refuse = (reason: string): never => {
  throw new Error(`bounded-context: ${reason}`);
};

const validatedName = (name: string | undefined): string => {
  if (name === undefined || name.trim().length === 0) {
    return refuse(
      'option name is required: it names the context directory, the Nx project and the Go module',
    );
  }
  if (!isIdentifier(name)) {
    return refuse(`option name ${JSON.stringify(name)} must match ${IDENTIFIER_PATTERN.source}`);
  }
  return name;
};

const validatedBoundedContext = (boundedContext: string | undefined): string => {
  if (boundedContext === undefined || boundedContext.trim().length === 0) {
    return refuse('bounded_context is declared, never inferred (ADR-012): pass --bounded-context');
  }
  if (!isIdentifier(boundedContext)) {
    return refuse(
      `option boundedContext ${JSON.stringify(boundedContext)} must match ${IDENTIFIER_PATTERN.source}`,
    );
  }
  return boundedContext;
};

const validatedDirectory = (directory: string | undefined): string => {
  const value = directory === undefined || directory.length === 0 ? DEFAULT_DIRECTORY : directory;
  const segments = value.split('/');
  const escapes = value.startsWith('/') || segments.includes('..') || segments.includes('');
  if (escapes) {
    return refuse(
      `option directory ${JSON.stringify(value)} must be a path relative to the workspace root, with no ".." segment`,
    );
  }
  if (value !== DEFAULT_DIRECTORY) {
    return refuse(
      `option directory ${JSON.stringify(value)} must be ${DEFAULT_DIRECTORY}: the Dockerfile, the deploy compose and infrasync read ${DEFAULT_DIRECTORY}/<name>`,
    );
  }
  return value;
};

const pascalOf = (name: string): string =>
  name
    .split('-')
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join('');

const validatedServiceName = ({
  serviceName,
  name,
}: {
  serviceName: string | undefined;
  name: string;
}): string => {
  if (serviceName === undefined || serviceName.length === 0) {
    return `company.${name.replaceAll('-', '')}.service.v1.${pascalOf(name)}Service`;
  }
  if (!SERVICE_NAME_PATTERN.test(serviceName)) {
    return refuse(
      `option serviceName ${JSON.stringify(serviceName)} must match ${SERVICE_NAME_PATTERN.source}`,
    );
  }
  return serviceName;
};

// The contract is never a package of the context module: it is always generated
// as the contract module beside it, so naming the block asks for nothing more.
const validatedBlocks = (blocks: readonly string[] | undefined): Block[] => {
  const requested = (blocks === undefined ? [...BLOCK_NAMES] : [...new Set(blocks)]).filter(
    (block) => block !== CONTRACT_BLOCK,
  );
  const unknown = requested.filter((block) => !isBlock(block));
  if (unknown.length > 0) {
    return refuse(
      `unknown block(s) ${unknown.join(', ')}: the generated blocks are ${BLOCK_NAMES.join(', ')}`,
    );
  }
  const selected = requested.filter(isBlock);
  if (selected.length === 0) {
    return refuse(`option blocks must name at least one of ${BLOCK_NAMES.join(', ')}`);
  }
  const missing = missingDependencies(selected);
  if (missing.length > 0) {
    return refuse(
      `blocks ${selected.join(', ')} leave dependencies open: add the missing block(s) ${missing.join(', ')}`,
    );
  }
  return orderBlocks(selected);
};

// The tag the @nx/docker plugin gives the image of a project root
// (getProjectNameFromPath in its plugin.js).
const imageRefOf = (projectRoot: string): string =>
  projectRoot.replace(/[\\/\s]+/g, '-').toLowerCase();

const roleResourceOf = (name: string, role: string): string =>
  `\${OTEL_RESOURCE_ATTRIBUTES:+$OTEL_RESOURCE_ATTRIBUTES,}service.instance.id=${name}-local-${role},dmpf.process.role=${role}`;

const serveCommandOf = (name: string, role: string): string =>
  `set -a; [ ! -f deploy/.env ] || . deploy/.env; OTEL_RESOURCE_ATTRIBUTES="${roleResourceOf(name, role)}"; set +a; exec go run ./cmd --role ${role}`;

const dockerRunCommandOf = (name: string, image: string, role: string): string =>
  `[ ! -f deploy/.env ] || . deploy/.env; exec docker run --rm --name ${name}-${role} --network host --env-file deploy/.env -e OTEL_RESOURCE_ATTRIBUTES="${roleResourceOf(name, role)}" ${image} --role ${role}`;

const testRaceCommandOf = (integration: boolean): string =>
  integration ? 'go test -race -count=1 -p 1 -tags=integration ./...' : 'go test -race ./...';

const GRPC_ADDR_PATTERN = /^GRPC_ADDR=[^:\n]*:(\d+)$/m;

const declaredGrpcPorts = (tree: Tree): number[] =>
  tree.children(DEFAULT_DIRECTORY).flatMap((app) => {
    const env = tree.read(`${DEFAULT_DIRECTORY}/${app}/${DEPLOY_DIRECTORY}/.env.example`, 'utf-8');
    const match = env === null ? null : GRPC_ADDR_PATTERN.exec(env);
    return match === null ? [] : [Number(match[1])];
  });

const validatedGrpcPort = (tree: Tree, port: number | undefined): number => {
  const taken = declaredGrpcPorts(tree);
  const value = port ?? Math.max(DEFAULT_GRPC_PORT - 1, ...taken) + 1;
  if (!Number.isInteger(value) || value < 1024 || value > 65535) {
    return refuse(`option grpcPort ${JSON.stringify(port)} must be an integer from 1024 to 65535`);
  }
  if (taken.includes(value)) {
    return refuse(
      `option grpcPort ${value} is taken by another context; omit it to take the next free one`,
    );
  }
  return value;
};

const blocksTableOf = ({
  layouts,
  boundedContext,
}: {
  layouts: readonly BlockLayout[];
  boundedContext: string;
}): string =>
  layouts
    .map(
      (layout) =>
        `| \`${layout.dirName}\` | \`${layout.block}\` | \`${boundedContext}/${layout.unitSuffix}\` | ${layout.summary} |`,
    )
    .join('\n');

const planModule = ({
  name,
  boundedContext,
  directory,
  blocks,
  goVersion,
  serviceName,
  grpcPort,
}: {
  name: string;
  boundedContext: string;
  directory: string;
  blocks: readonly Block[];
  goVersion: string;
  serviceName: string;
  grpcPort: number;
}): Omit<Plan, 'goWork'> => {
  const moduleDirectory = `${directory}/${name}`;
  const modulePath = `${MODULE_PREFIX}/${moduleDirectory}`;
  const contractDirectory = `${moduleDirectory}/${CONTRACT_DIRECTORY}`;
  const contractModulePath = `${modulePath}/${CONTRACT_DIRECTORY}`;
  const contractProjectName = `${name}-${CONTRACT_DIRECTORY}`;
  const depth = moduleDirectory.split('/').length;
  const layouts = blocks.map(layoutOf);
  const layer = highestLayer(layouts);
  const integration = layouts.some((layout) => layout.integration);
  const versions = readVersions();

  return {
    directory: moduleDirectory,
    contractDirectory,
    useEntries: [useEntryOf(moduleDirectory), useEntryOf(contractDirectory)],
    substitutions: {
      tmpl: '',
      name,
      grpcPort: String(grpcPort),
      contractDirectory,
      contractModulePath,
      contractProjectName,
      contractProjectNameJson: JSON.stringify(contractProjectName),
      contractPackageNameJson: JSON.stringify(`${NPM_SCOPE}/${contractProjectName}`),
      contractSourceRootJson: JSON.stringify(contractDirectory),
      contractNxSchemaJson: JSON.stringify(
        `${'../'.repeat(depth + 1)}node_modules/nx/schemas/project-schema.json`,
      ),
      contractUnitIdJson: JSON.stringify(`${boundedContext}/${CONTRACT_BLOCK}`),
      contractModulePathJson: JSON.stringify(contractModulePath),
      contractServicePackageJson: JSON.stringify(
        `${contractModulePath}/gen/go/${serviceName.split('.').slice(0, -1).join('/')}`,
      ),
      boundedContextJson: JSON.stringify(boundedContext),
      serveApiCommandJson: JSON.stringify(serveCommandOf(name, 'api')),
      serveRelayCommandJson: JSON.stringify(serveCommandOf(name, 'relay')),
      dockerRunRelayCommandJson: JSON.stringify(
        dockerRunCommandOf(name, imageRefOf(moduleDirectory), 'relay'),
      ),
      testDistributedCommandJson: JSON.stringify(
        'go test -race -count=1 -p 1 -tags=integration,distributed ./distkit/...',
      ),
      pascalName: pascalOf(name),
      envName: name.replaceAll('-', '_').toUpperCase(),
      serviceName,
      moduleDirectory,
      projectNameJson: JSON.stringify(name),
      packageNameJson: JSON.stringify(`${NPM_SCOPE}/${name}`),
      sourceRootJson: JSON.stringify(moduleDirectory),
      nxSchemaJson: JSON.stringify(
        `${'../'.repeat(depth)}node_modules/nx/schemas/project-schema.json`,
      ),
      modulePath,
      goVersion,
      goImage: versions.go.image,
      protocGenGoVersion: versions.protocGenGo,
      govulncheckVersion: versions.govulncheck,
      boundedContext,
      layer,
      layerTagJson: JSON.stringify(`layer:${layer}`),
      blocksTable: blocksTableOf({ layouts, boundedContext }),
      unitsFragment: unitsFragment({
        level: 2,
        // A companion unit shares the block and keeps a membership of its own,
        // so the verifier reads one include per directory.
        units: layouts.flatMap((layout) =>
          [
            { unitSuffix: layout.unitSuffix, dirName: layout.dirName },
            ...(layout.companions ?? []),
          ].map((unit) => ({
            id: `${boundedContext}/${unit.unitSuffix}`,
            block: layout.block,
            boundedContext,
            // O binário pertence à unidade do bloco app: é composition root,
            // não unidade de si mesmo.
            include:
              unit.dirName === 'app'
                ? [`${modulePath}/app`, `${modulePath}/app/rpc`, `${modulePath}/cmd`]
                : [`${modulePath}/${unit.dirName}`],
          })),
        ),
      }),
      externalFragment: externalFragment({ level: 1, external: externalOf(layouts) }),
      hasApp: blocks.includes('app'),
      testRaceCacheJson: integration ? 'false' : 'true',
      testRaceCommandJson: JSON.stringify(testRaceCommandOf(integration)),
    },
    blocks: layouts.map((layout) => ({
      layout,
      directory: `${moduleDirectory}/${layout.dirName}`,
      substitutions: {
        tmpl: '',
        goPackage: layout.dirName,
        block: layout.block,
        blockRole: layout.role,
        boundedContext,
      },
    })),
  };
};

const planGeneration = (tree: Tree, options: BoundedContextGeneratorSchema): Plan => {
  const name = validatedName(options.name);
  const boundedContext = validatedBoundedContext(options.boundedContext);
  const directory = validatedDirectory(options.directory);
  const blocks = validatedBlocks(options.blocks);
  const serviceName = validatedServiceName({ serviceName: options.serviceName, name });
  const grpcPort = validatedGrpcPort(tree, options.grpcPort);

  const goWorkContent =
    tree.read(GO_WORK, 'utf-8') ?? refuse(`${GO_WORK} was not found at the workspace root`);
  const { goVersion, useEntries } = parseGoWork(goWorkContent);

  const module = planModule({
    name,
    boundedContext,
    directory,
    blocks,
    goVersion,
    serviceName,
    grpcPort,
  });

  if (tree.exists(module.directory)) {
    refuse(`${module.directory} already exists: refusing to overwrite a module in place`);
  }
  for (const entry of module.useEntries) {
    if (useEntries.includes(entry)) {
      refuse(`${GO_WORK} already registers ${entry}: refusing to duplicate the entry`);
    }
  }

  return {
    ...module,
    goWork: registerModules({ content: goWorkContent, modules: module.useEntries }),
  };
};

// nx.json is edited as text, one group inserted at the top of release.groups,
// so the rest of the file keeps its formatting; a group already there is left.
const withReleaseGroup = ({
  content,
  name,
  contractDirectory,
}: {
  content: string;
  name: string;
  contractDirectory: string;
}): string => {
  const group = `go-contract-${name}`;
  const parsed = JSON.parse(content) as { release?: { groups?: Record<string, unknown> } };
  if (parsed.release?.groups?.[group] !== undefined) {
    return content;
  }
  const releaseGroup = {
    projects: [`${name}-${CONTRACT_DIRECTORY}`],
    releaseTag: { pattern: `${contractDirectory}/v{version}` },
  };
  const anchor = /^([ \t]*)"groups": \{\n/m.exec(content);
  if (anchor === null) {
    const release = parsed.release ?? {};
    const updated = { ...parsed, release: { ...release, groups: { [group]: releaseGroup } } };
    return `${JSON.stringify(updated, null, 2)}\n`;
  }
  const indent = `${anchor[1]}  `;
  const entry = [
    `${indent}"${group}": {`,
    `${indent}  "projects": ["${name}-${CONTRACT_DIRECTORY}"],`,
    `${indent}  "releaseTag": {`,
    `${indent}    "pattern": "${contractDirectory}/v{version}"`,
    `${indent}  }`,
    `${indent}},`,
    '',
  ].join('\n');
  const at = anchor.index + anchor[0].length;
  return content.slice(0, at) + entry + content.slice(at);
};

const withComposeInclude = ({ content, directory }: { content: string; directory: string }) => {
  const include = `../../${directory}/${DEPLOY_DIRECTORY}/compose.yml`;
  const lines = content.split('\n');
  if (lines.some((line) => line.trim() === `- ${include}`)) {
    return content;
  }
  const last = lines.reduce(
    (found, line, index) =>
      /^\s*- \.\.\/\.\.\/.*\/deploy\/compose\.yml$/.test(line) ? index : found,
    -1,
  );
  const at = last >= 0 ? last + 1 : lines.findIndex((line) => line.trim() === 'include:') + 1;
  if (at <= 0) {
    return refuse(`${LOCAL_COMPOSE} has no include list to add ${include} to`);
  }
  lines.splice(at, 0, `  - ${include}`);
  return lines.join('\n');
};

const runGo = ({ root, args, directory }: { root: string; args: string[]; directory: string }) => {
  try {
    execFileSync('go', args, { cwd: root, stdio: 'inherit' });
  } catch (cause) {
    throw new Error(
      `bounded-context: the module was written to ${directory}, but go ${args[1]} did not run. Fix the cause above, then run: go ${args.join(' ')}`,
      { cause },
    );
  }
};

const templateDir = (name: string): string => templatesDir('bounded-context', name);

export const boundedContextGenerator = async (
  tree: Tree,
  options: BoundedContextGeneratorSchema,
): Promise<GeneratorCallback> => {
  const plan = planGeneration(tree, options);

  generateFiles(tree, templateDir('module'), plan.directory, plan.substitutions);
  generateFiles(tree, templateDir('contract'), plan.contractDirectory, plan.substitutions);
  if (plan.substitutions.hasApp) {
    // The binary, its composition root and its deploy only exist when the
    // context has an app block: there is nothing to serve without one.
    generateFiles(tree, templateDir('app'), plan.directory, plan.substitutions);
    generateFiles(
      tree,
      templateDir('deploy'),
      `${plan.directory}/${DEPLOY_DIRECTORY}`,
      plan.substitutions,
    );
    const compose = tree.read(LOCAL_COMPOSE, 'utf-8');
    if (compose !== null) {
      tree.write(
        LOCAL_COMPOSE,
        withComposeInclude({ content: compose, directory: plan.directory }),
      );
    }
  }
  for (const block of plan.blocks) {
    generateFiles(tree, templateDir('block'), block.directory, block.substitutions);
    if (block.layout.block === 'provider') {
      generateFiles(tree, templateDir('provider'), block.directory, plan.substitutions);
    }
    if (block.layout.block === 'application') {
      generateFiles(tree, templateDir('application'), block.directory, plan.substitutions);
    }
  }

  tree.write(GO_WORK, plan.goWork);
  const nxJson = tree.read(NX_JSON, 'utf-8');
  if (nxJson !== null) {
    tree.write(
      NX_JSON,
      withReleaseGroup({
        content: nxJson,
        name: String(plan.substitutions.name),
        contractDirectory: plan.contractDirectory,
      }),
    );
  }

  const blocks = plan.blocks.map((block) => block.layout.dirName).join(', ');
  logger.info(
    `\nbounded-context: 2 módulos gerados, ${plan.directory} com ${plan.blocks.length} bloco(s) (${blocks}) e ${plan.contractDirectory}.\n${BASELINE_INSTRUCTION}`,
  );

  // WHY: o modsync e o infrasync leem o disco, não a Tree — só o callback
  // pós-flush enxerga os módulos e o deploy novos.
  return () => {
    runGo({ root: tree.root, args: MODSYNC_ARGS, directory: plan.directory });
    if (plan.substitutions.hasApp) {
      runGo({ root: tree.root, args: INFRASYNC_ARGS, directory: plan.directory });
    }
  };
};

export default boundedContextGenerator;
