import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { packageRoot } from './paths';

export const VERSIONS_SCHEMA = 'dmpf/versions@1';

export type DmpfVersions = {
  kernel: string;
  conformance: string;
  go: { directive: string; image: string };
  buf: string;
  protocGenGo: string;
  golangciLint: string;
  govulncheck: string;
  nxGo: string;
  workflowRef: string;
};

const MODULE_VERSION = /^v\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$/;
const GO_DIRECTIVE = /^\d+\.\d+(\.\d+)?$/;
const IMAGE = /^[a-z0-9][a-z0-9./_-]*:[\w.-]+$/;
const COMMIT_SHA = /^[0-9a-f]{40}$/;
const NPM_VERSION = /^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$/;

const field = (source: string, name: string, value: unknown, pattern: RegExp): string => {
  if (typeof value !== 'string' || !pattern.test(value)) {
    throw new Error(`${source}: ${name} ${JSON.stringify(value)} does not match ${pattern}`);
  }
  return value;
};

export const parseVersions = (raw: unknown, source: string): DmpfVersions => {
  const doc = (raw ?? {}) as Record<string, unknown>;
  if (doc.schema !== VERSIONS_SCHEMA) {
    throw new Error(`${source}: schema ${JSON.stringify(doc.schema)}, expected ${VERSIONS_SCHEMA}`);
  }
  const go = (doc.go ?? {}) as Record<string, unknown>;
  const workflowRef =
    doc.workflowRef === '' ? '' : field(source, 'workflowRef', doc.workflowRef, COMMIT_SHA);
  return {
    kernel: field(source, 'kernel', doc.kernel, MODULE_VERSION),
    conformance: field(source, 'conformance', doc.conformance, MODULE_VERSION),
    go: {
      directive: field(source, 'go.directive', go.directive, GO_DIRECTIVE),
      image: field(source, 'go.image', go.image, IMAGE),
    },
    buf: field(source, 'buf', doc.buf, MODULE_VERSION),
    protocGenGo: field(source, 'protocGenGo', doc.protocGenGo, MODULE_VERSION),
    golangciLint: field(source, 'golangciLint', doc.golangciLint, MODULE_VERSION),
    govulncheck: field(source, 'govulncheck', doc.govulncheck, MODULE_VERSION),
    nxGo: field(source, 'nxGo', doc.nxGo, NPM_VERSION),
    workflowRef,
  };
};

export const readVersions = (path: string = join(packageRoot(), 'versions.json')): DmpfVersions =>
  parseVersions(JSON.parse(readFileSync(path, 'utf-8')), path);

export const readPluginVersion = (): string =>
  JSON.parse(readFileSync(join(packageRoot(), 'package.json'), 'utf-8')).version;
