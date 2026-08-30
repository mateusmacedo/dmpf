#!/usr/bin/env node
// ephemeral-refs-ok-file: cita a spec do próprio contrato que este script materializa
// Verificador de congruência do acervo de ADRs.
// Contrato de escrita: docs/adr/README.md (guia + índice) e a seção «Formato do ADR»
// de docs/specs/SPEC-DBTRMM3X-dmpf-adrs-minimos.md.
// Uso: node tools/adr-verify.mjs [--root <dir>] [--check a1,a4] [--from N] [--to N] [--json]

import { existsSync, readdirSync, readFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

export const ACERVO_DIR = 'docs/adr';
export const SPECS_DIR = 'docs/specs';
export const INDEX_FILE = 'README.md';
export const INDEX_SECTION = 'Decisões registradas';

/** Nome de arquivo do ADR: número de três dígitos e slug em kebab-case minúsculo. */
export const RE_ADR_FILE = /^(\d{3})-([a-z0-9]+(?:-[a-z0-9]+)*)\.md$/;
export const RE_H1 = /^#\s+ADR-(\d+):\s*(\S.*?)\s*$/;
export const RE_INDEX_ROW = /^\|\s*ADR-(\d+)\s*\|(.*)$/;

export const SECTION_STATUS = 'Status';
export const SECTION_CONTEXTO = 'Contexto';
export const SECTION_DECISAO = 'Decisão';
export const SECTION_ALTERNATIVAS = 'Alternativas descartadas';
export const SECTION_CONSEQUENCIAS = 'Consequências';
export const SECTION_REFERENCIAS = 'Referências';

/** Ordem canônica das seções do template. `Status` e `Referências` são opcionais. */
export const CANONICAL_SECTIONS = [
  SECTION_STATUS,
  SECTION_CONTEXTO,
  SECTION_DECISAO,
  SECTION_ALTERNATIVAS,
  SECTION_CONSEQUENCIAS,
  SECTION_REFERENCIAS,
];
export const REQUIRED_SECTIONS = [
  SECTION_CONTEXTO,
  SECTION_DECISAO,
  SECTION_ALTERNATIVAS,
  SECTION_CONSEQUENCIAS,
];

export const ALTERNATIVES_HEADER = '| Alternativa | Por que foi rejeitada |';
export const RE_ALTERNATIVES_HEADER = /^\|\s*Alternativa\s*\|\s*Por que foi rejeitada\s*\|\s*$/;
export const RE_TABLE_SEPARATOR = /^\|[\s:|-]+\|\s*$/;
export const LABEL_POSITIVAS = '**Positivas:**';
export const LABEL_NEGATIVAS = '**Negativas:**';
export const COST_LABEL = '**Custo aceito:**';
// O custo pode ser item próprio ao fim das negativas ou vir inline no bullet da
// negativa que ele precifica (ADR-028 casa três custos 1:1 com três negativas).
// O que o gate fixa é presença e localização; a diagramação é livre.
export const RE_COST_LABEL = /\*\*Custo aceito:\*\*/;
export const RE_STANDALONE_LABEL = /^\s*\*\*[^*]+:\*\*\s*$/;
export const RE_PENDING_HEADING = /^###\s+Pendência declarada\b/;
export const RE_PENDING_OWNER = /^\s*-\s+\*\*[^*]*\bowner\b[^*]*:\*\*\s*(\S.*)?$/i;
export const RE_PENDING_DEADLINE = /^\s*-\s+\*\*Prazo:\*\*\s*(\S.*)?$/;
export const RE_STATUS_PENDING = /\bpendente\b/i;
export const RE_ADDENDUM = /^Addendum\b/;
export const RE_SPEC_ID = /\bSPEC-[A-Z0-9]{8}\b/g;

/** Referências físicas: as três formas que o acervo DMPF já reconhece. */
export const RE_LINE_COMPLETE = /\b((?:[\w.-]+\/)*[\w.-]+\.md):(\d+)(?:-(\d+))?/g;
export const RE_LINE_ABBREV = /\b(SPEC-[A-Z0-9]{8}):(\d+)(?:-(\d+))?/g;
export const RE_LINE_SHORTHAND = /(?<![\w./-]):(\d+)(?:-(\d+))?\b/g;

const USAGE = `uso: node tools/adr-verify.mjs [--root <dir>] [--check a1,..,a8] [--from N] [--to N] [--json]

  --root <dir>   raiz do repositório a verificar (default: a raiz deste script)
  --check <l>    executa apenas as checagens listadas
  --from <N>     ignora ADRs com número menor que N
  --to <N>       ignora ADRs com número maior que N
  --json         emite o relatório como JSON
`;

const parseArgs = (argv) => {
  const out = { root: null, checks: null, from: null, to: null, json: false };
  const number = (raw, flag) => {
    const n = Number(raw);
    if (!Number.isInteger(n) || n < 0) throw new Error(`${flag} exige um inteiro: ${raw}`);
    return n;
  };
  let i = 0;
  while (i < argv.length) {
    const a = argv[i];
    i += 1;
    if (a === '--root') {
      out.root = argv[i];
      i += 1;
    } else if (a === '--check') {
      out.checks = (argv[i] ?? '').split(',').map((s) => s.trim().toLowerCase());
      i += 1;
    } else if (a === '--from') {
      out.from = number(argv[i], '--from');
      i += 1;
    } else if (a === '--to') {
      out.to = number(argv[i], '--to');
      i += 1;
    } else if (a === '--json') out.json = true;
    else if (a === '--help' || a === '-h') out.help = true;
    else throw new Error(`argumento desconhecido: ${a}`);
  }
  return out;
};

/** Violação: o que falhou, onde, e por quê. */
const violation = (check, file, line, target, reason) => ({ check, file, line, target, reason });

export const headingsOfLevel = (lines, level) => {
  const marker = '#'.repeat(level);
  const re = new RegExp(`^${marker}\\s+(\\S.*?)\\s*$`);
  const out = [];
  lines.forEach((l, i) => {
    if (new RegExp(`^${marker}#`).test(l)) return;
    const m = re.exec(l);
    if (m) out.push({ name: m[1], line: i + 1 });
  });
  return out;
};

/**
 * Intervalo `[início, fim)` em base 1 do corpo de uma seção: da linha seguinte ao
 * heading até o heading seguinte de nível igual ou menor.
 */
export const sectionBody = (lines, name, level = 2) => {
  const re = /^(#{1,6})\s+(\S.*?)\s*$/;
  let start = -1;
  let depth = 0;
  for (let i = 0; i < lines.length; i += 1) {
    const m = re.exec(lines[i]);
    if (!m) continue;
    if (start === -1) {
      if (m[1].length === level && m[2] === name) {
        start = i + 2;
        depth = level;
      }
    } else if (m[1].length <= depth) return [start, i + 1];
  }
  return start === -1 ? null : [start, lines.length + 1];
};

/** Mesmo intervalo em base 1 de `sectionBody`, a partir de um heading já localizado. */
export const blockFrom = (lines, headingLine, level) => {
  const re = /^(#{1,6})\s+\S/;
  for (let i = headingLine; i < lines.length; i += 1) {
    const m = re.exec(lines[i]);
    if (m && m[1].length <= level) return [headingLine + 1, i + 1];
  }
  return [headingLine + 1, lines.length + 1];
};

const sliceRange = (lines, range) =>
  range === null ? [] : lines.slice(range[0] - 1, range[1] - 1);

/**
 * Intervalo `[início, fim)` em offsets 0-based do bloco de um rótulo destacado
 * (`**Negativas:**`), do próprio rótulo até o rótulo destacado seguinte.
 */
export const labelBlock = (bodyLines, label) => {
  const start = bodyLines.findIndex((l) => RE_STANDALONE_LABEL.test(l) && l.includes(label));
  if (start === -1) return null;
  for (let i = start + 1; i < bodyLines.length; i += 1)
    if (RE_STANDALONE_LABEL.test(bodyLines[i])) return [start, i];
  return [start, bodyLines.length];
};

export const parseIndexRows = (lines, range) => {
  const out = [];
  if (range === null) return out;
  for (let i = range[0] - 1; i < range[1] - 1 && i < lines.length; i += 1) {
    const m = RE_INDEX_ROW.exec(lines[i]);
    if (!m) continue;
    const cells = m[2].split('|').map((c) => c.trim());
    if (cells.length && cells[cells.length - 1] === '') cells.pop();
    out.push({ number: m[1], title: cells[0] ?? '', decides: cells[1] ?? '', line: i + 1 });
  }
  return out;
};

export const extractLineRefs = (lines) => {
  const out = [];
  lines.forEach((l, i) => {
    for (const m of l.matchAll(RE_LINE_COMPLETE)) out.push({ raw: m[0], line: i + 1 });
    for (const m of l.matchAll(RE_LINE_ABBREV)) out.push({ raw: m[0], line: i + 1 });
    // As formas nomeadas terminam em `:N`; sem mascará-las, o shorthand as recontaria.
    const masked = l
      .replace(RE_LINE_COMPLETE, (s) => ' '.repeat(s.length))
      .replace(RE_LINE_ABBREV, (s) => ' '.repeat(s.length));
    for (const m of masked.matchAll(RE_LINE_SHORTHAND)) out.push({ raw: m[0], line: i + 1 });
  });
  return out;
};

const a1 = (ctx) => {
  const out = [];
  for (const stray of ctx.strayFiles)
    out.push(
      violation(
        'A1',
        stray,
        null,
        stray,
        'arquivo em docs/adr/ fora do padrão `NNN-slug-em-kebab-case.md` e que não é o índice',
      ),
    );
  for (const adr of ctx.adrs) {
    const first = adr.lines.findIndex((l) => l.trim() !== '');
    if (first === -1) {
      out.push(violation('A1', adr.file, null, adr.number, 'arquivo vazio'));
      continue;
    }
    const m = RE_H1.exec(adr.lines[first]);
    if (!m) {
      out.push(
        violation(
          'A1',
          adr.file,
          first + 1,
          adr.number,
          'a primeira linha com conteúdo não é o H1 `# ADR-NNN: <título>` (o formato é sem frontmatter)',
        ),
      );
      continue;
    }
    if (m[1] !== adr.number)
      out.push(
        violation(
          'A1',
          adr.file,
          first + 1,
          adr.number,
          `o H1 numera ADR-${m[1]} e o arquivo numera ${adr.number}`,
        ),
      );
    const extras = adr.lines
      .map((l, i) => ({ m: RE_H1.exec(l), line: i + 1 }))
      .filter((x) => x.m && x.line !== first + 1);
    for (const e of extras)
      out.push(violation('A1', adr.file, e.line, adr.number, 'segundo H1 de ADR no mesmo arquivo'));
  }
  return out;
};

const a2 = (ctx) => {
  const out = [];
  if (ctx.indexLines === null)
    return [violation('A2', INDEX_FILE, null, null, 'índice `docs/adr/README.md` ausente')];
  if (ctx.indexRange === null)
    return [violation('A2', INDEX_FILE, null, null, `seção «${INDEX_SECTION}» ausente do índice`)];
  const byNumber = new Map();
  for (const row of ctx.indexRows) {
    if (byNumber.has(row.number)) {
      out.push(
        violation(
          'A2',
          INDEX_FILE,
          row.line,
          `ADR-${row.number}`,
          `entrada duplicada no índice (a primeira está na linha ${byNumber.get(row.number).line})`,
        ),
      );
      continue;
    }
    byNumber.set(row.number, row);
  }
  for (const adr of ctx.adrs) {
    const row = byNumber.get(adr.number);
    if (!row) {
      out.push(violation('A2', INDEX_FILE, null, `ADR-${adr.number}`, `ADR sem entrada no índice`));
      continue;
    }
    if (adr.title !== null && row.title !== adr.title)
      out.push(
        violation(
          'A2',
          INDEX_FILE,
          row.line,
          `ADR-${adr.number}`,
          `o índice titula ${JSON.stringify(row.title)} e o H1 titula ${JSON.stringify(adr.title)}`,
        ),
      );
    if (row.decides === '')
      out.push(
        violation(
          'A2',
          INDEX_FILE,
          row.line,
          `ADR-${adr.number}`,
          'entrada do índice sem a coluna «O que decide»',
        ),
      );
  }
  const onDisk = new Set(ctx.adrs.map((a) => a.number));
  for (const row of byNumber.values())
    if (!onDisk.has(row.number) && ctx.inScope(Number(row.number)))
      out.push(
        violation(
          'A2',
          INDEX_FILE,
          row.line,
          `ADR-${row.number}`,
          'entrada do índice sem arquivo correspondente em docs/adr/',
        ),
      );
  return out;
};

const a3 = (ctx) => {
  const out = [];
  const numbers = ctx.adrs.map((a) => Number(a.number)).sort((x, y) => x - y);
  if (numbers.length === 0) return out;
  const seen = new Map();
  for (const adr of ctx.adrs) {
    if (seen.has(adr.number))
      out.push(
        violation(
          'A3',
          adr.file,
          null,
          adr.number,
          `número repetido — já usado por ${seen.get(adr.number)}`,
        ),
      );
    else seen.set(adr.number, adr.file);
  }
  const first = ctx.from ?? 1;
  const last = numbers[numbers.length - 1];
  const present = new Set(numbers);
  for (let n = first; n <= last; n += 1)
    if (!present.has(n))
      out.push(
        violation(
          'A3',
          ACERVO_DIR,
          null,
          String(n).padStart(3, '0'),
          `buraco na sequência — não há ADR com este número entre ${String(first).padStart(3, '0')} e ${String(last).padStart(3, '0')}`,
        ),
      );
  return out;
};

const a4 = (ctx) => {
  const out = [];
  for (const adr of ctx.adrs) {
    const headings = headingsOfLevel(adr.lines, 2);
    const counts = new Map();
    for (const h of headings) counts.set(h.name, (counts.get(h.name) ?? 0) + 1);
    for (const name of REQUIRED_SECTIONS) {
      const n = counts.get(name) ?? 0;
      if (n === 0) out.push(violation('A4', adr.file, null, name, 'seção obrigatória ausente'));
      else if (n > 1)
        out.push(violation('A4', adr.file, null, name, `seção obrigatória repetida ${n} vezes`));
    }
    if (headings.length === 0) continue;
    const status = headings.find((h) => h.name === SECTION_STATUS);
    if (status && status.line !== headings[0].line)
      out.push(
        violation(
          'A4',
          adr.file,
          status.line,
          SECTION_STATUS,
          `quando presente, «${SECTION_STATUS}» abre o arquivo — aqui «${headings[0].name}» vem antes`,
        ),
      );
    const addendum = headings.filter((h) => RE_ADDENDUM.test(h.name));
    const lastHeading = headings[headings.length - 1];
    for (const ad of addendum)
      if (ad.line !== lastHeading.line)
        out.push(
          violation(
            'A4',
            adr.file,
            ad.line,
            ad.name,
            'o addendum é acrescentado ao final, depois de todas as seções',
          ),
        );
    const canonical = headings.filter((h) => CANONICAL_SECTIONS.includes(h.name));
    let highest = null;
    for (const h of canonical) {
      const rank = CANONICAL_SECTIONS.indexOf(h.name);
      if (highest !== null && rank < CANONICAL_SECTIONS.indexOf(highest.name))
        out.push(
          violation(
            'A4',
            adr.file,
            h.line,
            h.name,
            `aparece depois de «${highest.name}», fora da ordem canônica (${CANONICAL_SECTIONS.join(' → ')})`,
          ),
        );
      else highest = h;
    }
  }
  return out;
};

const a5 = (ctx) => {
  const out = [];
  for (const adr of ctx.adrs) {
    const alternatives = sectionBody(adr.lines, SECTION_ALTERNATIVAS);
    if (alternatives !== null) {
      const body = sliceRange(adr.lines, alternatives);
      const headerAt = body.findIndex((l) => RE_ALTERNATIVES_HEADER.test(l));
      if (headerAt === -1)
        out.push(
          violation(
            'A5',
            adr.file,
            alternatives[0],
            SECTION_ALTERNATIVAS,
            `sem o cabeçalho literal \`${ALTERNATIVES_HEADER}\``,
          ),
        );
      else {
        const rest = body.slice(headerAt + 1);
        const separator = rest.findIndex((l) => RE_TABLE_SEPARATOR.test(l));
        const rows =
          separator === -1 ? [] : rest.slice(separator + 1).filter((l) => l.trim().startsWith('|'));
        if (separator === -1)
          out.push(
            violation(
              'A5',
              adr.file,
              alternatives[0] + headerAt,
              SECTION_ALTERNATIVAS,
              'cabeçalho de tabela sem a linha separadora',
            ),
          );
        else if (rows.length === 0)
          out.push(
            violation(
              'A5',
              adr.file,
              alternatives[0] + headerAt,
              SECTION_ALTERNATIVAS,
              'tabela de alternativas sem nenhuma linha de alternativa',
            ),
          );
      }
    }
    const consequences = sectionBody(adr.lines, SECTION_CONSEQUENCIAS);
    if (consequences === null) continue;
    const body = sliceRange(adr.lines, consequences);
    for (const label of [LABEL_POSITIVAS, LABEL_NEGATIVAS])
      if (!body.some((l) => l.includes(label)))
        out.push(
          violation(
            'A5',
            adr.file,
            consequences[0],
            label,
            `rótulo ausente de «${SECTION_CONSEQUENCIAS}»`,
          ),
        );
    const negatives = labelBlock(body, LABEL_NEGATIVAS);
    const costLines = adr.lines
      .map((l, i) => ({ hit: RE_COST_LABEL.test(l), line: i + 1 }))
      .filter((x) => x.hit)
      .map((x) => x.line);
    if (negatives === null) continue;
    const inNegatives = (line) =>
      line >= consequences[0] + negatives[0] && line < consequences[0] + negatives[1];
    if (!costLines.some(inNegatives))
      out.push(
        violation(
          'A5',
          adr.file,
          consequences[0] + negatives[0],
          COST_LABEL,
          `rótulo do custo aceito ausente do bloco ${LABEL_NEGATIVAS} de «${SECTION_CONSEQUENCIAS}»`,
        ),
      );
    for (const line of costLines.filter((l) => !inNegatives(l)))
      out.push(
        violation(
          'A5',
          adr.file,
          line,
          COST_LABEL,
          `rótulo do custo aceito fora do bloco ${LABEL_NEGATIVAS} de «${SECTION_CONSEQUENCIAS}»`,
        ),
      );
  }
  return out;
};

const a6 = (ctx) => {
  const out = [];
  for (const adr of ctx.adrs) {
    const blocks = adr.lines
      .map((l, i) => ({ ok: RE_PENDING_HEADING.test(l), line: i + 1, name: l.trim() }))
      .filter((x) => x.ok);
    const status = sectionBody(adr.lines, SECTION_STATUS);
    const statusDeclaresPending =
      status !== null && sliceRange(adr.lines, status).some((l) => RE_STATUS_PENDING.test(l));
    if (statusDeclaresPending && blocks.length === 0)
      out.push(
        violation(
          'A6',
          adr.file,
          status[0],
          SECTION_STATUS,
          'o status declara pendência sem o bloco `### Pendência declarada` que a registra',
        ),
      );
    for (const b of blocks) {
      const body = sliceRange(adr.lines, blockFrom(adr.lines, b.line, 3));
      const owner = body.find((l) => RE_PENDING_OWNER.test(l));
      const deadline = body.find((l) => RE_PENDING_DEADLINE.test(l));
      if (!owner)
        out.push(
          violation('A6', adr.file, b.line, b.name, 'pendência declarada sem rubrica de owner'),
        );
      else if (!RE_PENDING_OWNER.exec(owner)[1])
        out.push(violation('A6', adr.file, b.line, b.name, 'rubrica de owner declarada e vazia'));
      if (!deadline)
        out.push(
          violation(
            'A6',
            adr.file,
            b.line,
            b.name,
            'pendência declarada sem a rubrica `- **Prazo:**`',
          ),
        );
      else if (!RE_PENDING_DEADLINE.exec(deadline)[1])
        out.push(violation('A6', adr.file, b.line, b.name, 'rubrica de prazo declarada e vazia'));
    }
  }
  return out;
};

const a7 = (ctx) => {
  const out = [];
  for (const adr of [...ctx.adrs, ...ctx.indexAsArtifact]) {
    for (const ref of extractLineRefs(adr.lines))
      out.push(
        violation(
          'A7',
          adr.file,
          ref.line,
          ref.raw,
          'referência à fonte por número de linha — o contrato é citar por seção (`§N`)',
        ),
      );
  }
  return out;
};

const a8 = (ctx) => {
  const out = [];
  for (const adr of ctx.adrs) {
    const status = sectionBody(adr.lines, SECTION_STATUS);
    if (status === null) continue;
    const body = sliceRange(adr.lines, status);
    body.forEach((l, i) => {
      for (const m of l.matchAll(RE_SPEC_ID))
        if (!ctx.specs.has(m[0]))
          out.push(
            violation(
              'A8',
              adr.file,
              status[0] + i,
              m[0],
              `spec de origem sem arquivo em ${SPECS_DIR}/`,
            ),
          );
    });
  }
  return out;
};

const CHECKS = { a1, a2, a3, a4, a5, a6, a7, a8 };

export const loadContext = (root, { from = null, to = null } = {}) => {
  const acervo = join(root, ACERVO_DIR);
  if (!existsSync(acervo)) throw new Error(`acervo ausente: ${ACERVO_DIR} (root: ${root})`);
  const inScope = (n) => (from === null || n >= from) && (to === null || n <= to);

  const entries = readdirSync(acervo)
    .filter((x) => x.endsWith('.md'))
    .sort();
  const adrs = [];
  const strayFiles = [];
  for (const file of entries) {
    if (file === INDEX_FILE) continue;
    const m = RE_ADR_FILE.exec(file);
    if (!m) {
      strayFiles.push(file);
      continue;
    }
    if (!inScope(Number(m[1]))) continue;
    const lines = readFileSync(join(acervo, file), 'utf8').split('\n');
    const h1 = lines.map((l) => RE_H1.exec(l)).find(Boolean);
    adrs.push({ file, number: m[1], slug: m[2], lines, title: h1 ? h1[2] : null });
  }

  const indexPath = join(acervo, INDEX_FILE);
  const indexLines = existsSync(indexPath) ? readFileSync(indexPath, 'utf8').split('\n') : null;
  const indexRange = indexLines === null ? null : sectionBody(indexLines, INDEX_SECTION);
  const indexRows = indexLines === null ? [] : parseIndexRows(indexLines, indexRange);

  const specsDir = join(root, SPECS_DIR);
  const specs = new Set();
  if (existsSync(specsDir))
    for (const f of readdirSync(specsDir)) {
      const m = /^(SPEC-[A-Z0-9]{8})/.exec(f);
      if (m) specs.add(m[1]);
    }

  return {
    root,
    from,
    to,
    inScope,
    adrs,
    strayFiles,
    indexLines,
    indexRange,
    indexRows,
    indexAsArtifact:
      indexLines === null ? [] : [{ file: INDEX_FILE, lines: indexLines, number: null }],
    specs,
  };
};

const main = () => {
  let args;
  try {
    args = parseArgs(process.argv.slice(2));
  } catch (e) {
    process.stderr.write(`${e.message}\n\n${USAGE}`);
    return 2;
  }
  if (args.help) {
    process.stdout.write(USAGE);
    return 0;
  }
  const root = resolve(args.root ?? resolve(dirname(fileURLToPath(import.meta.url)), '..'));
  const selected = args.checks ?? Object.keys(CHECKS);
  for (const c of selected)
    if (!(c in CHECKS)) {
      process.stderr.write(`checagem desconhecida: ${c}\n`);
      return 2;
    }

  let ctx;
  try {
    ctx = loadContext(root, { from: args.from, to: args.to });
  } catch (e) {
    process.stderr.write(`erro ao carregar o contexto: ${e.message}\n`);
    return 2;
  }

  const results = [];
  for (const name of selected) {
    const violations = CHECKS[name](ctx);
    results.push({ check: name.toUpperCase(), ok: violations.length === 0, violations });
  }
  const total = results.reduce((n, r) => n + r.violations.length, 0);

  if (args.json) {
    process.stdout.write(`${JSON.stringify({ root, results, total }, null, 2)}\n`);
    return total === 0 ? 0 : 1;
  }
  for (const r of results) {
    process.stdout.write(
      `${r.ok ? 'ok  ' : 'FALHA'} ${r.check}${r.ok ? '' : `  (${r.violations.length})`}\n`,
    );
    for (const v of r.violations)
      process.stdout.write(
        `      ${v.file}${v.line ? `:${v.line}` : ''}  ${v.target ?? ''}  ${v.reason}\n`,
      );
  }
  process.stdout.write(`\nADRs no escopo: ${ctx.adrs.length}\n`);
  process.stdout.write(total === 0 ? '\nsem violações\n' : `\n${total} violação(ões)\n`);
  return total === 0 ? 0 : 1;
};

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exit(main());
}
