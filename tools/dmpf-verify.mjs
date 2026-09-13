#!/usr/bin/env node
// ephemeral-refs-ok-file: cita a spec do próprio contrato que este script materializa
// Verificador de congruência horizontal do acervo DMPF.
// Contrato de extração: docs/specs/SPEC-4W1BQK93-dmpf-consolidacao-horizontal.md, §Design.
// Uso: node tools/dmpf-verify.mjs [--root <dir>] [--check c1,c3] [--json]

import { existsSync, readdirSync, readFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

export const MANIFEST_DIR = 'tools/tests/dmpf-verify';
export const ACERVO_DIR = 'docs/dmpf';

const parseArgs = (argv) => {
  const out = { root: null, checks: null, json: false };
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
    } else if (a === '--json') out.json = true;
    else if (a === '--help' || a === '-h') out.help = true;
    else throw new Error(`argumento desconhecido: ${a}`);
  }
  return out;
};

const USAGE = `uso: node tools/dmpf-verify.mjs [--root <dir>] [--check c1,c2a,c2b,c3,c4,c5,c6] [--json]

  --root <dir>   raiz do repositório a verificar (default: a raiz deste script)
  --check <l>    executa apenas as checagens listadas
  --json         emite o relatório como JSON
`;

/** Violação: o que falhou, onde, e por quê. */
const violation = (check, file, line, target, reason) => ({ check, file, line, target, reason });

/** Índice de seções de um arquivo: número da seção -> linha do heading. */
export const indexHeadings = (lines) => {
  const idx = new Map();
  lines.forEach((l, i) => {
    const h = /^#{1,6}\s+(.*)$/.exec(l);
    if (!h) return;
    const m = /^(\d+(?:\.\d+)*)/.exec(h[1].trim().replace(/^§\s*/, ''));
    if (m) idx.set(m[1], i + 1);
  });
  return idx;
};

