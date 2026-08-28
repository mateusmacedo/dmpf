#!/usr/bin/env node
// ephemeral-refs-ok-file: cita a spec do contrato cujos manifestos este script emite
// Emissor dos manifestos de tools/dmpf-verify.mjs.
// A semente (mapa de documentos, prefixos, seletores de total, projeções do README)
// é o contrato de RF5; o resto é derivado do acervo pelo MESMO extrator que o
// verificador usa — emissor e verificador não podem divergir.
// Uso: node tools/dmpf-verify-emit.mjs <profiles|counts|line-refs> [--root <dir>] [--base <rev>]

import { execFileSync } from 'node:child_process';
import { existsSync, readdirSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import {
  ACERVO_DIR,
  extractDefinitions,
  extractPhysicalRefs,
  MANIFEST_DIR,
} from './dmpf-verify.mjs';

const SEED = {
  documents: {
    'FND-03': {
      file: 'upr-decision-mensagens.md',
      prefixes: ['DEC', 'CTR', 'FRT', 'ESC', 'UPR-I', 'UPR-L', 'MSG-N'],
      declaredTotal: 54,
    },
    'FND-04': {
      file: 'uow-inbox-outbox.md',
      prefixes: ['UOW', 'OBX', 'INB', 'GAR', 'BLK'],
      declaredTotal: 64,
    },
    'FND-05': {
      file: 'cloudevents-protobuf-buf.md',
      prefixes: ['ENV', 'PTB', 'BUF', 'REP', 'INT'],
      declaredTotal: 64,
    },
    'FND-06': {
      file: 'politicas-transporte.md',
      prefixes: ['TRP', 'RST', 'GRP', 'KFK', 'SQS', 'ASY', 'COE'],
      declaredTotal: 131,
    },
    'FND-07': {
      file: 'contexto-erros-seguranca.md',
      prefixes: ['CTX', 'IDN', 'ERR', 'MAP', 'THR', 'DAT'],
      declaredTotal: 112,
    },
    'FND-08': {
      file: 'resiliencia-observabilidade.md',
      prefixes: ['RES', 'MET', 'TRC', 'LOG', 'RUN'],
      declaredTotal: 121,
    },
    'FND-09': {
      file: 'testes-interop.md',
      prefixes: ['PIR', 'CEN', 'FIX', 'ORA', 'KIT', 'FIT', 'RAS'],
      declaredTotal: 142,
    },
    'FND-10': {
      file: 'governanca-bom-pilotos.md',
      prefixes: ['GOV', 'AUT', 'BOM', 'ADO', 'PIL', 'RDY'],
      declaredTotal: 78,
    },
  },
  // Citáveis por seção, sem regras próprias indexadas: entram no mapa de C3, não no de definição.
  extraDocuments: { 'FND-01': 'inventario-as-is.md', RFC: 'rfc-dmpf-foundation-v0.1.md' },
  ignoredDocuments: ['FND-02', 'FND-11', 'Parte-1'],
  highlightedForm: {
    document: 'FND-09',
    file: 'testes-interop.md',
    section: '11',
    encaminhado: ['RAS-41'],
    from: 30,
    to: 41,
    prefix: 'RAS',
  },
  reservedRanges: [
    { prefix: 'ORA', from: 14, to: 29 },
    { prefix: 'RAS', from: 17, to: 29 },
    { prefix: 'TRP', from: 45, to: 45 },
    { prefix: 'GOV', from: 27, to: 29 },
  ],
  totalSelectors: {
    'upr-decision-mensagens.md': { line: 1394, pattern: 'São 54 regras com ID estável' },
    'uow-inbox-outbox.md': { line: 2346, pattern: '`registro` — \\*\\*64 regras com ID\\*\\*' },
    'cloudevents-protobuf-buf.md': {
      line: 1921,
      pattern: 'As 64 regras têm por sujeito o contrato',
      atypical:
        'o FND-05 não declara total em bloco próprio; a única afirmação da contagem está na célula M1 da tabela de monotonicidade',
    },
    'politicas-transporte.md': { line: 1830, pattern: '`registro` — 131 regras, em sete prefixos' },
    'contexto-erros-seguranca.md': {
      line: 1796,
      pattern: '\\*\\*Total: 112 regras `normativo`\\.\\*\\*',
    },
    'resiliencia-observabilidade.md': {
      line: 1919,
      pattern: '`registro` — 121 regras `normativo`\\.',
    },
    'testes-interop.md': {
      line: 2666,
      pattern:
        '\\*\\*Total: 142 regras próprias: 141 `normativo` e uma `encaminhado`, `RAS-41`\\.\\*\\*',
    },
    'governanca-bom-pilotos.md': {
      line: 1758,
      pattern:
        '`registro` — \\*\\*78 regras com identificador\\*\\*: 77 `normativo` e uma `encaminhado`',
    },
  },
  readmeProjections: [
    {
      id: 'P1-fontes',
      readmePattern: 'O artefato herda de \\*\\*(\\w+)\\*\\* fontes',
      source: 'testes-interop.md',
      sourcePattern: 'este artefato depende de (\\w+) fontes',
    },
    {
      id: 'P2-obrigacoes',
      readmePattern: 'a matriz de §3 trata\\s+\\*\\*(\\d+) obrigações atômicas\\*\\*',
      source: 'testes-interop.md',
      sourcePattern: 'As (\\d+) obrigações estão fechadas',
    },
    {
      id: 'P3-identificadores',
      readmePattern: 'o acervo enumera\\s+(\\d+) identificadores',
      source: 'testes-interop.md',
      sourcePattern: 'endereçáveis por \\*\\*(\\d+) identificadores\\*\\*',
    },
    {
      id: 'P4-estaveis',
      readmePattern: '\\*\\*(\\d+) são estáveis\\*\\*',
      source: 'testes-interop.md',
      sourcePattern: 'declara ali a sua própria contagem: (\\d+)',
    },
    {
      id: 'P5-clausulas',
      readmePattern: 'A distinção entre (\\d+) cláusulas `normativo`',
      source: 'testes-interop.md',
      sourcePattern: 'trazem \\*\\*(\\d+)\\s+cláusulas `normativo`\\*\\*',
    },
    {
      id: 'P6-fnd08',
      readmePattern: 'As (\\d+) regras `normativo` — incluídos os sufixos',
      source: 'resiliencia-observabilidade.md',
      sourcePattern: '`registro` — (\\d+) regras `normativo`\\.',
    },
  ],
  // Afirmações que a sincronização de RF1 removeu: se voltarem, o README envelheceu de novo.
  readmeForbidden: [
    {
      id: 'F1-seis-fontes',
      pattern: 'herda de \\*\\*seis\\*\\* fontes',
      why: 'correção 1 de RF1 — são sete fontes',
    },
    { id: 'F2-551', pattern: '551 identificadores', why: 'correção 3 de RF1 — são 672' },
    { id: 'F3-535', pattern: '535 cláusulas', why: 'correção 4 de RF1 — são 682' },
    { id: 'F4-118', pattern: 'As 118 regras `normativo`', why: 'correção 5 de RF1 — são 121' },
    {
      id: 'F5-inexistentes',
      pattern: 'encaminhados a FND-09 e ainda inexistentes',
      why: 'correção 6 de RF1 — a pendência foi quitada (REC-001)',
    },
    {
      id: 'F6-mesmo-byte',
      pattern: 'cuja spec hoje\\s+exige «mesmo byte»',
      why: 'correção 7 de RF1 — reconciliado antes da redação (REC-004)',
    },
  ],
};

const parse = (argv) => {
  const what = argv[0];
  const out = { what, root: null, base: 'f46ced2' };
  let i = 1;
  while (i < argv.length) {
    const a = argv[i];
    i += 1;
    if (a === '--root') {
      out.root = argv[i];
      i += 1;
    } else if (a === '--base') {
      out.base = argv[i];
      i += 1;
    } else throw new Error(`argumento desconhecido: ${a}`);
  }
  if (!['profiles', 'counts', 'line-refs'].includes(what))
    throw new Error(
      'uso: dmpf-verify-emit.mjs <profiles|counts|line-refs> [--root <dir>] [--base <rev>]',
    );
  return out;
};

const readAcervo = (root) => {
  const dir = join(root, ACERVO_DIR);
  const files = {};
  for (const f of readdirSync(dir)
    .filter((x) => x.endsWith('.md'))
    .sort())
    files[f] = readFileSync(join(dir, f), 'utf8').split('\n');
  return files;
};

const buildHighlighted = () => {
  const hl = SEED.highlightedForm;
  const forces = {};
  for (let n = hl.from; n <= hl.to; n += 1) {
    const id = `${hl.prefix}-${String(n).padStart(2, '0')}`;
    forces[id] = hl.encaminhado.includes(id) ? 'encaminhado' : 'normativo';
  }
  return { document: hl.document, file: hl.file, section: hl.section, forces };
};

const emitProfiles = (root) => {
  const files = readAcervo(root);
  const highlighted = buildHighlighted();
  const documents = {};
  for (const [doc, seed] of Object.entries(SEED.documents)) {
    // Um cabeçalho de tabela entra na lista de definição quando traz ID que a prosa
    // e a forma destacada não cobrem, na ordem em que aparece no documento. O mesmo
    // ID reaparece depois nas tabelas de índice, de vetores e de rastreabilidade.
    const probe = extractDefinitions(files[seed.file], seed, highlighted, { allHeaders: true });
    const covered = new Set(probe.found.keys());
    const definitionTableHeaders = [];
    const seenHeader = new Set();
    for (const row of probe.rows) {
      if (seenHeader.has(row.header)) {
        if (definitionTableHeaders.includes(row.header)) covered.add(row.id);
        continue;
      }
      seenHeader.add(row.header);
      const novos = probe.rows.filter((r) => r.header === row.header && !covered.has(r.id));
      if (novos.length > 0) {
        definitionTableHeaders.push(row.header);
        for (const r of novos) covered.add(r.id);
      }
    }
    const { found } = extractDefinitions(
      files[seed.file],
      { ...seed, definitionTableHeaders },
      highlighted,
    );
    const ids = {};
    for (const id of [...found.keys()].sort()) ids[id] = found.get(id).line;
    documents[doc] = {
      file: seed.file,
      prefixes: seed.prefixes,
      definitionTableHeaders,
      forces: ['normativo', 'encaminhado'],
      forceExceptions: doc === 'FND-09' ? { 'RAS-41': 'encaminhado' } : {},
      declaredTotal: seed.declaredTotal,
      ids,
    };
    if (Object.keys(ids).length !== seed.declaredTotal) {
      process.stderr.write(
        `aviso: ${doc} extraiu ${Object.keys(ids).length} definições, a semente declara ${seed.declaredTotal}\n`,
      );
    }
  }
  return {
    documents,
    extraDocuments: SEED.extraDocuments,
    ignoredDocuments: SEED.ignoredDocuments,
    highlightedForm: highlighted,
    reservedRanges: SEED.reservedRanges,
  };
};

const emitCounts = () => ({
  totals: Object.fromEntries(
    Object.entries(SEED.totalSelectors).map(([f, s]) => [
      f,
      {
        line: s.line,
        pattern: s.pattern,
        value:
          SEED.documents[Object.keys(SEED.documents).find((d) => SEED.documents[d].file === f)]
            .declaredTotal,
        ...(s.atypical ? { atypical: s.atypical } : {}),
      },
    ]),
  ),
  readmeProjections: SEED.readmeProjections,
  readmeForbidden: SEED.readmeForbidden,
  navigationMap: {
    file: 'navegacao.md',
    prefixRow: '^\\| `([A-Z]+(?:-[A-Z])?)` \\| (\\d+) \\|',
    documentRow: '^\\| \\[[^\\]]+\\]\\([^)]+\\) \\((FND-\\d{2})\\) \\| (\\d+) \\|',
  },
});

const resolveTarget = (root, target) => {
  for (const c of [target, join(ACERVO_DIR, target)]) if (existsSync(join(root, c))) return c;
  for (const d of ['docs/specs', 'docs/adr']) {
    const dir = join(root, d);
    if (!existsSync(dir)) continue;
    for (const f of readdirSync(dir))
      if (f === target || f === `${target}.md` || f.startsWith(`${target}-`)) return `${d}/${f}`;
  }
  return null;
};

const emitLineRefs = (root, base) => {
  const files = readAcervo(root);
  const references = [];
  const problems = [];
  for (const [src, lines] of Object.entries(files)) {
    for (const r of extractPhysicalRefs(lines)) {
      const resolved = resolveTarget(root, r.target);
      if (!resolved) {
        problems.push(`${src}:${r.line} ${r.raw} -> alvo \`${r.target}\` não resolve`);
        continue;
      }
      const targetLines = readFileSync(join(root, resolved), 'utf8').split('\n');
      if (r.to > targetLines.length) {
        problems.push(`${src}:${r.line} ${r.raw} -> além do EOF de ${resolved}`);
        continue;
      }
      let baseLines = null;
      try {
        baseLines = execFileSync('git', ['-C', root, 'show', `${base}:${resolved}`], {
          encoding: 'utf8',
          stdio: ['ignore', 'pipe', 'ignore'],
        }).split('\n');
      } catch {
        problems.push(
          `${src}:${r.line} ${r.raw} -> ${resolved} não existe em ${base}; o manifesto não pode nascer sem preimage`,
        );
        continue;
      }
      for (let n = r.from; n <= r.to; n += 1) {
        if ((targetLines[n - 1] ?? null) !== (baseLines[n - 1] ?? null))
          problems.push(`${src}:${r.line} ${r.raw} -> ${resolved}:${n} DIVERGE de ${base}`);
      }
      references.push({
        source: src,
        sourceLine: r.line,
        raw: r.raw,
        form: r.form,
        target: r.target,
        resolved,
        from: r.from,
        to: r.to,
        excerpt: (targetLines[r.from - 1] ?? '').trim().slice(0, 60),
      });
    }
  }
  if (problems.length) {
    process.stderr.write(`o manifesto NÃO foi emitido — ${problems.length} problema(s):\n`);
    for (const p of problems) process.stderr.write(`  ${p}\n`);
    process.stderr.write(
      '\nemitir sobre um alvo divergente abençoaria o deslocamento em vez de acusá-lo.\n',
    );
    return null;
  }
  return { base, count: references.length, references };
};

const main = () => {
  let args;
  try {
    args = parse(process.argv.slice(2));
  } catch (e) {
    process.stderr.write(`${e.message}\n`);
    return 2;
  }
  const root = resolve(args.root ?? resolve(dirname(fileURLToPath(import.meta.url)), '..'));
  const name = { profiles: 'profiles.json', counts: 'counts.json', 'line-refs': 'line-refs.json' }[
    args.what
  ];
  const built =
    args.what === 'profiles'
      ? emitProfiles(root)
      : args.what === 'counts'
        ? emitCounts()
        : emitLineRefs(root, args.base);
  if (built === null) return 1;
  const dest = join(root, MANIFEST_DIR, name);
  writeFileSync(dest, `${JSON.stringify(built, null, 2)}\n`, 'utf8');
  // O manifesto é versionado e revisado no PR, então precisa sair no formato do
  // projeto — senão o `biome ci` reprova o que a própria ferramenta gerou.
  try {
    execFileSync('pnpm', ['biome', 'format', '--write', dest], { cwd: root, stdio: 'ignore' });
  } catch {
    process.stderr.write(
      'aviso: não foi possível formatar o manifesto com o Biome; rode `pnpm biome format --write` antes do commit\n',
    );
  }
  process.stdout.write(`${join(MANIFEST_DIR, name)} emitido\n`);
  return 0;
};

process.exit(main());
