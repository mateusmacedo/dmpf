import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { readdirSync, statSync } from 'node:fs';
import { basename, join, relative } from 'node:path';
import type { GeneratorCallback, NxJsonConfiguration, Tree } from '@nx/devkit';
import {
  generateFiles,
  installPackagesTask,
  logger,
  readJson,
  readNxJson,
  updateNxJson,
} from '@nx/devkit';
import {
  DMPF_CONFIG_FILE,
  DMPF_CONFIG_SCHEMA,
  type DmpfConfig,
  parseDmpfConfig,
  RESERVED_MODULE_PREFIX,
  readDmpfConfigFromTree,
} from '../../lib/dmpf-config';
import { parseJsonOf } from '../../lib/json';
import { templatesDir } from '../../lib/paths';
import {
  type DmpfVersions,
  isOlderRelease,
  readPluginVersion,
  readVersions,
} from '../../lib/versions';
import type { InitGeneratorSchema } from './schema';

const KERNEL_PREFIX = 'github.com/mateusmacedo/dmpf';
const GO_WORK = 'go.work';
const NX_GO = '@nx-go/nx-go';
const RENDER_ROOT = '.dmpf-init-render';
export const RENDERED_FILE = 'dmpf.rendered.json';
const RENDERED_SCHEMA = 'dmpf/rendered@1';
type Source = {
  dir: readonly string[];
  kind: 'managed' | 'seed';
  mode?: DmpfConfig['tooling']['mode'];
  needsWorkflowRef?: boolean;
};
const SOURCES: readonly Source[] = [
  { dir: ['init', 'managed'], kind: 'managed' },
  { dir: ['init', 'seed'], kind: 'seed' },
  { dir: ['init', 'version'], kind: 'managed', mode: 'version', needsWorkflowRef: true },
  { dir: ['ai'], kind: 'managed' },
];
const PLUGIN_IN_NODE_MODULES = 'node_modules/@mateusmacedo/dmpf-plugin';

type Kind = Source['kind'];
type Rendered = Map<string, { content: string; kind: Kind }>;
type Hashes = Record<string, string>;

export type TemplatePlan = {
  rendered: Rendered;
  writes: [string, string][];
  edited: string[];
  hashes: Hashes;
};

const trimDashes = (value: string): string => {
  let start = 0;
  let end = value.length;
  while (start < end && value[start] === '-') {
    start += 1;
  }
  while (end > start && value[end - 1] === '-') {
    end -= 1;
  }
  return value.slice(start, end);
};

const slugOf = (value: string): string =>
  trimDashes(value.toLowerCase().replace(/[^a-z0-9_-]+/g, '-')) || 'workspace';

const ownerOf = (modulePrefix: string): string | undefined => {
  const [host, owner] = modulePrefix.split('/');
  return host === 'github.com' && owner ? owner.toLowerCase() : undefined;
};

const derivedOf = (modulePrefix: string): Pick<DmpfConfig, 'imageRegistry' | 'bufModule'> => {
  const owner = ownerOf(modulePrefix);
  return {
    imageRegistry: owner ? `ghcr.io/${owner}` : 'registry.example.com/change-me',
    bufModule: owner ? `buf.build/${owner}` : 'buf.build/change-me',
  };
};

const defaultsOf = (tree: Tree, modulePrefix: string): DmpfConfig => {
  const name = tree.exists('package.json') ? readJson(tree, 'package.json').name : undefined;
  const scoped = typeof name === 'string' && name.startsWith('@');
  return {
    modulePrefix,
    npmScope: scoped ? name.split('/')[0] : `@${slugOf(basename(tree.root))}`,
    appsDir: 'apps/backend',
    edge: '',
    spiffeTrustDomain: 'dmpf',
    composeProfile: 'dmpf',
    composeProject: slugOf(
      typeof name === 'string' ? (name.split('/').pop() ?? '') : basename(tree.root),
    ),
    ...derivedOf(modulePrefix),
    tooling: { mode: 'version' },
  };
};

