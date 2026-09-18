import { execFileSync } from 'node:child_process';
import { join } from 'node:path';
import type { GeneratorCallback, Tree } from '@nx/devkit';
import { generateFiles, logger } from '@nx/devkit';
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

const BASELINE_INSTRUCTION = [
  'Unidades novas são ato de classificação (AUT-01). Regrave o baseline em commit próprio:',
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
  useEntry: string;
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
  return value;
};

const validatedBlocks = (blocks: readonly string[] | undefined): Block[] => {
  const requested = blocks === undefined ? [...BLOCK_NAMES] : [...new Set(blocks)];
  if (requested.includes(CONTRACT_BLOCK)) {
    return refuse(
      `block "${CONTRACT_BLOCK}" is not generated here: the contract source lives in contracts/ and is published by the Buf rite`,
    );
  }
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

const testRaceCommandOf = (integration: boolean): string =>
  integration ? 'go test -race -count=1 -p 1 -tags=integration ./...' : 'go test -race ./...';

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
}: {
  name: string;
  boundedContext: string;
  directory: string;
  blocks: readonly Block[];
  goVersion: string;
}): Omit<Plan, 'goWork'> => {
  const moduleDirectory = `${directory}/${name}`;
  const modulePath = `${MODULE_PREFIX}/${moduleDirectory}`;
  const depth = moduleDirectory.split('/').length;
  const layouts = blocks.map(layoutOf);
  const layer = highestLayer(layouts);
  const integration = layouts.some((layout) => layout.integration);
  const dependsOnProjects = [...new Set(layouts.flatMap((layout) => layout.dependsOnProjects))];

  return {
    directory: moduleDirectory,
    useEntry: useEntryOf(moduleDirectory),
    substitutions: {
      tmpl: '',
      name,
      projectNameJson: JSON.stringify(name),
      packageNameJson: JSON.stringify(`${NPM_SCOPE}/${name}`),
      sourceRootJson: JSON.stringify(moduleDirectory),
      nxSchemaJson: JSON.stringify(
        `${'../'.repeat(depth)}node_modules/nx/schemas/project-schema.json`,
      ),
      modulePath,
      goVersion,
      boundedContext,
      layer,
      layerTagJson: JSON.stringify(`layer:${layer}`),
      blocksTable: blocksTableOf({ layouts, boundedContext }),
      unitsFragment: unitsFragment({
        level: 2,
        units: layouts.map((layout) => ({
          id: `${boundedContext}/${layout.unitSuffix}`,
          block: layout.block,
          boundedContext,
          include: [`${modulePath}/${layout.dirName}`],
        })),
      }),
      externalFragment: externalFragment({ level: 1, external: externalOf(layouts) }),
      testRaceCacheJson: integration ? 'false' : 'true',
      testRaceCommandJson: JSON.stringify(testRaceCommandOf(integration)),
      hasTestRaceDependsOn: dependsOnProjects.length > 0,
      testRaceDependsOnJson: dependsOnProjects.map((project) => JSON.stringify(project)).join(', '),
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

  const goWorkContent =
    tree.read(GO_WORK, 'utf-8') ?? refuse(`${GO_WORK} was not found at the workspace root`);
  const { goVersion, useEntries } = parseGoWork(goWorkContent);

  const module = planModule({ name, boundedContext, directory, blocks, goVersion });

  if (tree.exists(module.directory)) {
    refuse(`${module.directory} already exists: refusing to overwrite a module in place`);
  }
  if (useEntries.includes(module.useEntry)) {
    refuse(`${GO_WORK} already registers ${module.useEntry}: refusing to duplicate the entry`);
  }

  return {
    ...module,
    goWork: registerModules({ content: goWorkContent, modules: [module.useEntry] }),
  };
};

const templateDir = (name: string): string => join(__dirname, 'files', name);

export const boundedContextGenerator = async (
  tree: Tree,
  options: BoundedContextGeneratorSchema,
): Promise<GeneratorCallback> => {
  const plan = planGeneration(tree, options);

  generateFiles(tree, templateDir('module'), plan.directory, plan.substitutions);
  for (const block of plan.blocks) {
    generateFiles(tree, templateDir('block'), block.directory, block.substitutions);
  }

  tree.write(GO_WORK, plan.goWork);

  const blocks = plan.blocks.map((block) => block.layout.dirName).join(', ');
  logger.info(
    `\nbounded-context: 1 módulo gerado em ${plan.directory} com ${plan.blocks.length} bloco(s): ${blocks}.\n${BASELINE_INSTRUCTION}`,
  );

  // WHY: o modsync lê `go.mod` e `go.work` do disco, não da Tree — só o
  // callback pós-flush enxerga o módulo novo.
  return () => {
    try {
      execFileSync('go', MODSYNC_ARGS, { cwd: tree.root, stdio: 'inherit' });
    } catch (cause) {
      throw new Error(
        `bounded-context: the module was written to ${plan.directory}, but dmpf-modsync did not run. Fix the cause above, then run: go ${MODSYNC_ARGS.join(' ')}`,
        { cause },
      );
    }
  };
};

export default boundedContextGenerator;
