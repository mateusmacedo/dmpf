// Harness do verificador de congruência horizontal.
// Executa o CLI por subprocesso e confere código de saída E diagnóstico específico:
// um teste que só olha o exit code passaria com a checagem errada acusando.

import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { after, describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import { buildFixture } from './fixture.mjs';

const HERE = dirname(fileURLToPath(import.meta.url));
const REPO = resolve(HERE, '../../..');
const CLI = join(REPO, 'tools/dmpf-verify.mjs');
const criados = [];

/** Roda o CLI e devolve o relatório JSON, sem lançar em código de saída diferente de zero. */
const run = (root, checks) => {
  const args = [CLI, '--root', root, '--json'];
  if (checks) args.push('--check', checks);
  try {
    return {
      code: 0,
      report: JSON.parse(execFileSync(process.execPath, args, { encoding: 'utf8' })),
    };
  } catch (e) {
    if (e.status === undefined) throw e;
    return { code: e.status, report: e.stdout ? JSON.parse(e.stdout) : null, stderr: e.stderr };
  }
};

const fixture = (mutate) => {
  const root = buildFixture(mutate);
  criados.push(root);
  return root;
};

/** Confere que a checagem esperada falhou, com alvo e motivo, e que nenhuma outra falhou. */
const assertOnly = (report, check, { target, reason }) => {
  const falhas = report.results.filter((r) => !r.ok);
  assert.deepEqual(
    falhas.map((f) => f.check.toLowerCase()),
    [check.toLowerCase()],
    `esperava apenas ${check} falhando, veio ${JSON.stringify(falhas.map((f) => f.check))}`,
  );
  const v = falhas[0].violations;
  assert.ok(v.length >= 1, `${check} sem violação registrada`);
  if (target)
    assert.ok(
      v.some((x) => x.target === target),
      `${check} não apontou o alvo ${target}: ${JSON.stringify(v.map((x) => x.target))}`,
    );
  if (reason)
    assert.ok(
      v.some((x) => reason.test(x.reason)),
      `${check} não deu o motivo esperado: ${JSON.stringify(v.map((x) => x.reason))}`,
    );
};

after(() => {
  for (const r of criados) rmSync(r, { recursive: true, force: true });
});

describe('dmpf-verify — CLI', () => {
  it('imprime o uso e sai com 0 em --help', () => {
    const out = execFileSync(process.execPath, [CLI, '--help'], { encoding: 'utf8' });
    assert.match(out, /--root <dir>/);
  });

  it('sai com 2 quando o argumento é desconhecido', () => {
    const r = run('/inexistente-xyz');
    assert.equal(r.code, 2);
  });

  it('sai com 2 quando os manifestos faltam', () => {
    const r = run(REPO === '/' ? '/tmp' : '/tmp');
    assert.equal(r.code, 2);
  });

  it('--check executa apenas a checagem pedida', () => {
    const root = fixture();
    const r = run(root, 'c6');
    assert.equal(r.code, 0);
    assert.deepEqual(
      r.report.results.map((x) => x.check),
      ['C6'],
    );
  });
});

describe('dmpf-verify — positivo', () => {
  it('a base sintética passa nas sete checagens', () => {
    const root = fixture();
    const r = run(root);
    assert.equal(
      r.code,
      0,
      `esperava 0, veio ${r.code}: ${JSON.stringify(
        r.report?.results.filter((x) => !x.ok),
        null,
        2,
      )}`,
    );
    assert.equal(r.report.total, 0);
    assert.equal(r.report.results.length, 7);
  });
});

describe('C1 — uma definição primária por ID, no documento dono', () => {
  it('acusa definição duplicada', () => {
    const root = fixture((f) => {
      f['docs/dmpf/uow-inbox-outbox.md'] +=
        '\n`normativo` `UOW-01` — Segunda definição do mesmo ID.\n';
    });
    const r = run(root, 'c1');
    assert.equal(r.code, 1);
    assertOnly(r.report, 'C1', { target: 'UOW-01', reason: /duplicada/ });
  });

  it('acusa definição fora do documento dono', () => {
    const root = fixture((f) => {
      f['docs/dmpf/testes-interop.md'] += '\n`normativo` `UOW-99` — Definição de prefixo alheio.\n';
    });
    const r = run(root, 'c1');
    assert.equal(r.code, 1);
    assertOnly(r.report, 'C1', { target: 'UOW-99', reason: /fora do documento dono/ });
  });
});

describe('C2a — totais e conjunto exato de IDs', () => {
  it('acusa ID do perfil sem definição extraída', () => {
    const root = fixture((f) => {
      f['docs/dmpf/uow-inbox-outbox.md'] = f['docs/dmpf/uow-inbox-outbox.md'].replace(
        '`normativo` `UOW-02` —',
        'texto solto sobre `UOW-02` —',
      );
    });
    const r = run(root, 'c2a');
    assert.equal(r.code, 1);
    assertOnly(r.report, 'C2a', { target: 'UOW-02', reason: /sem definição primária extraída/ });
  });

  it('acusa definição que o perfil não declara', () => {
    const root = fixture((f) => {
      f['docs/dmpf/uow-inbox-outbox.md'] +=
        '\n`normativo` `UOW-07` — Regra que o perfil não conhece.\n';
    });
    const r = run(root, 'c2a');
    assert.equal(r.code, 1);
    assertOnly(r.report, 'C2a', { target: 'UOW-07', reason: /o perfil não declara/ });
  });

  it('acusa a declaração de total que mudou de forma', () => {
    const root = fixture((f) => {
      f['docs/dmpf/uow-inbox-outbox.md'] = f['docs/dmpf/uow-inbox-outbox.md'].replace(
        '**2 regras com ID**',
        '**duas regras com ID**',
      );
    });
    const r = run(root, 'c2a');
    assert.equal(r.code, 1);
    assertOnly(r.report, 'C2a', {
      reason: /não está na linha que counts.json aponta, ou mudou de forma/,
    });
  });
});

describe('C2b — projeções do README contra a métrica-fonte', () => {
  it('acusa projeção divergente da fonte', () => {
    const root = fixture((f) => {
      f['docs/dmpf/README.md'] = f['docs/dmpf/README.md'].replace('**2** fontes', '**9** fontes');
    });
    const r = run(root, 'c2b');
    assert.equal(r.code, 1);
    assertOnly(r.report, 'C2b', { target: 'P1', reason: /projeta 9 e .* declara 2/ });
  });

  it('acusa afirmação removida que voltou ao README', () => {
    const root = fixture((f) => {
      f['docs/dmpf/README.md'] += '\nUma afirmação caduca que a sincronização removeu.\n';
    });
    const r = run(root, 'c2b');
    assert.equal(r.code, 1);
    assertOnly(r.report, 'C2b', { target: 'F1', reason: /voltou ao README/ });
  });
});

describe('C3 — citações de seção com documento adjacente', () => {
  it('acusa seção inexistente no alvo', () => {
    const root = fixture((f) => {
      f['docs/dmpf/testes-interop.md'] = f['docs/dmpf/testes-interop.md'].replace(
        'herda de FND-04 §1',
        'herda de FND-04 §99.9',
      );
    });
    const r = run(root, 'c3');
    assert.equal(r.code, 1);
    assertOnly(r.report, 'C3', { target: 'FND-04 §99.9', reason: /seção inexistente/ });
  });

  it('ignora FND-11, que é futuro', () => {
    const root = fixture((f) => {
      f['docs/dmpf/testes-interop.md'] = f['docs/dmpf/testes-interop.md'].replace(
        'e da RFC §1',
        'e da RFC §1, mais FND-11 §4.2',
      );
    });
    const r = run(root, 'c3');
    assert.equal(r.code, 0);
  });

  it('acusa documento fora do mapa', () => {
    const root = fixture((f, m) => {
      m['tools/tests/dmpf-verify/profiles.json'].ignoredDocuments = ['FND-02'];
      f['docs/dmpf/testes-interop.md'] = f['docs/dmpf/testes-interop.md'].replace(
        'e da RFC §1',
        'e da RFC §1, mais FND-11 §4.2',
      );
    });
    const r = run(root, 'c3');
    assert.equal(r.code, 1);
    assertOnly(r.report, 'C3', { reason: /fora do mapa/ });
  });
});

describe('C4 — referências físicas', () => {
  it('acusa linha além do EOF', () => {
    // Uma mutação só: o alvo encolhe, e as duas referências passam do fim do arquivo.
    const root = fixture((f) => {
      f['docs/dmpf/uow-inbox-outbox.md'] = '# UoW mini — DMPF FND-04\n\n## §1. Fronteira\n';
    });
    const r = run(root, 'c4');
    assert.equal(r.code, 1);
    assertOnly(r.report, 'C4', { reason: /além do EOF/ });
  });

  it('distingue conteúdo deslocado de linha inexistente', () => {
    const root = fixture((f) => {
      f['docs/dmpf/uow-inbox-outbox.md'] = f['docs/dmpf/uow-inbox-outbox.md'].replace(
        '`normativo` `UOW-01` — Uma UoW tem exatamente uma transação local.',
        '`normativo` `UOW-01` — Redação inteiramente diferente na mesma linha.',
      );
    });
    const r = run(root, 'c4');
    assert.equal(r.code, 1);
    assertOnly(r.report, 'C4', { reason: /conteúdo deslocado/ });
  });

  it('acusa referência do acervo ausente do manifesto', () => {
    const root = fixture((f) => {
      f['docs/dmpf/testes-interop.md'] += '\nOutra âncora em `rfc-dmpf-foundation-v0.1.md:3`.\n';
    });
    const r = run(root, 'c4');
    assert.equal(r.code, 1);
    assertOnly(r.report, 'C4', { reason: /ausente do manifesto/ });
  });

  it('acusa entrada do manifesto sem referência no acervo', () => {
    const root = fixture((f) => {
      f['docs/dmpf/testes-interop.md'] = f['docs/dmpf/testes-interop.md'].replace(
        ', e o par em :7',
        '',
      );
    });
    const r = run(root, 'c4');
    assert.equal(r.code, 1);
    assertOnly(r.report, 'C4', { reason: /sem referência correspondente/ });
  });
});

describe('C5 — forma destacada, escopo fechado', () => {
  it('aceita a forma dentro do escopo', () => {
    const root = fixture();
    assert.equal(run(root, 'c5').code, 0);
  });

  it('acusa a forma em outro documento', () => {
    const root = fixture((f) => {
      f['docs/dmpf/uow-inbox-outbox.md'] +=
        '\n`normativo` — **`RAS-33`. Forma destacada no documento errado.**\n';
    });
    const r = run(root, 'c5');
    assert.equal(r.code, 1);
    assertOnly(r.report, 'C5', { target: 'RAS-33', reason: /fora do documento do escopo fechado/ });
  });

  it('acusa a forma fora da seção do escopo', () => {
    const root = fixture((f) => {
      f['docs/dmpf/testes-interop.md'] = f['docs/dmpf/testes-interop.md'].replace(
        '## §13. Índice',
        '## §13. Índice\n\n`normativo` — **`RAS-34`. Forma destacada fora de §11.**\n',
      );
    });
    const r = run(root, 'c5');
    assert.equal(r.code, 1);
    assertOnly(r.report, 'C5', { target: 'RAS-34', reason: /fora de §11/ });
  });

  it('acusa ID fora do conjunto autorizado', () => {
    const root = fixture((f) => {
      f['docs/dmpf/testes-interop.md'] = f['docs/dmpf/testes-interop.md'].replace(
        '### §11.1 Modos',
        '### §11.1 Modos\n\n`normativo` — **`RAS-42`. ID fora do conjunto.**\n',
      );
    });
    const r = run(root, 'c5');
    assert.equal(r.code, 1);
    assertOnly(r.report, 'C5', { target: 'RAS-42', reason: /fora do conjunto autorizado/ });
  });

  it('acusa força divergente da esperada', () => {
    const root = fixture((f) => {
      f['docs/dmpf/testes-interop.md'] = f['docs/dmpf/testes-interop.md'].replace(
        '`encaminhado` — **`RAS-41`.',
        '`normativo` — **`RAS-41`.',
      );
    });
    const r = run(root, 'c5');
    assert.equal(r.code, 1);
    assertOnly(r.report, 'C5', { target: 'RAS-41', reason: /divergente da esperada/ });
  });
});

describe('C6 — faixas reservadas', () => {
  it('acusa definição em faixa reservada', () => {
    const root = fixture((f, m) => {
      f['docs/dmpf/testes-interop.md'] = f['docs/dmpf/testes-interop.md'].replace(
        '### §11.1 Modos',
        '### §11.1 Modos\n\n`normativo` `RAS-20` — Definição em faixa reservada.\n',
      );
      m['tools/tests/dmpf-verify/profiles.json'].documents['FND-09'].ids['RAS-20'] = 11;
      m['tools/tests/dmpf-verify/profiles.json'].documents['FND-09'].declaredTotal = 3;
      m['tools/tests/dmpf-verify/counts.json'].totals['testes-interop.md'].value = 3;
    });
    const r = run(root, 'c6');
    assert.equal(r.code, 1);
    assertOnly(r.report, 'C6', { target: 'RAS-20', reason: /faixa reservada/ });
  });
});

describe('teste histórico — o acervo de f46ced2 reprova', () => {
  // Fixture hermética: docs/dmpf/ do commit base, os alvos externos de C4, e os
  // manifestos ATUAIS. Não é `--root` no worktree base — ele não tem os manifestos.
  const BASE = 'f46ced2';

  const buildHistorico = () => {
    const root = mkdtempSync(join(tmpdir(), 'dmpf-hist-'));
    criados.push(root);
    const listar = (dir) =>
      execFileSync('git', ['-C', REPO, 'ls-tree', '--name-only', BASE, `${dir}/`], {
        encoding: 'utf8',
      })
        .trim()
        .split('\n')
        .filter(Boolean);
    const copiar = (rel) => {
      const dest = join(root, rel);
      mkdirSync(dirname(dest), { recursive: true });
      writeFileSync(
        dest,
        execFileSync('git', ['-C', REPO, 'show', `${BASE}:${rel}`], { encoding: 'utf8' }),
        'utf8',
      );
    };
    for (const rel of listar('docs/dmpf')) copiar(rel);
    for (const rel of ['docs/adr/README.md', 'docs/specs/SPEC-DBTRMM3X-dmpf-adrs-minimos.md'])
      copiar(rel);
    mkdirSync(join(root, 'tools/tests/dmpf-verify'), { recursive: true });
    for (const m of ['profiles.json', 'counts.json', 'line-refs.json']) {
      writeFileSync(
        join(root, 'tools/tests/dmpf-verify', m),
        readFileSync(join(REPO, 'tools/tests/dmpf-verify', m), 'utf8'),
        'utf8',
      );
    }
    return root;
  };

  it('reprova, e C2b nomeia as projeções caducas do README', () => {
    const r = run(buildHistorico());
    assert.equal(r.code, 1, 'o acervo anterior à correção precisa reprovar');
    const c2b = r.report.results.find((x) => x.check === 'C2B');
    assert.ok(!c2b.ok, 'C2b precisa acusar o README desatualizado');
    const alvos = c2b.violations.map((v) => v.target);
    for (const esperado of ['P1-fontes', 'P2-obrigacoes', 'P3-identificadores']) {
      assert.ok(alvos.includes(esperado), `C2b não acusou ${esperado}: ${JSON.stringify(alvos)}`);
    }
    const fontes = c2b.violations.find((v) => v.target === 'P1-fontes');
    assert.match(fontes.reason, /projeta seis e testes-interop\.md declara sete/);
  });

  it('reprova, e C2a nomeia as definições que a errata criou', () => {
    const r = run(buildHistorico());
    const c2a = r.report.results.find((x) => x.check === 'C2A');
    assert.ok(!c2a.ok, 'C2a precisa acusar os IDs sem definição no estado anterior');
    const alvos = c2a.violations.map((v) => v.target);
    for (const esperado of ['ESC-04', 'ESC-05', 'ESC-06', 'RAS-30']) {
      assert.ok(alvos.includes(esperado), `C2a não acusou ${esperado}: ${JSON.stringify(alvos)}`);
    }
  });

  it('o acervo corrigido, ao contrário, passa nas sete', () => {
    const r = run(REPO);
    assert.equal(
      r.code,
      0,
      `o acervo atual precisa passar: ${JSON.stringify(
        r.report?.results.filter((x) => !x.ok),
        null,
        2,
      )}`,
    );
  });
});

describe('C2b — mapa de navegação', () => {
  // O mapa projeta a contagem por prefixo e por documento. Sem vigilância ele
  // envelheceria como o README envelheceu — a classe de defeito da entrega.
  const comMapa = (mutar) =>
    fixture((f, m) => {
      f['docs/dmpf/navegacao.md'] =
        '# Mapa mini\n\n| Prefixo | Regras | Doc |\n|---|---|---|\n| `UOW` | 2 |\n| `RAS` | 2 |\n\n| [uow](./uow-inbox-outbox.md) (FND-04) | 2 |\n| [testes](./testes-interop.md) (FND-09) | 2 |\n';
      m['tools/tests/dmpf-verify/counts.json'].navigationMap = {
        file: 'navegacao.md',
        prefixRow: '^\\| `([A-Z]+(?:-[A-Z])?)` \\| (\\d+) \\|',
        documentRow: '^\\| \\[[^\\]]+\\]\\([^)]+\\) \\((FND-\\d{2})\\) \\| (\\d+) \\|',
      };
      if (mutar) mutar(f, m);
    });

  it('o mapa coerente passa', () => {
    assert.equal(run(comMapa(), 'c2b').code, 0);
  });

  it('acusa contagem por prefixo divergente', () => {
    const root = comMapa((f) => {
      f['docs/dmpf/navegacao.md'] = f['docs/dmpf/navegacao.md'].replace(
        '| `UOW` | 2 |',
        '| `UOW` | 9 |',
      );
    });
    const r = run(root, 'c2b');
    assert.equal(r.code, 1);
    assertOnly(r.report, 'C2b', { target: 'UOW', reason: /projeta 9 regras e o acervo tem 2/ });
  });

  it('acusa prefixo do acervo ausente do mapa', () => {
    const root = comMapa((f) => {
      f['docs/dmpf/navegacao.md'] = f['docs/dmpf/navegacao.md'].replace('| `RAS` | 2 |\n', '');
    });
    const r = run(root, 'c2b');
    assert.equal(r.code, 1);
    assertOnly(r.report, 'C2b', { target: 'RAS', reason: /ausente do mapa/ });
  });

  it('acusa total por documento divergente', () => {
    const root = comMapa((f) => {
      f['docs/dmpf/navegacao.md'] = f['docs/dmpf/navegacao.md'].replace(
        '(FND-09) | 2 |',
        '(FND-09) | 7 |',
      );
    });
    const r = run(root, 'c2b');
    assert.equal(r.code, 1);
    assertOnly(r.report, 'C2b', { target: 'FND-09', reason: /projeta 7 regras para FND-09/ });
  });
});