/** Intervalo de linhas de uma seção: do heading até o próximo de nível igual ou maior. */
export const sectionRange = (lines, section) => {
  let start = -1;
  let depth = 0;
  for (let i = 0; i < lines.length; i += 1) {
    const h = /^(#{1,6})\s+(.*)$/.exec(lines[i]);
    if (!h) continue;
    const num = /^(\d+(?:\.\d+)*)/.exec(h[2].trim().replace(/^§\s*/, ''));
    if (start === -1) {
      if (num && num[1] === section) {
        start = i + 1;
        depth = h[1].length;
      }
    } else if (h[1].length <= depth) {
      return [start, i];
    }
  }
  return start === -1 ? null : [start, lines.length];
};

export const idPattern = (prefixes) =>
  `(?:${prefixes.map((p) => p.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('|')})-?\\d{2,}[a-z]?`;

/**
 * Extrai as definições primárias de um documento sob o contrato de RF5:
 * tabela com a assinatura do perfil, prosa canônica, e a forma destacada
 * quando o documento está no escopo fechado.
 */
export const extractDefinitions = (lines, profile, highlighted, opts = {}) => {
  const ID = idPattern(profile.prefixes);
  const reTable = new RegExp(`^\\|\\s*\`(${ID})\`\\s*\\|`);
  const reProse = new RegExp(`\`([a-zç]+)\`\\s+\`(${ID})\`\\s+—`);
  const reHighlighted = new RegExp(`\`([a-zç]+)\`\\s*—\\s*\\*\\*\`(${ID})\`\\.`);
  // Uma linha de tabela só é definição se o cabeçalho da sua tabela está na lista do
  // perfil: o mesmo ID reaparece nas tabelas de índice, de vetores e de rastreabilidade.
  const headers = opts.allHeaders ? null : new Set(profile.definitionTableHeaders ?? []);
  const found = new Map();
  const duplicates = [];
  const rows = [];
  const add = (id, line, form, force) => {
    if (found.has(id)) duplicates.push({ id, line, form, first: found.get(id) });
    else found.set(id, { line, form, force });
  };
  let header = null;
  lines.forEach((l, i) => {
    if (/^\|[\s:|-]+\|$/.test(l)) header = (lines[i - 1] ?? '').trim();
    const p = reProse.exec(l);
    if (p) add(p[2], i + 1, 'prosa', p[1]);
    const h = reHighlighted.exec(l);
    if (h && highlighted && highlighted.file === profile.file) add(h[2], i + 1, 'destacada', h[1]);
    const t = reTable.exec(l);
    if (t && header) {
      if (headers === null) rows.push({ id: t[1], line: i + 1, header });
      else if (headers.has(header)) add(t[1], i + 1, 'tabela', null);
    }
  });
  return { found, duplicates, rows };
};

/** Citações de seção com documento adjacente. O símbolo nu fica fora do escopo de C3 (contrato de RF5). */
export const extractCitations = (lines, docTokens) => {
  // `FND-\\d{2}` entra sempre: um documento fora do mapa precisa ser reconhecido
  // como citação para ser acusado, e não ignorado por não casar o padrão.
  const tokens = [
    'FND-\\d{2}',
    ...docTokens.map((t) => t.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')),
  ].join('|');
  const re = new RegExp(
    `\\b(${tokens})\\s+§(§?)\\s*(\\d+(?:\\.\\d+)*)((?:\\s*[–—-]\\s*\\d+(?:\\.\\d+)*)|(?:\\s*(?:,|e)\\s*§\\s*\\d+(?:\\.\\d+)*)*)`,
    'g',
  );
  const out = [];
  lines.forEach((l, i) => {
    if (/^#{1,6}\s/.test(l)) return;
    for (const m of l.matchAll(re)) {
      const sections = [m[3]];
      const tail = m[4] || '';
      const range = /^\s*[–—-]\s*(\d+(?:\.\d+)*)$/.exec(tail);
      if (range && m[2] === '§') {
        const a = Number(m[3]);
        const b = Number(range[1]);
        if (Number.isInteger(a) && Number.isInteger(b) && b > a) {
          sections.length = 0;
          for (let k = a; k <= b; k += 1) sections.push(String(k));
        }
      } else {
        for (const x of tail.matchAll(/§\s*(\d+(?:\.\d+)*)/g)) sections.push(x[1]);
      }
      out.push({ doc: m[1], sections, line: i + 1, raw: m[0].trim() });
    }
  });
  return out;
};

export const RE_COMPLETE = /\b((?:[\w.-]+\/)*[\w.-]+\.md):(\d+)(?:-(\d+))?/g;
export const RE_ABBREV = /\b(SPEC-[A-Z0-9]{8}):(\d+)(?:-(\d+))?/g;
export const RE_SHORTHAND = /(?<![\w./-]):(\d+)(?:-(\d+))?\b/g;

/** Referências físicas nas quatro formas, com o shorthand herdando no fluxo do texto. */
export const extractPhysicalRefs = (lines) => {
  const out = [];
  let parent = null;
  lines.forEach((l, i) => {
    const events = [];
    for (const m of l.matchAll(RE_COMPLETE))
      events.push({
        at: m.index,
        target: m[1],
        from: Number(m[2]),
        to: Number(m[3] ?? m[2]),
        raw: m[0],
        form: 'completa',
      });
    for (const m of l.matchAll(RE_ABBREV))
      events.push({
        at: m.index,
        target: m[1],
        from: Number(m[2]),
        to: Number(m[3] ?? m[2]),
        raw: m[0],
        form: 'abreviada',
      });
    const masked = l
      .replace(RE_COMPLETE, (s) => ' '.repeat(s.length))
      .replace(RE_ABBREV, (s) => ' '.repeat(s.length));
    for (const m of masked.matchAll(RE_SHORTHAND))
      events.push({
        at: m.index,
        target: null,
        from: Number(m[1]),
        to: Number(m[2] ?? m[1]),
        raw: m[0],
        form: 'shorthand',
      });
    events.sort((a, b) => a.at - b.at);
    for (const e of events) {
      if (e.target) parent = e.target;
      out.push({ ...e, target: e.target ?? parent, line: i + 1 });
    }
  });
  return out;
};

const c1 = (ctx) => {
  const out = [];
  const owner = new Map();
  for (const [doc, p] of Object.entries(ctx.profiles.documents))
    for (const pfx of p.prefixes) owner.set(pfx, doc);
  for (const [doc, p] of Object.entries(ctx.profiles.documents)) {
    const { duplicates } = ctx.extracted[doc];
    for (const d of duplicates)
      out.push(
        violation(
          'C1',
          p.file,
          d.line,
          d.id,
          `definição primária duplicada (a primeira está na linha ${d.first.line})`,
        ),
      );
  }
  // ownership: definição de prefixo alheio dentro de um documento
  for (const [doc, p] of Object.entries(ctx.profiles.documents)) {
    const alheios = [...owner.keys()].filter((pfx) => owner.get(pfx) !== doc);
    if (alheios.length === 0) continue;
    const ID = idPattern(alheios);
    const reProse = new RegExp(`\`[a-zç]+\`\\s+\`(${ID})\`\\s+—`);
    ctx.files[p.file].forEach((l, i) => {
      const m = reProse.exec(l);
      if (m) {
        const pfx = alheios.find((x) => m[1].startsWith(x));
        out.push(
          violation(
            'C1',
            p.file,
            i + 1,
            m[1],
            `definição fora do documento dono — o prefixo \`${pfx}\` pertence a ${owner.get(pfx)}`,
          ),
        );
      }
    });
  }
  return out;
};

const c2a = (ctx) => {
  const out = [];
  for (const [doc, p] of Object.entries(ctx.profiles.documents)) {
    const found = ctx.extracted[doc].found;
    const declared = new Set(Object.keys(p.ids));
    for (const id of declared)
      if (!found.has(id))
        out.push(
          violation('C2a', p.file, p.ids[id], id, 'ID do perfil sem definição primária extraída'),
        );
    for (const id of found.keys())
      if (!declared.has(id))
        out.push(
          violation(
            'C2a',
            p.file,
            found.get(id).line,
            id,
            'definição extraída que o perfil não declara',
          ),
        );
    for (const id of declared) {
      const f = found.get(id);
      if (f && f.line !== p.ids[id])
        out.push(
          violation(
            'C2a',
            p.file,
            f.line,
            id,
            `definição em linha diferente da que o perfil declara (${p.ids[id]})`,
          ),
        );
    }
    const entry = ctx.counts.totals[p.file];
    if (!entry) {
      out.push(violation('C2a', p.file, null, null, 'artefato sem entrada em counts.json'));
      continue;
    }
    if (found.size !== entry.value)
      out.push(
        violation(
          'C2a',
          p.file,
          null,
          null,
          `extraídas ${found.size} definições, o índice declara ${entry.value}`,
        ),
      );
    const declLine = ctx.files[p.file][entry.line - 1];
    if (declLine === undefined || !new RegExp(entry.pattern).test(declLine)) {
      out.push(
        violation(
          'C2a',
          p.file,
          entry.line,
          null,
          'a declaração de total não está na linha que counts.json aponta, ou mudou de forma',
        ),
      );
    }
  }
  return out;
};

const c2b = (ctx) => {
  const out = [];
  const readme = ctx.files['README.md'];
  if (!readme) return [violation('C2b', 'README.md', null, null, 'README do acervo ausente')];
  const readmeText = readme.join('\n');
  for (const proj of ctx.counts.readmeProjections) {
    const rm = new RegExp(proj.readmePattern).exec(readmeText);
    if (!rm) {
      out.push(
        violation(
          'C2b',
          'README.md',
          null,
          proj.id,
          `projeção não encontrada no README (padrão: ${proj.readmePattern})`,
        ),
      );
      continue;
    }
    const srcLines = ctx.files[proj.source];
    if (!srcLines) {
      out.push(
        violation(
          'C2b',
          proj.source,
          null,
          proj.id,
          'métrica-fonte aponta para artefato inexistente',
        ),
      );
      continue;
    }
    const sm = new RegExp(proj.sourcePattern).exec(srcLines.join('\n'));
    if (!sm) {
      out.push(
        violation(
          'C2b',
          proj.source,
          null,
          proj.id,
          `métrica-fonte não encontrada (padrão: ${proj.sourcePattern})`,
        ),
      );
      continue;
    }
    if (rm[1] !== sm[1])
      out.push(
        violation(
          'C2b',
          'README.md',
          null,
          proj.id,
          `o README projeta ${rm[1]} e ${proj.source} declara ${sm[1]}`,
        ),
      );
  }
  for (const f of ctx.counts.readmeForbidden ?? []) {
    if (new RegExp(f.pattern).test(readmeText))
      out.push(
        violation(
          'C2b',
          'README.md',
          null,
          f.id,
          `afirmação que a sincronização removeu voltou ao README — ${f.why}`,
        ),
      );
  }
  // O mapa de navegação projeta a contagem por prefixo e por documento. Se ele
  // envelhecer, reproduz a classe de defeito que esta entrega extingue.
  const mapa = ctx.counts.navigationMap;
  if (mapa) {
    const lines = ctx.files[mapa.file];
    if (!lines) out.push(violation('C2b', mapa.file, null, null, 'mapa de navegação ausente'));
    else {
      const real = new Map();
      const porDoc = new Map();
      for (const [doc, p] of Object.entries(ctx.profiles.documents)) {
        porDoc.set(doc, ctx.extracted[doc].found.size);
        for (const id of ctx.extracted[doc].found.keys()) {
          const pfx = p.prefixes
            .filter((x) => id.startsWith(x))
            .sort((a, b) => b.length - a.length)[0];
          if (pfx) real.set(pfx, (real.get(pfx) ?? 0) + 1);
        }
      }
      const declaradoPfx = new Map();
      const declaradoDoc = new Map();
      lines.forEach((l, i) => {
        const p = new RegExp(mapa.prefixRow).exec(l);
        if (p) declaradoPfx.set(p[1], { n: Number(p[2]), line: i + 1 });
        const d = new RegExp(mapa.documentRow).exec(l);
        if (d) declaradoDoc.set(d[1], { n: Number(d[2]), line: i + 1 });
      });
      for (const [pfx, n] of real) {
        const dec = declaradoPfx.get(pfx);
        if (!dec)
          out.push(
            violation(
              'C2b',
              mapa.file,
              null,
              pfx,
              'prefixo do acervo ausente do mapa de navegação',
            ),
          );
        else if (dec.n !== n)
          out.push(
            violation(
              'C2b',
              mapa.file,
              dec.line,
              pfx,
              `o mapa projeta ${dec.n} regras e o acervo tem ${n}`,
            ),
          );
      }
      for (const pfx of declaradoPfx.keys())
        if (!real.has(pfx))
          out.push(
            violation(
              'C2b',
              mapa.file,
              declaradoPfx.get(pfx).line,
              pfx,
              'prefixo no mapa que o acervo não tem',
            ),
          );
      for (const [doc, n] of porDoc) {
        const dec = declaradoDoc.get(doc);
        if (!dec)
          out.push(
            violation(
              'C2b',
              mapa.file,
              null,
              doc,
              'documento ausente da tabela por documento do mapa',
            ),
          );
        else if (dec.n !== n)
          out.push(
            violation(
              'C2b',
              mapa.file,
              dec.line,
              doc,
              `o mapa projeta ${dec.n} regras para ${doc} e o acervo tem ${n}`,
            ),
          );
      }
    }
  }
  return out;
};

const c3 = (ctx) => {
  const out = [];
  const known = new Set(Object.keys(ctx.docToFile));
  for (const [file, lines] of Object.entries(ctx.files)) {
    for (const cite of extractCitations(lines, ctx.citationTokens)) {
      if (ctx.profiles.ignoredDocuments.includes(cite.doc)) continue;
      if (!known.has(cite.doc)) {
        out.push(
          violation('C3', file, cite.line, cite.raw, `documento \`${cite.doc}\` fora do mapa`),
        );
        continue;
      }
      const targetFile = ctx.docToFile[cite.doc];
      const headings = ctx.headings[targetFile];
      if (!headings) {
        out.push(violation('C3', file, cite.line, cite.raw, `alvo ${targetFile} ausente`));
        continue;
      }
      for (const s of cite.sections)
        if (!headings.has(s))
          out.push(
            violation(
              'C3',
              file,
              cite.line,
              `${cite.doc} §${s}`,
              `seção inexistente em ${targetFile}`,
            ),
          );
    }
  }
  return out;
};

const c4 = (ctx) => {
  const out = [];
  const byKey = new Map();
  for (const e of ctx.lineRefs.references) byKey.set(`${e.source}|${e.sourceLine}|${e.raw}`, e);
  const seen = new Set();
  for (const [file, lines] of Object.entries(ctx.files)) {
    for (const r of extractPhysicalRefs(lines)) {
      const key = `${file}|${r.line}|${r.raw}`;
      const entry = byKey.get(key);
      if (!entry) {
        out.push(
          violation(
            'C4',
            file,
            r.line,
            r.raw,
            'referência física ausente do manifesto line-refs.json',
          ),
        );
        continue;
      }
      seen.add(key);
      const targetPath = join(ctx.root, entry.resolved);
      if (!existsSync(targetPath)) {
        out.push(
          violation(
            'C4',
            file,
            r.line,
            entry.resolved,
            'caminho resolvido do manifesto não existe',
          ),
        );
        continue;
      }
      const targetLines = readFileSync(targetPath, 'utf8').split('\n');
      if (entry.to > targetLines.length) {
        out.push(
          violation(
            'C4',
            file,
            r.line,
            `${entry.resolved}:${entry.to}`,
            `linha além do EOF (o alvo tem ${targetLines.length} linhas)`,
          ),
        );
        continue;
      }
      const excerpt = targetLines[entry.from - 1] ?? '';
      if (!excerpt.includes(entry.excerpt)) {
        out.push(
          violation(
            'C4',
            file,
            r.line,
            `${entry.resolved}:${entry.from}`,
            `conteúdo deslocado — a linha existe, mas não contém o trecho registrado (${JSON.stringify(entry.excerpt.slice(0, 40))})`,
          ),
        );
      }
    }
  }
  for (const key of byKey.keys())
    if (!seen.has(key))
      out.push(
        violation(
          'C4',
          key.split('|')[0],
          Number(key.split('|')[1]),
          key.split('|')[2],
          'entrada do manifesto sem referência correspondente no acervo',
        ),
      );
  return out;
};

const c5 = (ctx) => {
  const out = [];
  const hl = ctx.profiles.highlightedForm;
  const allPrefixes = Object.values(ctx.profiles.documents).flatMap((p) => p.prefixes);
  const ID = idPattern(allPrefixes);
  const reHighlighted = new RegExp(`\`([a-zç]+)\`\\s*—\\s*\\*\\*\`(${ID})\`\\.`, 'g');
  const scope = ctx.sectionRanges[hl.file]?.[hl.section] ?? null;
  for (const [file, lines] of Object.entries(ctx.files)) {
    lines.forEach((l, i) => {
      for (const m of l.matchAll(reHighlighted)) {
        const [, force, id] = m;
        if (file !== hl.file) {
          out.push(
            violation(
              'C5',
              file,
              i + 1,
              id,
              `forma destacada fora do documento do escopo fechado (só ${hl.file})`,
            ),
          );
          continue;
        }
        if (scope && (i + 1 < scope[0] || i + 1 > scope[1])) {
          out.push(
            violation(
              'C5',
              file,
              i + 1,
              id,
              `forma destacada fora de §${hl.section}, que é o escopo fechado`,
            ),
          );
          continue;
        }
        if (!(id in hl.forces)) {
          out.push(
            violation(
              'C5',
              file,
              i + 1,
              id,
              `ID fora do conjunto autorizado da forma destacada (${Object.keys(hl.forces)[0]}..${Object.keys(hl.forces).slice(-1)[0]})`,
            ),
          );
          continue;
        }
        if (force !== hl.forces[id])
          out.push(
            violation(
              'C5',
              file,
              i + 1,
              id,
              `força \`${force}\` divergente da esperada \`${hl.forces[id]}\``,
            ),
          );
      }
    });
  }
  // força admitida por documento
  for (const [doc, p] of Object.entries(ctx.profiles.documents)) {
    for (const [id, d] of ctx.extracted[doc].found) {
      if (d.force && !p.forces.includes(d.force))
        out.push(
          violation(
            'C5',
            p.file,
            d.line,
            id,
            `força \`${d.force}\` não admitida pelo perfil de ${doc}`,
          ),
        );
      const expected = p.forceExceptions?.[id];
      if (expected && d.force !== expected)
        out.push(
          violation(
            'C5',
            p.file,
            d.line,
            id,
            `exceção nomeada esperava força \`${expected}\`, encontrou \`${d.force}\``,
          ),
        );
    }
  }
  return out;
};

const c6 = (ctx) => {
  const out = [];
  for (const range of ctx.profiles.reservedRanges) {
    for (const [doc, p] of Object.entries(ctx.profiles.documents)) {
      for (const [id, d] of ctx.extracted[doc].found) {
        const m = new RegExp(`^${range.prefix}-(\\d+)`).exec(id);
        if (!m) continue;
        const n = Number(m[1]);
        if (n >= range.from && n <= range.to)
          out.push(
            violation(
              'C6',
              p.file,
              d.line,
              id,
              `definição em faixa reservada ${range.prefix}-${range.from}..${range.to}`,
            ),
          );
      }
    }
  }
  return out;
};

const CHECKS = { c1, c2a, c2b, c3, c4, c5, c6 };

export const loadContext = (root) => {
  const manifestPath = (name) => join(root, MANIFEST_DIR, name);
  for (const n of ['profiles.json', 'counts.json', 'line-refs.json']) {
    if (!existsSync(manifestPath(n)))
      throw new Error(`manifesto ausente: ${join(MANIFEST_DIR, n)} (root: ${root})`);
  }
  const profiles = JSON.parse(readFileSync(manifestPath('profiles.json'), 'utf8'));
  const counts = JSON.parse(readFileSync(manifestPath('counts.json'), 'utf8'));
  const lineRefs = JSON.parse(readFileSync(manifestPath('line-refs.json'), 'utf8'));

  const acervo = join(root, ACERVO_DIR);
  if (!existsSync(acervo)) throw new Error(`acervo ausente: ${ACERVO_DIR} (root: ${root})`);
  const files = {};
  for (const f of readdirSync(acervo)
    .filter((x) => x.endsWith('.md'))
    .sort()) {
    files[f] = readFileSync(join(acervo, f), 'utf8').split('\n');
  }
  const headings = {};
  for (const [f, lines] of Object.entries(files)) headings[f] = indexHeadings(lines);

  const docToFile = {};
  for (const [doc, p] of Object.entries(profiles.documents)) docToFile[doc] = p.file;
  for (const [doc, file] of Object.entries(profiles.extraDocuments ?? {})) docToFile[doc] = file;

  const extracted = {};
  for (const [doc, p] of Object.entries(profiles.documents)) {
    if (!files[p.file]) throw new Error(`perfil aponta para arquivo ausente: ${p.file}`);
    extracted[doc] = extractDefinitions(files[p.file], p, profiles.highlightedForm);
  }
  const sectionRanges = {};
  const hl = profiles.highlightedForm;
  if (hl && files[hl.file])
    sectionRanges[hl.file] = { [hl.section]: sectionRange(files[hl.file], hl.section) };

  const citationTokens = [...Object.keys(docToFile), ...profiles.ignoredDocuments];
  return {
    root,
    profiles,
    counts,
    lineRefs,
    files,
    headings,
    docToFile,
    extracted,
    sectionRanges,
    citationTokens,
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
    ctx = loadContext(root);
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
    for (const v of r.violations) {
      process.stdout.write(
        `      ${v.file}${v.line ? `:${v.line}` : ''}  ${v.target ?? ''}  ${v.reason}\n`,
      );
    }
  }
  const totals = Object.keys(ctx.profiles.documents).map((doc) => ctx.extracted[doc].found.size);
  const soma = totals.reduce((a, b) => a + b, 0);
  process.stdout.write(`\ntotais por artefato: ${totals.join(', ')}\n`);
  process.stdout.write(`soma derivada: ${soma} (não é número declarado por artefato algum)\n`);
  process.stdout.write(total === 0 ? '\nsem violações\n' : `\n${total} violação(ões)\n`);
  return total === 0 ? 0 : 1;
};

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exit(main());
}
