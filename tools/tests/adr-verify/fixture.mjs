// ephemeral-refs-ok-file: os IDs de spec aqui são dado de fixture, não citação viva
// Constrói um acervo sintético mínimo de ADRs que passa nas oito checagens, em
// diretório temporário. Cada teste negativo aplica UMA mutação sobre esta base —
// duas violações no mesmo negativo tornariam o diagnóstico ambíguo.

import { mkdirSync, mkdtempSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';

const ADR_001 = `# ADR-001: Fixar a base sintética do harness

## Status

Aceito — 2026-08-30. Implementa SPEC-AAAA1111.

## Contexto

O harness precisa de um ADR mínimo que satisfaça o contrato do guia de escrita.

## Decisão

Manter um ADR de base com as quatro seções obrigatórias e o bloco de status.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Reaproveitar um ADR real do acervo | Amarra o harness ao conteúdo, que muda por outra trilha. |

## Consequências

**Positivas:**

- A base exercita o caminho feliz de todas as checagens.

**Negativas:**

- **Custo aceito:** manter a base em dia com o guia é trabalho recorrente.

## Referências

- ADR-002 — o segundo ADR da base.
`;

const ADR_002 = `# ADR-002: Cobrir o ADR sem bloco de status

## Contexto

O guia declara o bloco \`## Status\` opcional, e ADR-001 e ADR-002 do acervo real
não o trazem.

## Decisão

Manter na base um ADR sem status, para que a checagem de seções não o exija.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Exigir status em todo ADR | Contraria o guia, que o declara opcional. |

## Consequências

**Positivas:**

- A opcionalidade do status fica travada por teste.

**Negativas:**

- **Custo aceito:** a base fica com dois formatos de ADR em vez de um.

## Addendum — 2026-08-30

O addendum vai ao final, depois de todas as seções.
`;

const ADR_003 = `# ADR-003: Registrar uma pendência com owner e prazo

## Status

Aceito — 2026-08-30. Implementa SPEC-BBBB2222. A escolha do mecanismo permanece
**pendente**, registrada na subseção própria da seção Decisão.

## Contexto

Um ADR pode promover a parte decidível e deixar o resto em aberto.

## Decisão

Promover a parte decidível e registrar o resto como pendência declarada.

### Pendência declarada — o mecanismo

O mecanismo não é decidido aqui e permanece **aberto**:

- **Quem decide (owner):** a própria série de ADRs desta base.
- **Prazo:** a fonte de origem não fixa data, e o elo fica explicitamente aberto.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Decidir o mecanismo sem insumo | Anteciparia uma escolha sem material para fundamentá-la. |

## Consequências

**Positivas:**

- A parte decidível não fica refém da parte indecidível.

**Negativas:**

- A operação de quem consome esta decisão fica condicionada ao que ela defere, e a
  fonte não fixa prazo. **Custo aceito:** a pendência é, ela própria, um elo em
  aberto, e este ADR não promete o fechamento que não tem.
- O leitor precisa percorrer duas seções para saber o que ficou decidido.
  **Custo aceito:** casar cada custo com a negativa que ele precifica vale a
  releitura.
`;

const INDEX = `# Decisões Arquiteturais (ADRs)

Índice e guia de escrita da base sintética.

## Decisões registradas

| Nº | Título | O que decide |
| --- | --- | --- |
| ADR-001 | Fixar a base sintética do harness | Mantém um ADR mínimo que satisfaz o contrato do guia. |
| ADR-002 | Cobrir o ADR sem bloco de status | Trava por teste a opcionalidade do bloco \`## Status\`. |
| ADR-003 | Registrar uma pendência com owner e prazo | Promove a parte decidível e registra o resto como pendência. |
`;

const FILES = {
  'docs/adr/001-base-sintetica.md': ADR_001,
  'docs/adr/002-sem-bloco-de-status.md': ADR_002,
  'docs/adr/003-pendencia-declarada.md': ADR_003,
  'docs/adr/README.md': INDEX,
  'docs/specs/SPEC-AAAA1111-base-sintetica.md': '# SPEC-AAAA1111\n',
  'docs/specs/SPEC-BBBB2222-pendencia.md': '# SPEC-BBBB2222\n',
};

/**
 * Monta a base num diretório temporário.
 * `mutate` recebe o mapa de arquivos e altera exatamente um ponto.
 */
export const buildFixture = (mutate) => {
  const root = mkdtempSync(join(tmpdir(), 'adr-verify-'));
  const files = { ...FILES };
  if (mutate) mutate(files);
  for (const [rel, content] of Object.entries(files)) {
    const dest = join(root, rel);
    mkdirSync(dirname(dest), { recursive: true });
    writeFileSync(dest, content, 'utf8');
  }
  return root;
};

/** Lê a base para conferir que uma âncora de mutação existe antes de aplicá-la. */
export const baseFile = (rel) => FILES[rel];
