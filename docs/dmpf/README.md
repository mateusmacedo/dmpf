# DMPF — artefatos promovidos

Índice versionado da fundação **DMPF** (épico [ARQ-436](https://lider-cap.atlassian.net/browse/ARQ-436)).

Os drafts de trabalho continuam em `plans/references/` (gitignored). Quando
estão prontos para revisão, são **promovidos** para cá; a aprovação formal
ocorre no PR (reviewers de Plataforma/Arquitetura). Specs permanecem em
`docs/specs/`. ADRs do DMPF, quando acionados, usam a faixa `docs/adr/010`–`024`
(contínua aos `001`–`009` do template).

## Artefatos

| Artefato | Status | Spec / Issue |
|----------|--------|--------------|
| [inventario-as-is.md](./inventario-as-is.md) | **baseline candidato** (promovido para revisão; AC externo aberto) | [SPEC-K9H204F1](../specs/SPEC-K9H204F1-dmpf-inventario-as-is.md) / [ARQ-438](https://lider-cap.atlassian.net/browse/ARQ-438) |
| [rfc-dmpf-foundation-v0.1.md](./rfc-dmpf-foundation-v0.1.md) | **`normativo`** (aceito em 2026-08-14) | [SPEC-8YVF0RR5](../specs/SPEC-8YVF0RR5-dmpf-rfc-limites-deps.md) / [ARQ-439](https://lider-cap.atlassian.net/browse/ARQ-439) |
| [upr-decision-mensagens.md](./upr-decision-mensagens.md) | **promovido para revisão** (adição à RFC pela âncora ANC-01, sem editá-la) | [SPEC-8MNDEWDP](../specs/SPEC-8MNDEWDP-dmpf-upr-decision-mensagens.md) / [ARQ-440](https://lider-cap.atlassian.net/browse/ARQ-440) |

A RFC obriga: as regras nela escritas valem para todo trabalho novo do DMPF.

Duas ressalvas ficam registradas na própria RFC. A revisão por área não chegou a
produzir parecer (RFC §14.6), e a promoção ocorreu com o inventário AS-IS ainda
em `baseline candidato` (RFC §1.5) — se a aprovação do inventário alterar
denominadores ou conclusões, as decisões que citam a evidência afetada devem ser
revisitadas. Para a `0.2` em diante, revisão por área e reconciliação com
baseline aprovado voltam a ser exigidas (RFC §14.3).

### Sobre `upr-decision-mensagens.md`

Adiciona à RFC pelo mecanismo de âncora (ANC-01, RFC §12.3), sob as regras de
monotonicidade M1–M4: detalha o bloco `domain library` sem editar a RFC e sem
incremento de versão. Três pendências ficam registradas no próprio artefato:

- a revisão dos exemplos por um representante de cada stack, gate externo não
  bloqueante (§10.3);
- os vetores de equivalência do desfecho e o diagnóstico estável por regra,
  encaminhados a FND-09 e ainda inexistentes (§10.4);
- a marcação de Parte-1 §§5 e 6 como consolidados na tabela de RFC §14.4, que o
  artefato não pode fazer sozinho (§1.5).

## Evidências do inventário

Há **13 relatórios** cobrindo **10 repositórios** (alguns monorepos têm mais de
um corte) em [`docs/specs/SPEC-K9H204F1/`](../specs/SPEC-K9H204F1/).