const definedOptions = (options: InitGeneratorSchema): Partial<DmpfConfig> => {
  const { force: _force, ...values } = options;
  return Object.fromEntries(
    Object.entries(values).filter(([, value]) => value !== undefined && value !== ''),
  );
};

// A registry derived from the reserved prefix follows the real one: keeping it would
// carry example values into the first real configuration.
const resolveConfig = (
  tree: Tree,
  options: InitGeneratorSchema,
  previous: DmpfConfig | null,
): DmpfConfig => {
  const modulePrefix = options.modulePrefix || previous?.modulePrefix || RESERVED_MODULE_PREFIX;
  const kept: Partial<DmpfConfig> = { ...previous };
  if (previous) {
    const before = derivedOf(previous.modulePrefix);
    for (const key of ['imageRegistry', 'bufModule'] as const) {
      if (previous[key] === before[key]) {
        delete kept[key];
      }
    }
  }
  const merged = {
    ...defaultsOf(tree, modulePrefix),
    ...kept,
    ...definedOptions(options),
    modulePrefix,
  };
  return parseDmpfConfig({ schema: DMPF_CONFIG_SCHEMA, ...merged }, 'init options');
};

const templateFiles = (dir: string): string[] =>
  readdirSync(dir).flatMap((entry) => {
    const path = join(dir, entry);
    return statSync(path).isDirectory() ? templateFiles(path) : [path];
  });

export const renderedPaths = (): string[] =>
  SOURCES.flatMap(({ dir }) =>
    templateFiles(templatesDir(...dir)).map((path) =>
      relative(templatesDir(...dir), path).replace(/__tmpl__$/, ''),
    ),
  );

const treeFiles = (tree: Tree, dir: string): string[] =>
  tree.children(dir).flatMap((child) => {
    const path = `${dir}/${child}`;
    return tree.isFile(path) ? [path] : treeFiles(tree, path);
  });

// In local mode the AI assets cite the platform tree; in version mode, the same
// files at the commit the plugin was released from.
const substitutionsOf = (config: DmpfConfig, versions: DmpfVersions): Record<string, unknown> => {
  const local = config.tooling.mode === 'local';
  const pluginDir = local ? 'tools/dmpf-plugin' : PLUGIN_IN_NODE_MODULES;
  const contextCheckScript = `${pluginDir}/scripts/dmpf-context-check.sh`;
  const workflowRef = versions.workflowRef || 'master';
  return {
    tmpl: '',
    ...config,
    localTooling: local,
    tool: (name: string) =>
      local
        ? `go run ./tools/dmpf-conformance/cmd/${name}`
        : `go run ${KERNEL_PREFIX}/tools/dmpf-conformance/cmd/${name}@${versions.conformance}`,
    contextCheckScript,
    contextCheck: local
      ? `bash ${contextCheckScript}`
      : `DMPF_APPS_DIR=${config.appsDir} bash ${contextCheckScript}`,
    src: (path: string) =>
      local ? path : `https://github.com/mateusmacedo/dmpf/blob/${workflowRef}/${path}`,
    workflowRef,
    pluginVersion: readPluginVersion(),
  };
};

const render = (tree: Tree, config: DmpfConfig, versions: DmpfVersions): Rendered => {
  const rendered: Rendered = new Map();
  for (const [index, { dir, kind, mode, needsWorkflowRef }] of SOURCES.entries()) {
    if (mode && mode !== config.tooling.mode) {
      continue;
    }
    // Without the SHA the caller would run the reusable workflow from a moving branch.
    if (needsWorkflowRef && !versions.workflowRef) {
      logger.warn(
        'dmpf-plugin: this build has no workflowRef, so .github/workflows/dmpf-ci.yml was not written; a released plugin always has one.',
      );
      continue;
    }
    const scratch = `${RENDER_ROOT}/${index}`;
    generateFiles(tree, templatesDir(...dir), scratch, substitutionsOf(config, versions));
    for (const path of treeFiles(tree, scratch)) {
      rendered.set(path.slice(scratch.length + 1), {
        content: tree.read(path, 'utf-8') ?? '',
        kind,
      });
      tree.delete(path);
    }
  }
  tree.delete(RENDER_ROOT);
  return rendered;
};

