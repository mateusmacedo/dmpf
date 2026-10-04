# ADR-058: Revogar os controles que exigem uma segunda pessoa e o rito derivado deles

## Status

Aceito — 2026-10-04. Supersede o [ADR-028](./028-processo-de-autorizacao-da-classificacao.md). Supersede parcialmente o [ADR-013](./013-autoridade-sobre-classificacao.md), o [ADR-027](./027-bom-combinacao-certificada-compatibilidade-e-escape-hatch.md), o [ADR-029](./029-titular-da-revisao-de-seguranca-fnd-07.md), o [ADR-031](./031-verificador-de-conformidade-dmpf-em-go.md), o [ADR-033](./033-adaptacoes-monorepo-dos-contratos-wire.md), o [ADR-042](./042-shared-kernel.md) e o [ADR-054](./054-apps-autocontidos-e-infras-separadas.md), nos pontos listados na Decisão.

## Contexto

O projeto tem um mantenedor só. Parte da governança herdada da RFC DMPF supõe uma equipe: a autorização tem de vir de alguém diferente do autor, e o gate reprova quando autor e autorizador coincidem. Num projeto solo essa condição nunca se cumpre, e o controle deixa de proteger para virar bloqueio ou formulário:

- **Marca de baseline do Buf (BUF-08).** O `buf-breaking` exigia a tag anotada `contracts-baseline/<projeto>` criada por alguém que não fosse o autor do primeiro commit do módulo. Os quatro módulos de contrato têm o mesmo autor, então todo PR que os afetava reprovava por "estado inválido" — o PR #6 entrou em `develop` assim, e o PR #7 parou no mesmo ponto.
- **Commit próprio para mudança normativa (`DMPF-T002`).** O verificador reprovava o commit que misturasse `dmpf-units.json` ou o baseline com código. A regra existia para que um revisor distinto aprovasse a classificação isolada. Sem esse revisor, ela só obriga a reescrever o histórico: o PR #7 teve de ser rebaseado para separar três manifestos dos commits de código.
- **Aprovação em dobro preenchida pela mesma pessoa.** A exceção com convergência `review` exigia `approved_by: [arquitetura, plataforma]`, e a entrada certificada do BOM exigia `promoted.reviewed_by`. No BOM `0.1.0`, `by` e `reviewed_by` valem `mateusmacedo` em todas as entradas; os dois papéis da exceção também são a mesma pessoa.
- **Processos que só existem no papel.** A Autoridade de Classificação Arquitetural com titular, dois aprovadores distintos do autor e o gate G2 (ADR-028, `AUT-01`..`AUT-10` da governança), a aprovação do PR por revisor distinto (ADR-013, ADR-031) e a re-revisão de Segurança por titular independente (ADR-029) dependem de pessoas que o projeto não tem.

## Decisão

**Sai do projeto todo controle que exige a ação de uma segunda pessoa, e sai junto o rito que só existia para servir a esses controles. Ficam as verificações mecânicas que pegam erro real.**

1. **BUF-08 sem marca.** O estado de cada módulo vem só de `NX_BASE`: módulo com pacote publicado na base tem `buf breaking` obrigatório contra ela; módulo sem pacote publicado fica `sem baseline`, com o breaking dispensado e aviso na saída (`tools/buf-gate.sh`). Não há tag `contracts-baseline/*` nem conferência de `tagger`. O pacote publicado que some de todos os módulos continua reprovando. A tag legada `contracts-baseline/proto` fica no remoto, sem efeito.
2. **`DMPF-T002` revogado.** O verificador não lê mais o intervalo em revisão: saíram a flag `--base` do `conformance`, a comparação do baseline anterior com o atual e a leitura dos commits (`BaselineEm` e `CommitsQueTocaram`). O código segue no conjunto fechado de §10.3 como ausência declarada (`Applicable: false`), como o `DMPF-E004`, para que a tabela continue igual à da RFC. Manifesto, baseline e código podem ir no mesmo commit.
3. **Sem aprovação em dobro.** A convergência `review` de uma exceção é `{kind, review_by, replanning_condition}`: o campo `approved_by` saiu do schema, dos manifestos e da admissão (`internal/exception/admit.go`). O ato de `BOM-05` é `promoted {by, pr}`: o `reviewed_by` saiu do schema e da validação. O BOM `0.1.0` publicado continua com o campo, ignorado na leitura.
4. **Processos revogados.** Deixam de valer a Autoridade de Classificação e o rito do ADR-028 (`AUT-01`..`AUT-10` e o gate G2 da governança), a aprovação por revisor distinto do autor (ADR-013, ADR-031, ADR-042), a re-revisão por titular independente condicionada à criação de uma área de Segurança (ADR-029) e a criação da marca de baseline por segunda pessoa (ADR-033, ADR-054).
5. **O que fica.** O baseline de classificação e o `DMPF-T001`, que pegam divergência acidental entre manifesto e baseline; o `buf breaking` contra `NX_BASE`; o prazo e a data de revisão das exceções; e a regra de que o agente nunca regrava o baseline por conta própria — quem roda `--write-baseline` é o mantenedor.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Criar as tags e as aprovações com outra identidade git do mesmo mantenedor | Passaria na comparação de e-mail, mas o controle viraria teatro: a mesma pessoa nos dois papéis, só que escondida |
| Rebaixar `DMPF-T002` e BUF-08 a aviso | Aviso sem ação possível vira ruído que ensina a ignorar a saída do gate |
| Tirar `DMPF-T002` do conjunto fechado de §10.3 | Quebraria o alinhamento com a tabela da RFC; a ausência declarada já tem precedente no `DMPF-E004` |
| Manter `reviewed_by` e `approved_by` como campos opcionais | Campo sem semântica e sem verificação só acumula dado que ninguém lê |

## Consequências

**Positivas:**

- O CI deixa de travar em pré-requisito organizacional: os quatro contratos voltam a passar pelo `buf breaking` contra a base em todo PR.
- A classificação muda no mesmo commit do código que a motiva, sem reescrever o histórico.
- Manifestos, BOM e guias perdem campos e passos que o mantenedor preenchia para si mesmo.

**Negativas:**

- Não há revisão independente da classificação nem da quebra de contrato. A proteção que sobra é mecânica: o `DMPF-T001` acusa a divergência acidental, e o `buf breaking` acusa a quebra de wire. Uma reclassificação deliberada e coerente passa sem segundo olhar.
- O texto normativo da RFC, da governança e da norma Buf em `docs/dmpf/` continua descrevendo os controles revogados. Cada um desses documentos termina com uma nota que aponta para este ADR: a nota fica no fim porque o `dmpf-verify` ancora definições e referências por número de linha, e uma inserção no meio deslocaria as âncoras.
- Se o projeto ganhar um segundo mantenedor, a decisão precisa ser revista num ADR novo, em vez de religar os controles removidos.

## Referências

- RFC DMPF §10.2 (T4–T6) e §10.3 — `docs/dmpf/rfc-dmpf-foundation-v0.1.md`
- Governança, §5.2 (`AUT-01`..`AUT-10`) — `docs/dmpf/governanca-bom-pilotos.md`
- `tools/buf-gate.sh`, `tools/dmpf-conformance/README.md`, `libs/backend/go/contracts/README.md`, `bom/README.md`
