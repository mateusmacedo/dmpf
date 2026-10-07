import { existsSync, readFileSync } from 'node:fs';
import { join, posix } from 'node:path';
import type { Tree } from '@nx/devkit';

export const DMPF_CONFIG_SCHEMA = 'dmpf/workspace@1';
export const DMPF_CONFIG_FILE = 'dmpf.json';
export const RESERVED_MODULE_PREFIX = 'example.com/change-me';

export type ToolingMode = 'local' | 'version';

export type DmpfConfig = {
  modulePrefix: string;
  npmScope: string;
  appsDir: string;
  edge: string;
  spiffeTrustDomain: string;
  composeProfile: string;
  imageRegistry: string;
  bufModule: string;
  tooling: { mode: ToolingMode };
};

const MODULE_PREFIX = /^[a-z0-9][a-z0-9.-]*(\/[A-Za-z0-9._~-]+)+$/;
const NPM_SCOPE = /^@[a-z0-9][a-z0-9._-]*$/;
const PATH_SEGMENT = /^[A-Za-z0-9._-]+$/;
const DNS_LABEL = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/;
const TRUST_DOMAIN = /^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$/;
const COMPOSE_PROFILE = /^[a-z0-9][a-z0-9_.-]*$/;
const IMAGE_REGISTRY = /^[a-z0-9.-]+(:[0-9]+)?(\/[a-z0-9._-]+)*$/;
const BUF_MODULE = /^[a-z0-9.-]+\/[a-z0-9_-]+$/;
const TOOLING_MODES: readonly ToolingMode[] = ['local', 'version'];

const MISSING = `create it with: pnpm nx g @mateusmacedo/dmpf-plugin:init`;

const invalid = (source: string, field: string, value: unknown): never => {
  throw new Error(`${source}: ${field} ${JSON.stringify(value)} is invalid`);
};

const matching = (source: string, field: string, value: unknown, pattern: RegExp): string =>
  typeof value === 'string' && pattern.test(value) ? value : invalid(source, field, value);

// Same rule as the infrasync Layout: a clean relative path that stays inside the workspace.
const appsDirOf = (source: string, value: unknown): string => {
  const clean =
    typeof value === 'string' &&
    value !== '' &&
    !posix.isAbsolute(value) &&
    posix.normalize(value) === value &&
    value !== '.' &&
    !value.startsWith('..') &&
    value.split('/').every((segment) => PATH_SEGMENT.test(segment));
  return clean ? (value as string) : invalid(source, 'appsDir', value);
};

export const parseDmpfConfig = (raw: unknown, source: string): DmpfConfig => {
  const doc = (raw ?? {}) as Record<string, unknown>;
  if (doc.schema !== DMPF_CONFIG_SCHEMA) {
    throw new Error(
      `${source}: schema ${JSON.stringify(doc.schema)}, expected ${DMPF_CONFIG_SCHEMA}`,
    );
  }
  const tooling = (doc.tooling ?? {}) as Record<string, unknown>;
  const mode = TOOLING_MODES.find((candidate) => candidate === tooling.mode);
  return {
    modulePrefix: matching(source, 'modulePrefix', doc.modulePrefix, MODULE_PREFIX),
    npmScope: matching(source, 'npmScope', doc.npmScope, NPM_SCOPE),
    appsDir: appsDirOf(source, doc.appsDir),
    edge: doc.edge === '' ? '' : matching(source, 'edge', doc.edge, DNS_LABEL),
    spiffeTrustDomain: matching(source, 'spiffeTrustDomain', doc.spiffeTrustDomain, TRUST_DOMAIN),
    composeProfile: matching(source, 'composeProfile', doc.composeProfile, COMPOSE_PROFILE),
    imageRegistry: matching(source, 'imageRegistry', doc.imageRegistry, IMAGE_REGISTRY),
    bufModule: matching(source, 'bufModule', doc.bufModule, BUF_MODULE),
    tooling: { mode: mode ?? invalid(source, 'tooling.mode', tooling.mode) },
  };
};

export const readDmpfConfig = (root: string): DmpfConfig => {
  const path = join(root, DMPF_CONFIG_FILE);
  if (!existsSync(path)) {
    throw new Error(`${path} not found; ${MISSING}`);
  }
  return parseDmpfConfig(JSON.parse(readFileSync(path, 'utf-8')), path);
};

export const readDmpfConfigFromTree = (tree: Tree): DmpfConfig => {
  const content = tree.read(DMPF_CONFIG_FILE, 'utf-8');
  if (content === null) {
    throw new Error(`${DMPF_CONFIG_FILE} not found; ${MISSING}`);
  }
  return parseDmpfConfig(JSON.parse(content), DMPF_CONFIG_FILE);
};
