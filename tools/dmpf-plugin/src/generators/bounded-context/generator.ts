import { join } from 'node:path';
import type { Tree } from '@nx/devkit';
import { generateFiles, logger } from '@nx/devkit';
import type { BlockLayout } from './blocks';
import { BLOCK_NAMES, CONTRACT_BLOCK, isBlock, layoutOf, missingDependencies } from './blocks';
import { parseGoWork, registerModules, useEntryOf } from './go-work';
import { IDENTIFIER_PATTERN, identifiersOf, isIdentifier } from './identifiers';
import { externalFragment, includeFragment } from './manifest';
import type { Block, BoundedContextGeneratorSchema } from './schema';

const MODULE_PREFIX = 'gitea.lidercap.com.br/lidercap-apps/lidercap-platform';
const DEFAULT_DIRECTORY = 'libs/backend/go';
const GO_WORK = 'go.work';
const NPM_SCOPE = '@lidercap-apps';

const BASELINE_INSTRUCTION = [
  'Unidades novas são ato de classificação (AUT-01). Regrave o baseline em commit próprio:',
  '  go run ./libs/backend/go/dmpf-conformance/cmd/dmpf-conformance --root . --write-baseline',
].join('\n');

type Substitutions = Record<string, string | boolean | readonly string[]>;

type PlannedModule = {
  layout: BlockLayout;
  directory: string;
  useEntry: string;
  substitutions: Substitutions;
};

type Plan = {
  directory: string;
  modules: readonly PlannedModule[];
  goWork: string;
};

const refuse = (reason: string): never => {
  throw new Error(`bounded-context: ${reason}`);
};

const validatedName = (name: string | undefined): string => {
  if (name === undefined || name.trim().length === 0) {
    return refuse('option name is required: it prefixes every generated module directory');
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
  return selected;
};

const testRaceCommandOf = (layout: BlockLayout): string =>
  layout.integration
    ? 'go test -race -count=1 -p 1 -tags=integration ./...'
    : 'go test -race ./...';

const planModule = ({
  layout,
  name,
  boundedContext,
  directory,
  goVersion,
}: {
  layout: BlockLayout;
  name: string;
  boundedContext: string;
  directory: string;
  goVersion: string;
}): PlannedModule => {
  const identifiers = identifiersOf(name);
  const moduleDirectory = `${directory}/${name}-${layout.suffix}`;
  const projectName = `${name}-${layout.suffix}-go`;
  const modulePath = `${MODULE_PREFIX}/${moduleDirectory}`;
  const depth = moduleDirectory.split('/').length;
  const dependsOnProjects = layout.dependsOnBlocks.map(
    (block) => `${name}-${layoutOf(block).suffix}-go`,
  );
  return {
    layout,
    directory: moduleDirectory,
    useEntry: useEntryOf(moduleDirectory),
    substitutions: {
      tmpl: '',
      block: layout.block,
      blockJson: JSON.stringify(layout.block),
      blockRole: layout.role,
      blockSummary: layout.summary,
      layer: layout.layer,
      layerTagJson: JSON.stringify(`layer:${layout.layer}`),
      projectName,
      projectNameJson: JSON.stringify(projectName),
      packageNameJson: JSON.stringify(`${NPM_SCOPE}/${projectName}`),
      sourceRootJson: JSON.stringify(moduleDirectory),
      nxSchemaJson: JSON.stringify(
        `${'../'.repeat(depth)}node_modules/nx/schemas/project-schema.json`,
      ),
      modulePath,
      goVersion,
      goPackage: `${identifiers.goIdent}${layout.packageSuffix}`,
      boundedContext,
      boundedContextJson: JSON.stringify(boundedContext),
      unitIdJson: JSON.stringify(`${boundedContext}/${layout.suffix}`),
      unitId: `${boundedContext}/${layout.suffix}`,
      includeFragment: includeFragment({ level: 3, packages: [modulePath] }),
      externalFragment: externalFragment({ level: 1, external: layout.external }),
      testRaceCacheJson: layout.integration ? 'false' : 'true',
      testRaceCommandJson: JSON.stringify(testRaceCommandOf(layout)),
      hasTestRaceDependsOn: dependsOnProjects.length > 0,
      testRaceDependsOnJson: dependsOnProjects.map((project) => JSON.stringify(project)).join(', '),
    },
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

  const modules = blocks.map((block) =>
    planModule({ layout: layoutOf(block), name, boundedContext, directory, goVersion }),
  );

  for (const module of modules) {
    if (tree.exists(module.directory)) {
      refuse(`${module.directory} already exists: refusing to overwrite a module in place`);
    }
    if (useEntries.includes(module.useEntry)) {
      refuse(`${GO_WORK} already registers ${module.useEntry}: refusing to duplicate the entry`);
    }
  }

  return {
    directory,
    modules,
    goWork: registerModules({
      content: goWorkContent,
      modules: modules.map((module) => module.useEntry),
    }),
  };
};

const templateDir = (name: string): string => join(__dirname, 'files', name);

export const boundedContextGenerator = async (
  tree: Tree,
  options: BoundedContextGeneratorSchema,
): Promise<void> => {
  const plan = planGeneration(tree, options);

  for (const module of plan.modules) {
    generateFiles(tree, templateDir('module'), module.directory, module.substitutions);
  }

  tree.write(GO_WORK, plan.goWork);

  const generated =
    plan.modules.length === 1 ? '1 módulo gerado' : `${plan.modules.length} módulos gerados`;
  logger.info(`\nbounded-context: ${generated} em ${plan.directory}.\n${BASELINE_INSTRUCTION}`);
};

export default boundedContextGenerator;