const hashOf = (content: string): string => createHash('sha256').update(content).digest('hex');

const readHashes = (tree: Tree): Hashes => {
  const raw = tree.read(RENDERED_FILE, 'utf-8');
  if (raw === null) {
    return {};
  }
  const parsed = parseJsonOf<{ schema?: unknown; files?: Hashes }>(raw, RENDERED_FILE);
  if (parsed?.schema !== RENDERED_SCHEMA || typeof parsed.files !== 'object' || !parsed.files) {
    throw new Error(`${RENDERED_FILE}: expected schema ${RENDERED_SCHEMA} with a files object`);
  }
  return parsed.files;
};

const hashesContent = (hashes: Hashes): string => {
  const files = Object.fromEntries(Object.entries(hashes).sort(([a], [b]) => (a < b ? -1 : 1)));
  return `${JSON.stringify({ schema: RENDERED_SCHEMA, files }, null, 2)}\n`;
};

// A managed file is ours while its hash matches the one recorded when it was
// written: that holds across plugin versions, whose templates differ.
export const planTemplates = (
  tree: Tree,
  config: DmpfConfig,
  versions: DmpfVersions,
  force = false,
): TemplatePlan => {
  const hashes = readHashes(tree);
  const rendered = render(tree, config, versions);
  const writes: [string, string][] = [];
  const edited: string[] = [];
  for (const [path, { content, kind }] of rendered) {
    const current = tree.read(path, 'utf-8');
    if (current === null) {
      writes.push([path, content]);
    } else if (kind === 'managed' && current !== content) {
      if (force || hashes[path] === hashOf(current)) {
        writes.push([path, content]);
      } else {
        edited.push(path);
      }
    }
  }
  return { rendered, writes, edited, hashes };
};

export const applyTemplates = (tree: Tree, { rendered, writes, hashes }: TemplatePlan): void => {
  for (const [path, content] of writes) {
    tree.write(path, content);
  }
  const next: Hashes = {};
  for (const [path, { content, kind }] of rendered) {
    if (kind !== 'managed') {
      continue;
    }
    if (tree.read(path, 'utf-8') === content) {
      next[path] = hashOf(content);
    } else if (hashes[path]) {
      next[path] = hashes[path];
    }
  }
  const content = hashesContent(next);
  if (tree.read(RENDERED_FILE, 'utf-8') !== content) {
    tree.write(RENDERED_FILE, content);
  }
};

const configContent = (config: DmpfConfig): string =>
  `${JSON.stringify({ schema: DMPF_CONFIG_SCHEMA, ...config }, null, 2)}\n`;

const goTargetDefaults = (
  versions: DmpfVersions,
): NonNullable<NxJsonConfiguration['targetDefaults']> => ({
  'deploy-env': {
    executor: 'nx:run-commands',
    cache: false,
    options: {
      command: '[ -f deploy/.env ] || cp deploy/.env.example deploy/.env',
      cwd: '{projectRoot}',
    },
  },
  [`${NX_GO}:generate`]: { options: { args: ['./...'] } },
  [`${NX_GO}:lint`]: {
    inputs: ['go', '^go', '{workspaceRoot}/.golangci.yml'],
    options: {
      linter: 'go',
      args: [
        'run',
        `github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${versions.golangciLint}`,
        'run',
        '--allow-parallel-runners',
      ],
    },
  },
  [`${NX_GO}:test`]: {
    dependsOn: ['^build'],
    cache: true,
    inputs: ['go', '^go'],
    options: { cover: true },
  },
});

const GO_NAMED_INPUT = [
  '{projectRoot}/**/*',
  '!{projectRoot}/deploy/**/*',
  '{workspaceRoot}/go.work',
  '{workspaceRoot}/go.work.sum',
];

