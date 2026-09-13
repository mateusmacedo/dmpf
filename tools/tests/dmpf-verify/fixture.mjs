// Constrói um acervo sintético mínimo que passa nas sete checagens, em diretório
// temporário. Cada teste negativo aplica UMA mutação sobre esta base — duas
// violações no mesmo negativo tornariam o diagnóstico ambíguo.

import { mkdirSync, mkdtempSync, readFileSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';

const FND04 = `# UoW mini — DMPF FND-04

## §1. Fronteira

\`normativo\` \`UOW-01\` — Uma UoW tem exatamente uma transação local.

\`normativo\` \`UOW-02\` — Uma UoW não abrange dois recursos transacionais.

## §2. Índice

\`registro\` — **2 regras com ID**.
`;

const FND09 = `# Testes mini — DMPF FND-09

## §1. Fronteira

Este artefato herda de FND-04 §1 e da RFC §1.

## §11. A verificação

### §11.1 Modos

\`normativo\` — **\`RAS-30\`. A calibração é a chave de leitura.**

\`encaminhado\` — **\`RAS-41\`. A localização fica com FND-06.**

## §13. Índice

A âncora está em \`uow-inbox-outbox.md:5\`, e o par em :7.

**Total: 2 regras próprias.**
`;

const RFC = `# RFC mini

## §1. Escopo

Texto.
`;

const README = `# DMPF mini

O artefato herda de **2** fontes.
`;

export const PROFILES = {
  documents: {
    'FND-04': {
      file: 'uow-inbox-outbox.md',
      prefixes: ['UOW'],
      definitionTableHeaders: [],
      forces: ['normativo', 'encaminhado'],
      forceExceptions: {},
      declaredTotal: 2,
      ids: { 'UOW-01': 5, 'UOW-02': 7 },
    },
    'FND-09': {
      file: 'testes-interop.md',
      prefixes: ['RAS'],
      definitionTableHeaders: [],
      forces: ['normativo', 'encaminhado'],
      forceExceptions: { 'RAS-41': 'encaminhado' },
      declaredTotal: 2,
      ids: { 'RAS-30': 11, 'RAS-41': 13 },
    },
  },
  extraDocuments: { RFC: 'rfc-dmpf-foundation-v0.1.md' },
  ignoredDocuments: ['FND-02', 'FND-11', 'Parte-1'],
  highlightedForm: {
    document: 'FND-09',
    file: 'testes-interop.md',
    section: '11',
    forces: { 'RAS-30': 'normativo', 'RAS-41': 'encaminhado' },
  },
  reservedRanges: [{ prefix: 'RAS', from: 17, to: 29 }],
};

export const COUNTS = {
  totals: {
    'uow-inbox-outbox.md': {
      line: 11,
      pattern: '`registro` — \\*\\*2 regras com ID\\*\\*',
      value: 2,
    },
    'testes-interop.md': { line: 19, pattern: '\\*\\*Total: 2 regras próprias\\.\\*\\*', value: 2 },
  },
  readmeProjections: [
    {
      id: 'P1',
      readmePattern: 'herda de \\*\\*(\\d+)\\*\\* fontes',
      source: 'uow-inbox-outbox.md',
      sourcePattern: '`registro` — \\*\\*(\\d+) regras com ID\\*\\*',
    },
  ],
  readmeForbidden: [
    { id: 'F1', pattern: 'afirmação caduca', why: 'exemplo de negativa de regressão' },
  ],
};

export const LINE_REFS = {
  base: 'fixture',
  count: 2,
  references: [
    {
      source: 'testes-interop.md',
      sourceLine: 17,
      raw: 'uow-inbox-outbox.md:5',
      form: 'completa',
      target: 'uow-inbox-outbox.md',
      resolved: 'docs/dmpf/uow-inbox-outbox.md',
      from: 5,
      to: 5,
      excerpt: '`normativo` `UOW-01` — Uma UoW tem exatamente',
    },
    {
      source: 'testes-interop.md',
      sourceLine: 17,
      raw: ':7',
      form: 'shorthand',
      target: 'uow-inbox-outbox.md',
      resolved: 'docs/dmpf/uow-inbox-outbox.md',
      from: 7,
      to: 7,
      excerpt: '`normativo` `UOW-02` — Uma UoW não abrange',
    },
  ],
};

const FILES = {
  'docs/dmpf/uow-inbox-outbox.md': FND04,
  'docs/dmpf/testes-interop.md': FND09,
  'docs/dmpf/rfc-dmpf-foundation-v0.1.md': RFC,
  'docs/dmpf/README.md': README,
};

/**
 * Monta a base num diretório temporário.
 * `mutate` recebe o mapa de arquivos e os três manifestos, e altera exatamente um ponto.
 */
export const buildFixture = (mutate) => {
  const root = mkdtempSync(join(tmpdir(), 'dmpf-verify-'));
  const files = { ...FILES };
  const manifests = {
    'tools/tests/dmpf-verify/profiles.json': structuredClone(PROFILES),
    'tools/tests/dmpf-verify/counts.json': structuredClone(COUNTS),
    'tools/tests/dmpf-verify/line-refs.json': structuredClone(LINE_REFS),
  };
  if (mutate) mutate(files, manifests);
  for (const [rel, content] of Object.entries(files)) {
    const dest = join(root, rel);
    mkdirSync(dirname(dest), { recursive: true });
    writeFileSync(dest, content, 'utf8');
  }
  for (const [rel, obj] of Object.entries(manifests)) {
    const dest = join(root, rel);
    mkdirSync(dirname(dest), { recursive: true });
    writeFileSync(dest, `${JSON.stringify(obj, null, 2)}\n`, 'utf8');
  }
  return root;
};

/** Lê a base para conferir que uma âncora de mutação existe antes de aplicá-la. */
export const baseFile = (rel) => FILES[rel];

export const readFixtureFile = (root, rel) => readFileSync(join(root, rel), 'utf8');
