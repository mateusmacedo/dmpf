#!/usr/bin/env node
// comment-discipline-ok: rationale do dual-fix (specifiers + overrides pnpm 11) precisa caber no header do script CLI
// Normaliza dist do `nx prune` p/ `pnpm install --prod --frozen-lockfile` no Docker:
// reconstrói deps do importer raiz e remove `overrides:` (pnpm 11 só lê via workspace.yaml).
// Uso: node tools/normalize-pruned-manifest.mjs <distDir>

import { readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

const distDir = process.argv[2];
if (!distDir) {
  console.error('uso: normalize-pruned-manifest.mjs <distDir>');
  process.exit(1);
}

const lockPath = join(distDir, 'pnpm-lock.yaml');
const pkgPath = join(distDir, 'package.json');

const lock = readFileSync(lockPath, 'utf8');
const pkg = JSON.parse(readFileSync(pkgPath, 'utf8'));

/** Remove the top-level `overrides:` block from a pnpm-lock.yaml (v9). */
const stripLockfileOverrides = (lockText) => {
  const lines = lockText.split('\n');
  const out = [];
  let stripped = false;
  for (let i = 0; i < lines.length; ) {
    if (lines[i] === 'overrides:') {
      stripped = true;
      i += 1;
      while (i < lines.length && (lines[i].startsWith(' ') || lines[i] === '')) {
        if (lines[i] === '') {
          let j = i + 1;
          while (j < lines.length && lines[j] === '') {
            j += 1;
          }
          if (j >= lines.length || !lines[j].startsWith(' ')) {
            break;
          }
        }
        i += 1;
      }
      while (i < lines.length && lines[i] === '') {
        i += 1;
      }
      continue;
    }
    out.push(lines[i]);
    i += 1;
  }
  return { text: out.join('\n'), stripped };
};

// Reconstrói dependencies/devDependencies/optionalDependencies do manifest a partir do
// importer raiz (`.`) do lockfile — nome+specifier+seção. Ignora sub-importers de
// workspace_modules/* (cujos peers sobrescreveriam o specifier concreto do app) e
// elimina divergências de specifier (catalog:/workspace:) e de placement dep/devDep.
const SECTIONS = { dependencies: {}, devDependencies: {}, optionalDependencies: {} };
let inImporters = false;
let inRoot = false;
let section = null;
let pendingName = null;
for (const line of lock.split('\n')) {
  if (line === 'importers:') {
    inImporters = true;
    continue;
  }
  if (!inImporters) continue;
  if (/^\S/.test(line)) break; // saiu da seção importers
  if (/^ {2}\S/.test(line)) {
    inRoot = line.trim() === '.:';
    section = null;
    continue;
  }
  if (!inRoot) continue;
  const head = line.match(/^ {4}(\w+):\s*$/);
  if (head) {
    section = head[1] in SECTIONS ? head[1] : null;
    continue;
  }
  const nameLine = line.match(/^ {6}('?[^\n:]+?'?):\s*$/);
  if (nameLine && section) {
    pendingName = nameLine[1].replace(/^'|'$/g, '');
    continue;
  }
  const specLine = line.match(/^ {8}specifier: (.+)$/);
  if (specLine && section && pendingName) {
    SECTIONS[section][pendingName] = specLine[1].trim().replace(/^'|'$/g, '');
    pendingName = null;
  }
}

for (const [sectionName, deps] of Object.entries(SECTIONS)) {
  if (Object.keys(deps).length > 0) pkg[sectionName] = deps;
  else delete pkg[sectionName];
}

const { text: normalizedLock, stripped } = stripLockfileOverrides(lock);

writeFileSync(pkgPath, `${JSON.stringify(pkg, null, 2)}\n`);
if (stripped) {
  writeFileSync(lockPath, normalizedLock.endsWith('\n') ? normalizedLock : `${normalizedLock}\n`);
}

const total = Object.values(SECTIONS).reduce((n, d) => n + Object.keys(d).length, 0);
console.log(`normalize-pruned-manifest: ${total} dep(s) reconstruída(s) do lockfile em ${pkgPath}`);
if (stripped) {
  console.log(`normalize-pruned-manifest: overrides removidos de ${lockPath}`);
}
