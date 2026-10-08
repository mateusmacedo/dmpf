#!/usr/bin/env node
// Uso: node tools/dmpf-plugin/scripts/sync-versions.mjs [--root <dir>] [--target <versão>] [--workflow-ref <sha>] [--check]

import { existsSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const VERSIONS = 'tools/dmpf-plugin/versions.json';
const MIGRATIONS = 'tools/dmpf-plugin/migrations.json';
const MODULE_VERSION = /^v?(\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?)$/;
const COMMIT_SHA = /^[0-9a-f]{40}$/;

// O pin do buf não tem fonte fora daqui: o buf.sh do pacote lê o versions.json.
const PINS = [
  { field: 'go.directive', sources: ['go.work'], pattern: /^go (\d+\.\d+(?:\.\d+)?)$/m },
  {
    field: 'go.image',
    sources: ['apps/backend/bookings/Dockerfile'],
    pattern: /^FROM (golang:\S+) AS build$/m,
  },
  {
    field: 'protocGenGo',
    sources: ['libs/backend/go/contracts/buf.gen.yaml'],
    pattern: /google\.golang\.org\/protobuf\/cmd\/protoc-gen-go@(v\S+)/,
  },
  {
    field: 'golangciLint',
    sources: ['nx.json'],
    pattern: /github\.com\/golangci\/golangci-lint\/v2\/cmd\/golangci-lint@(v[^"\s]+)/,
  },
  {
    field: 'nxGo',
    sources: ['pnpm-workspace.yaml'],
    pattern: /^\s+"@nx-go\/nx-go": (\d+\.\d+\.\d+\S*)$/m,
  },
  {
    field: 'govulncheck',
    sources: ['libs/backend/go/domain/project.json'],
    pattern: /golang\.org\/x\/vuln\/cmd\/govulncheck@(v[^"\s]+)/,
  },
];

class UsageError extends Error {}

const parseArgs = (argv) => {
  const out = {
    root: resolve(dirname(fileURLToPath(import.meta.url)), '..', '..', '..'),
    check: false,
  };
  for (let i = 0; i < argv.length; i++) {
    const flag = argv[i];
    const value = () => {
      if (i + 1 >= argv.length) throw new UsageError(`${flag} exige um valor`);
      return argv[++i];
    };
    if (flag === '--root') out.root = resolve(value());
    else if (flag === '--target') out.target = value();
    else if (flag === '--workflow-ref') out.workflowRef = value();
    else if (flag === '--check') out.check = true;
    else throw new UsageError(`flag desconhecida: ${flag}`);
  }
  if (out.check && (out.target !== undefined || out.workflowRef !== undefined)) {
    throw new UsageError('--check não aceita --target nem --workflow-ref');
  }
  if (out.target !== undefined && !MODULE_VERSION.test(out.target)) {
    throw new UsageError(`--target ${JSON.stringify(out.target)} não é X.Y.Z[-pré-release]`);
  }
  if (out.workflowRef !== undefined && !COMMIT_SHA.test(out.workflowRef)) {
    throw new UsageError(
      `--workflow-ref ${JSON.stringify(out.workflowRef)} não é SHA de 40 caracteres`,
    );
  }
  return out;
};

const readPin = (root, pin) => {
  const source = pin.sources.find((path) => existsSync(join(root, path)));
  if (source === undefined)
    throw new Error(`${pin.field}: nenhuma fonte encontrada em ${pin.sources.join(', ')}`);
  const match = pin.pattern.exec(readFileSync(join(root, source), 'utf-8'));
  if (match === null) throw new Error(`${pin.field}: pin não encontrado em ${source}`);
  return match[1];
};

const getField = (doc, field) => field.split('.').reduce((node, key) => node?.[key], doc);

const setField = (doc, field, value) => {
  const keys = field.split('.');
  const last = keys.pop();
  const parent = keys.reduce((node, key) => {
    node[key] ??= {};
    return node[key];
  }, doc);
  parent[last] = value;
};

// Cada migration leva o workspace à saída do init da versão instalada, de qualquer
// versão anterior: uma entrada por migration, na versão do pacote, roda uma vez só.
const migrationsOf = (root) => {
  const path = join(root, MIGRATIONS);
  return existsSync(path) ? JSON.parse(readFileSync(path, 'utf-8')) : null;
};

const divergentMigrations = (migrations, version) =>
  Object.entries(migrations?.generators ?? {}).filter(([, entry]) => entry.version !== version);

const main = (argv) => {
  let args;
  try {
    args = parseArgs(argv);
  } catch (error) {
    console.error(`sync-versions: ${error.message}`);
    return 2;
  }
  const path = join(args.root, VERSIONS);
  try {
    const doc = JSON.parse(readFileSync(path, 'utf-8'));
    const pins = PINS.map((pin) => ({ field: pin.field, value: readPin(args.root, pin) }));

    if (args.check) {
      const divergent = pins.filter(({ field, value }) => getField(doc, field) !== value);
      for (const { field, value } of divergent) {
        console.log(
          `${VERSIONS}: ${field} ${JSON.stringify(getField(doc, field))}, a fonte fixa ${value}`,
        );
      }
      const version = MODULE_VERSION.exec(doc.kernel)?.[1];
      const migrations = divergentMigrations(migrationsOf(args.root), version);
      for (const [name, entry] of migrations) {
        console.log(
          `${MIGRATIONS}: ${name} na versão ${entry.version}, o kernel está em ${version}`,
        );
      }
      const ok = divergent.length === 0 && migrations.length === 0;
      console.log(ok ? 'sync-versions: conforme' : 'sync-versions: REPROVADO');
      return ok ? 0 : 1;
    }

    for (const { field, value } of pins) setField(doc, field, value);
    if (args.target !== undefined) {
      const version = `v${MODULE_VERSION.exec(args.target)[1]}`;
      doc.kernel = version;
      doc.conformance = version;
    }
    if (args.workflowRef !== undefined) doc.workflowRef = args.workflowRef;
    const kernel = MODULE_VERSION.exec(doc.kernel ?? '')?.[1];
    if (kernel === undefined) {
      throw new Error(
        `${VERSIONS}: kernel ${JSON.stringify(doc.kernel)} não é vX.Y.Z[-pré-release]`,
      );
    }
    const migrations = migrationsOf(args.root);
    writeFileSync(path, `${JSON.stringify(doc, null, 2)}\n`);
    console.log(`sync-versions: ${VERSIONS} gravado`);
    if (migrations !== null) {
      for (const entry of Object.values(migrations.generators ?? {})) entry.version = kernel;
      writeFileSync(join(args.root, MIGRATIONS), `${JSON.stringify(migrations, null, 2)}\n`);
      console.log(`sync-versions: ${MIGRATIONS} gravado`);
    }
    return 0;
  } catch (error) {
    console.error(`sync-versions: ${error.message}`);
    return 2;
  }
};

process.exit(main(process.argv.slice(2)));