// Only what is missing is added: rewriting nx.json would also reformat it.
export const mergeNxJson = (tree: Tree, versions: DmpfVersions): void => {
  const nxJson = readNxJson(tree) ?? {};
  let changed = false;
  if (!nxJson.namedInputs?.go) {
    nxJson.namedInputs = { ...nxJson.namedInputs, go: GO_NAMED_INPUT };
    changed = true;
  }
  const targetDefaults = { ...nxJson.targetDefaults };
  for (const [name, value] of Object.entries(goTargetDefaults(versions))) {
    if (!(name in targetDefaults)) {
      targetDefaults[name] = value;
      changed = true;
    }
  }
  nxJson.targetDefaults = targetDefaults;
  const plugins = nxJson.plugins ?? [];
  if (!plugins.some((plugin) => (typeof plugin === 'string' ? plugin : plugin.plugin) === NX_GO)) {
    nxJson.plugins = [...plugins, NX_GO];
    changed = true;
  }
  if (changed) {
    updateNxJson(tree, nxJson);
  }
};

// Only raises: a workspace already on a newer Go keeps it, or its go.mod files
// would ask for more than go.work declares.
export const pinGoWork = (tree: Tree, versions: DmpfVersions, { create = true } = {}): void => {
  const directive = `go ${versions.go.directive}`;
  const current = tree.read(GO_WORK, 'utf-8');
  if (current === null) {
    if (create) {
      tree.write(GO_WORK, `${directive}\n`);
    }
    return;
  }
  const declared = /^go (\S+)$/m.exec(current)?.[1];
  if (declared === undefined || !isOlderRelease(declared, versions.go.directive)) {
    return;
  }
  tree.write(GO_WORK, current.replace(/^go \S+$/m, directive));
};

const addNxGo = (tree: Tree, versions: DmpfVersions): boolean => {
  const manifest = readJson(tree, 'package.json');
  if (manifest.dependencies?.[NX_GO] || manifest.devDependencies?.[NX_GO]) {
    return false;
  }
  manifest.devDependencies = { ...manifest.devDependencies, [NX_GO]: versions.nxGo };
  tree.write('package.json', `${JSON.stringify(manifest, null, 2)}\n`);
  return true;
};

const infrasyncArgs = (config: DmpfConfig, versions: DmpfVersions): string[] => [
  'run',
  config.tooling.mode === 'local'
    ? './tools/dmpf-conformance/cmd/infrasync'
    : `${KERNEL_PREFIX}/tools/dmpf-conformance/cmd/infrasync@${versions.conformance}`,
  '--root',
  '.',
  '--write',
];

export const initGenerator = async (
  tree: Tree,
  options: InitGeneratorSchema,
): Promise<GeneratorCallback> => {
  const previous = tree.exists(DMPF_CONFIG_FILE) ? readDmpfConfigFromTree(tree) : null;
  const config = resolveConfig(tree, options, previous);
  const versions = readVersions();
  const plan = planTemplates(tree, config, versions, options.force);
  if (plan.edited.length > 0) {
    throw new Error(
      `init: these files were generated by init and edited since: ${plan.edited.join(', ')}. Re-run with --force to overwrite them.`,
    );
  }

  applyTemplates(tree, plan);
  if (tree.read(DMPF_CONFIG_FILE, 'utf-8') !== configContent(config)) {
    tree.write(DMPF_CONFIG_FILE, configContent(config));
  }
  pinGoWork(tree, versions);
  mergeNxJson(tree, versions);
  const installs = addNxGo(tree, versions);

  return () => {
    if (installs) {
      installPackagesTask(tree);
    }
    try {
      execFileSync('go', infrasyncArgs(config, versions), { cwd: tree.root, stdio: 'inherit' });
    } catch (cause) {
      throw new Error(
        `init: the workspace was written, but infrasync did not run. Fix the cause above, then run: go ${infrasyncArgs(config, versions).join(' ')}`,
        { cause },
      );
    }
  };
};

export default initGenerator;
