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

A RFC obriga: as regras nela escritas valem para todo trabalho novo do DMPF.

Duas ressalvas ficam registradas no próprio documento. A revisão por área não
chegou a produzir parecer (§14.6), e a promoção ocorreu com o inventário AS-IS
ainda em `baseline candidato` (§1.5) — se a aprovação do inventário alterar
denominadores ou conclusões, as decisões que citam a evidência afetada devem ser
revisitadas. Para a `0.2` em diante, revisão por área e reconciliação com
baseline aprovado voltam a ser exigidas (§14.3).

## Evidências do inventário

Há **13 relatórios** cobrindo **10 repositórios** (alguns monorepos têm mais de
um corte) em [`docs/specs/SPEC-K9H204F1/`](../specs/SPEC-K9H204F1/).
