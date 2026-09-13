// ephemeral-refs-ok-file: os IDs de spec aqui são dado de fixture, não citação viva
// Harness do verificador do acervo de ADRs.
// Executa o CLI por subprocesso e confere código de saída E diagnóstico específico:
// um teste que só olha o exit code passaria com a checagem errada acusando.

import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { rmSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { after, describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import { baseFile, buildFixture } from './fixture.mjs';

const HERE = dirname(fileURLToPath(import.meta.url));
const REPO = resolve(HERE, '../../..');
const CLI = join(REPO, 'tools/adr-verify.mjs');
const criados = [];

const ADR1 = 'docs/adr/001-base-sintetica.md';
const ADR2 = 'docs/adr/002-sem-bloco-de-status.md';
const ADR3 = 'docs/adr/003-pendencia-declarada.md';
const INDEX = 'docs/adr/README.md';

/** Roda o CLI e devolve o relatório JSON, sem lançar em código de saída diferente de zero. */
const run = (root, extra = []) => {
  const args = [CLI, '--root', root, '--json', ...extra];
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

/** Troca um trecho único do arquivo da base, falhando cedo se a âncora não existir. */
const swap = (files, rel, from, to) => {
  const base = baseFile(rel);
  assert.ok(base.includes(from), `âncora ausente da base ${rel}: ${JSON.stringify(from)}`);
  files[rel] = base.replace(from, to);
};

/** Confere que a checagem esperada falhou, com alvo e motivo, e que nenhuma outra falhou. */
const assertOnly = (report, check, { target, reason } = {}) => {
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

describe('adr-verify — CLI', () => {
  it('imprime o uso e sai com 0 em --help', () => {
    const out = execFileSync(process.execPath, [CLI, '--help'], { encoding: 'utf8' });
    assert.match(out, /--root <dir>/);
  });

  it('sai com 2 quando o argumento é desconhecido', () => {
    const root = fixture();
    const r = run(root, ['--bandeira-invalida']);
    assert.equal(r.code, 2);
  });

  it('sai com 2 quando --from não é inteiro', () => {
    const root = fixture();
    const r = run(root, ['--from', 'dez']);
    assert.equal(r.code, 2);
  });

  it('sai com 2 quando o acervo não existe', () => {
    const r = run(join(REPO, 'tools'));
    assert.equal(r.code, 2);
  });

  it('sai com 2 quando a checagem pedida é desconhecida', () => {
    const root = fixture();
    const r = run(root, ['--check', 'a99']);
    assert.equal(r.code, 2);
  });

  it('--check executa apenas a checagem pedida', () => {
    const root = fixture();
    const r = run(root, ['--check', 'a7']);
    assert.equal(r.code, 0);
    assert.deepEqual(
      r.report.results.map((x) => x.check),
      ['A7'],
    );
  });

  it('--from restringe o escopo e não acusa buraco antes do piso', () => {
    const root = fixture((files) => {
      delete files[ADR1];
      delete files[ADR2];
    });
    const r = run(root, ['--from', '3']);
    assert.equal(r.code, 0);
    assert.equal(
      r.report.results.every((x) => x.ok),
      true,
    );
  });

  it('--to exclui o ADR fora do teto', () => {
    const root = fixture();
    const r = run(root, ['--to', '2', '--check', 'a3']);
    assert.equal(r.code, 0);
  });
});

describe('adr-verify — positivo', () => {
  it('a base sintética passa nas oito checagens', () => {
    const root = fixture();
    const r = run(root);
    assert.equal(
      r.code,
      0,
      JSON.stringify(
        r.report?.results?.filter((x) => !x.ok),
        null,
        2,
      ),
    );
    assert.deepEqual(
      r.report.results.map((x) => x.check),
      ['A1', 'A2', 'A3', 'A4', 'A5', 'A6', 'A7', 'A8'],
    );
    assert.equal(r.report.total, 0);
  });

  it('o relatório em texto informa quantos ADRs entraram no escopo', () => {
    const root = fixture();
    const out = execFileSync(process.execPath, [CLI, '--root', root], { encoding: 'utf8' });
    assert.match(out, /ADRs no escopo: 3/);
    assert.match(out, /sem violações/);
  });
});

describe('adr-verify — A1 bijeção arquivo ↔ heading', () => {
  it('acusa H1 com número diferente do arquivo', () => {
    const root = fixture((files) => {
      swap(files, ADR1, '# ADR-001: Fixar', '# ADR-007: Fixar');
    });
    const r = run(root, ['--check', 'a1']);
    assert.equal(r.code, 1);
    assertOnly(r.report, 'a1', { target: '001', reason: /H1 numera ADR-007/ });
  });

  it('acusa arquivo sem H1 de ADR', () => {
    const root = fixture((files) => {
      swap(files, ADR1, '# ADR-001: Fixar a base sintética do harness', '# Sem identidade');
    });
    const r = run(root, ['--check', 'a1']);
    assert.equal(r.code, 1);
    assertOnly(r.report, 'a1', { target: '001', reason: /não é o H1/ });
  });

  it('acusa conteúdo antes do H1 — o formato é sem frontmatter', () => {
    const root = fixture((files) => {
      files[ADR1] = `---\nstatus: aceito\n---\n\n${baseFile(ADR1)}`;
    });
    const r = run(root, ['--check', 'a1']);
    assert.equal(r.code, 1);
    assertOnly(r.report, 'a1', { reason: /sem frontmatter/ });
  });

  it('acusa segundo H1 de ADR no mesmo arquivo', () => {
    const root = fixture((files) => {
      files[ADR1] = `${baseFile(ADR1)}\n# ADR-001: Título repetido\n`;
    });
    const r = run(root, ['--check', 'a1']);
    assert.equal(r.code, 1);
    assertOnly(r.report, 'a1', { reason: /segundo H1/ });
  });

  it('acusa arquivo em docs/adr fora do padrão de nome', () => {
    const root = fixture((files) => {
      files['docs/adr/notas-soltas.md'] = '# Notas\n';
    });
    const r = run(root, ['--check', 'a1']);
    assert.equal(r.code, 1);
    assertOnly(r.report, 'a1', { target: 'notas-soltas.md', reason: /fora do padrão/ });
  });
});

describe('adr-verify — A2 bijeção arquivo ↔ índice', () => {
  it('acusa ADR em disco sem entrada no índice', () => {
    const root = fixture((files) => {
      swap(
        files,
        INDEX,
        '| ADR-003 | Registrar uma pendência com owner e prazo | Promove a parte decidível e registra o resto como pendência. |\n',
        '',
      );
    });
    const r = run(root, ['--check', 'a2']);
    assert.equal(r.code, 1);
    assertOnly(r.report, 'a2', { target: 'ADR-003', reason: /sem entrada no índice/ });
  });

  it('acusa entrada no índice sem arquivo em disco', () => {
    const root = fixture((files) => {
      delete files[ADR3];
    });
    const r = run(root, ['--check', 'a2']);
    assert.equal(r.code, 1);
    assertOnly(r.report, 'a2', { target: 'ADR-003', reason: /sem arquivo correspondente/ });
  });

  it('acusa divergência entre o título do índice e o do H1', () => {
    const root = fixture((files) => {
      swap(
        files,
        INDEX,
        '| ADR-002 | Cobrir o ADR sem bloco de status |',
        '| ADR-002 | Outro título |',
      );
    });
    const r = run(root, ['--check', 'a2']);
    assert.equal(r.code, 1);
    assertOnly(r.report, 'a2', { target: 'ADR-002', reason: /o índice titula/ });
  });

  it('acusa entrada do índice sem a coluna «O que decide»', () => {
    const root = fixture((files) => {
      swap(
        files,
        INDEX,
        '| ADR-002 | Cobrir o ADR sem bloco de status | Trava por teste a opcionalidade do bloco `## Status`. |',
        '| ADR-002 | Cobrir o ADR sem bloco de status |  |',
      );
    });
    const r = run(root, ['--check', 'a2']);
    assert.equal(r.code, 1);
    assertOnly(r.report, 'a2', { target: 'ADR-002', reason: /sem a coluna/ });
  });

  it('acusa entrada duplicada no índice', () => {
    const root = fixture((files) => {
      swap(
        files,
        INDEX,
        '| ADR-002 | Cobrir o ADR sem bloco de status | Trava por teste a opcionalidade do bloco `## Status`. |\n',
        '| ADR-002 | Cobrir o ADR sem bloco de status | Trava por teste a opcionalidade do bloco `## Status`. |\n| ADR-002 | Cobrir o ADR sem bloco de status | Repetida. |\n',
      );
    });
    const r = run(root, ['--check', 'a2']);
    assert.equal(r.code, 1);
    assertOnly(r.report, 'a2', { target: 'ADR-002', reason: /duplicada/ });
  });

  it('acusa a seção «Decisões registradas» ausente do índice', () => {
    const root = fixture((files) => {
      swap(files, INDEX, '## Decisões registradas', '## Outra seção qualquer');
    });
    const r = run(root, ['--check', 'a2']);
    assert.equal(r.code, 1);
    assertOnly(r.report, 'a2', { reason: /seção «Decisões registradas» ausente/ });
  });
});

describe('adr-verify — A3 numeração contínua', () => {
  it('acusa buraco na sequência', () => {
    const root = fixture((files) => {
      delete files[ADR2];
      swap(
        files,
        INDEX,
        '| ADR-002 | Cobrir o ADR sem bloco de status | Trava por teste a opcionalidade do bloco `## Status`. |\n',
        '',
      );
    });
    const r = run(root, ['--check', 'a3']);
    assert.equal(r.code, 1);
    assertOnly(r.report, 'a3', { target: '002', reason: /buraco na sequência/ });
  });

  it('acusa número repetido em dois arquivos', () => {
    const root = fixture((files) => {
      files['docs/adr/003-outro-slug.md'] = baseFile(ADR3);
    });
    const r = run(root, ['--check', 'a3']);
    assert.equal(r.code, 1);
    assertOnly(r.report, 'a3', { target: '003', reason: /número repetido/ });
  });
});

describe('adr-verify — A4 seções obrigatórias e ordem canônica', () => {
  for (const secao of ['Contexto', 'Decisão', 'Alternativas descartadas', 'Consequências']) {
    it(`acusa a ausência de «${secao}»`, () => {
      const root = fixture((files) => {
        swap(files, ADR2, `## ${secao}\n`, `## Anotações sobre ${secao}\n`);
      });
      const r = run(root, ['--check', 'a4']);
      assert.equal(r.code, 1);
      assertOnly(r.report, 'a4', { target: secao, reason: /obrigatória ausente/ });
    });
  }

  it('acusa seção obrigatória repetida', () => {
    const root = fixture((files) => {
      files[ADR2] = `${baseFile(ADR2)}\n## Contexto\n\nDe novo.\n`;
    });
    const r = run(root, ['--check', 'a4']);
    assert.equal(r.code, 1);
    assertOnly(r.report, 'a4', { target: 'Contexto', reason: /repetida/ });
  });

  it('acusa «Status» que não abre o arquivo', () => {
    const root = fixture((files) => {
      swap(
        files,
        ADR1,
        '## Status\n\nAceito — 2026-08-30. Implementa SPEC-AAAA1111.\n\n## Contexto',
        '## Contexto',
      );
      files[ADR1] = files[ADR1].replace(
        '## Decisão',
        '## Status\n\nAceito — 2026-08-30. Implementa SPEC-AAAA1111.\n\n## Decisão',
      );
    });
    const r = run(root, ['--check', 'a4']);
    assert.equal(r.code, 1);
    assertOnly(r.report, 'a4', { target: 'Status', reason: /abre o arquivo/ });
  });

  it('acusa seção canônica fora de ordem', () => {
    const root = fixture((files) => {
      swap(files, ADR1, '## Referências\n\n- ADR-002 — o segundo ADR da base.\n', '');
      files[ADR1] = files[ADR1].replace(
        '## Consequências',
        '## Referências\n\n- ADR-002 — o segundo ADR da base.\n\n## Consequências',
      );
    });
    const r = run(root, ['--check', 'a4']);
    assert.equal(r.code, 1);
    assertOnly(r.report, 'a4', {
      target: 'Consequências',
      reason: /depois de «Referências», fora da ordem canônica/,
    });
  });

  it('acusa addendum que não fecha o arquivo', () => {
    const root = fixture((files) => {
      files[ADR2] = `${baseFile(ADR2)}\n## Consequências adicionais\n\nDepois do addendum.\n`;
    });
    const r = run(root, ['--check', 'a4']);
    assert.equal(r.code, 1);
    assertOnly(r.report, 'a4', { reason: /addendum é acrescentado ao final/ });
  });
});

describe('adr-verify — A5 forma interna', () => {
  it('acusa cabeçalho de alternativas fora da forma literal', () => {
    const root = fixture((files) => {
      swap(
        files,
        ADR1,
        '| Alternativa | Por que foi rejeitada |',
        '| Alternativa | Por que foi descartada |',
      );
    });
    const r = run(root, ['--check', 'a5']);
    assert.equal(r.code, 1);
    assertOnly(r.report, 'a5', {
      target: 'Alternativas descartadas',
      reason: /cabeçalho literal/,
    });
  });

  it('acusa tabela de alternativas sem nenhuma linha', () => {
    const root = fixture((files) => {
      swap(
        files,
        ADR1,
        '| Reaproveitar um ADR real do acervo | Amarra o harness ao conteúdo, que muda por outra trilha. |\n',
        '',
      );
    });
    const r = run(root, ['--check', 'a5']);
    assert.equal(r.code, 1);
    assertOnly(r.report, 'a5', {
      target: 'Alternativas descartadas',
      reason: /sem nenhuma linha de alternativa/,
    });
  });

  it('acusa cabeçalho de alternativas sem linha separadora', () => {
    const root = fixture((files) => {
      swap(files, ADR1, '| ----------- | --------------------- |\n', '');
    });
    const r = run(root, ['--check', 'a5']);
    assert.equal(r.code, 1);
    assertOnly(r.report, 'a5', { reason: /sem a linha separadora/ });
  });

  for (const rotulo of ['**Positivas:**', '**Negativas:**']) {
    it(`acusa a ausência de ${rotulo} em Consequências`, () => {
      const root = fixture((files) => {
        swap(files, ADR2, rotulo, rotulo.replace(/\*\*/g, ''));
      });
      const r = run(root, ['--check', 'a5']);
      assert.equal(r.code, 1);
      assertOnly(r.report, 'a5', { target: rotulo, reason: /rótulo ausente/ });
    });
  }

  it('aceita o custo inline no bullet da negativa que ele precifica', () => {
    const root = fixture((files) => {
      swap(
        files,
        ADR1,
        '- **Custo aceito:** manter a base em dia com o guia é trabalho recorrente.',
        '- A base duplica o guia. **Custo aceito:** manter a base em dia é recorrente.',
      );
    });
    const r = run(root, ['--check', 'a5']);
    assert.equal(r.code, 0);
  });

  it('aceita mais de um custo, casado 1:1 com cada negativa', () => {
    const root = fixture();
    const r = run(root, ['--check', 'a5']);
    assert.equal(r.code, 0);
    assert.equal(
      (baseFile(ADR3).match(/\*\*Custo aceito:\*\*/g) ?? []).length,
      2,
      'a base precisa manter dois custos inline para travar essa forma',
    );
  });

  it('acusa «Custo aceito» sem o negrito do rótulo', () => {
    const root = fixture((files) => {
      swap(files, ADR2, '- **Custo aceito:** a base fica', '- Custo aceito: a base fica');
    });
    const r = run(root, ['--check', 'a5']);
    assert.equal(r.code, 1);
    assertOnly(r.report, 'a5', {
      target: '**Custo aceito:**',
      reason: /ausente do bloco \*\*Negativas:\*\*/,
    });
  });

  it('acusa «Custo aceito» declarado em Positivas', () => {
    const root = fixture((files) => {
      swap(
        files,
        ADR2,
        '- A opcionalidade do status fica travada por teste.\n\n**Negativas:**\n\n- **Custo aceito:** a base fica com dois formatos de ADR em vez de um.',
        '- **Custo aceito:** declarado no bloco errado.\n\n**Negativas:**\n\n- A base fica com dois formatos de ADR em vez de um.',
      );
    });
    const r = run(root, ['--check', 'a5']);
    assert.equal(r.code, 1);
    assertOnly(r.report, 'a5', {
      target: '**Custo aceito:**',
      reason: /fora do bloco \*\*Negativas:\*\*/,
    });
  });

  it('acusa «Custo aceito» fora da seção Consequências', () => {
    const root = fixture((files) => {
      swap(
        files,
        ADR2,
        '- **Custo aceito:** a base fica com dois formatos de ADR em vez de um.',
        '- Sem custo declarado.',
      );
      files[ADR2] = files[ADR2].replace(
        '## Addendum — 2026-08-30',
        '## Addendum — 2026-08-30\n\n- **Custo aceito:** declarado no lugar errado.',
      );
    });
    const r = run(root, ['--check', 'a5']);
    assert.equal(r.code, 1);
    assertOnly(r.report, 'a5', { target: '**Custo aceito:**', reason: /fora do bloco/ });
  });
});

describe('adr-verify — A6 pendência com owner e prazo', () => {
  it('acusa pendência sem rubrica de owner', () => {
    const root = fixture((files) => {
      swap(
        files,
        ADR3,
        '- **Quem decide (owner):** a própria série de ADRs desta base.\n',
        '- **Quem decide:** a própria série de ADRs desta base.\n',
      );
    });
    const r = run(root, ['--check', 'a6']);
    assert.equal(r.code, 1);
    assertOnly(r.report, 'a6', { reason: /sem rubrica de owner/ });
  });

  it('acusa rubrica de owner declarada e vazia', () => {
    const root = fixture((files) => {
      swap(
        files,
        ADR3,
        '- **Quem decide (owner):** a própria série de ADRs desta base.',
        '- **Quem decide (owner):**',
      );
    });
    const r = run(root, ['--check', 'a6']);
    assert.equal(r.code, 1);
    assertOnly(r.report, 'a6', { reason: /owner declarada e vazia/ });
  });

  it('acusa pendência sem rubrica de prazo', () => {
    const root = fixture((files) => {
      swap(
        files,
        ADR3,
        '- **Prazo:** a fonte de origem não fixa data, e o elo fica explicitamente aberto.\n',
        '',
      );
    });
    const r = run(root, ['--check', 'a6']);
    assert.equal(r.code, 1);
    assertOnly(r.report, 'a6', { reason: /sem a rubrica `- \*\*Prazo:\*\*`/ });
  });

  it('acusa rubrica de prazo declarada e vazia', () => {
    const root = fixture((files) => {
      swap(
        files,
        ADR3,
        '- **Prazo:** a fonte de origem não fixa data, e o elo fica explicitamente aberto.',
        '- **Prazo:**',
      );
    });
    const r = run(root, ['--check', 'a6']);
    assert.equal(r.code, 1);
    assertOnly(r.report, 'a6', { reason: /prazo declarada e vazia/ });
  });

  it('acusa status que declara pendência sem o bloco que a registra', () => {
    const root = fixture((files) => {
      swap(files, ADR3, '### Pendência declarada — o mecanismo', '### Nota sobre o mecanismo');
    });
    const r = run(root, ['--check', 'a6']);
    assert.equal(r.code, 1);
    assertOnly(r.report, 'a6', { target: 'Status', reason: /sem o bloco/ });
  });

  it('não confunde a palavra «pendente» fora do bloco de status', () => {
    const root = fixture((files) => {
      swap(
        files,
        ADR2,
        'Manter na base um ADR sem status,',
        'A escolha do formato continua pendente em outra trilha. Manter na base um ADR sem status,',
      );
    });
    const r = run(root, ['--check', 'a6']);
    assert.equal(r.code, 0);
  });
});

describe('adr-verify — A7 rastreabilidade', () => {
  it('acusa referência à fonte por caminho e número de linha', () => {
    const root = fixture((files) => {
      swap(
        files,
        ADR1,
        '- ADR-002 — o segundo ADR da base.',
        '- A âncora está em `docs/adr/002-sem-bloco-de-status.md:12`.',
      );
    });
    const r = run(root, ['--check', 'a7']);
    assert.equal(r.code, 1);
    assertOnly(r.report, 'a7', {
      target: 'docs/adr/002-sem-bloco-de-status.md:12',
      reason: /por número de linha/,
    });
  });

  it('acusa referência abreviada por spec e número de linha', () => {
    const root = fixture((files) => {
      swap(
        files,
        ADR1,
        '- ADR-002 — o segundo ADR da base.',
        '- A origem está em SPEC-AAAA1111:40.',
      );
    });
    const r = run(root, ['--check', 'a7']);
    assert.equal(r.code, 1);
    assertOnly(r.report, 'a7', { target: 'SPEC-AAAA1111:40' });
  });

  it('acusa referência no shorthand `:N`', () => {
    const root = fixture((files) => {
      swap(files, ADR1, '- ADR-002 — o segundo ADR da base.', '- O par está em :7.');
    });
    const r = run(root, ['--check', 'a7']);
    assert.equal(r.code, 1);
    assertOnly(r.report, 'a7', { target: ':7' });
  });

  it('acusa referência por linha também no índice', () => {
    const root = fixture((files) => {
      swap(
        files,
        INDEX,
        'Índice e guia de escrita da base sintética.',
        'Ver `docs/adr/001-base-sintetica.md:3`.',
      );
    });
    const r = run(root, ['--check', 'a7']);
    assert.equal(r.code, 1);
    assertOnly(r.report, 'a7', { target: 'docs/adr/001-base-sintetica.md:3' });
  });

  it('não confunde URL com referência por linha', () => {
    const root = fixture((files) => {
      swap(
        files,
        ADR1,
        '- ADR-002 — o segundo ADR da base.',
        '- Ver [ARQ-448](https://exemplo.invalido/browse/ARQ-448) e a seção §7.1.',
      );
    });
    const r = run(root, ['--check', 'a7']);
    assert.equal(r.code, 0);
  });
});

describe('adr-verify — A8 origem', () => {
  it('acusa spec de origem sem arquivo em docs/specs', () => {
    const root = fixture((files) => {
      delete files['docs/specs/SPEC-AAAA1111-base-sintetica.md'];
    });
    const r = run(root, ['--check', 'a8']);
    assert.equal(r.code, 1);
    assertOnly(r.report, 'a8', { target: 'SPEC-AAAA1111', reason: /sem arquivo em docs\/specs/ });
  });

  it('ignora ID de spec citado fora do bloco de status', () => {
    const root = fixture((files) => {
      swap(
        files,
        ADR2,
        'O guia declara',
        'A spec SPEC-ZZZZ9999 não existe e é citada no contexto. O guia declara',
      );
    });
    const r = run(root, ['--check', 'a8']);
    assert.equal(r.code, 0);
  });
});
