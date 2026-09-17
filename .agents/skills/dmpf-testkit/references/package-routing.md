# Roteamento pelos packages

Leia somente a seção do package necessário. Em todos os casos, confirme as
assinaturas no código atual antes de implementar.

## Escolha rápida

| Necessidade | Package | Prova principal |
| --- | --- | --- |
| UPR, agregado, resposta, rejeição, eventos e estado | `domainkit` | projeção observável e determinismo |
| Caso de uso e fronteira transacional | `serviceskit` | ledger de UoW, escrita, outbox e commit |
| Realização de UoW, Inbox ou Outbox | `providerkit` | suíte de conformidade parametrizada |
| Contrato Protobuf/CloudEvents e bytes canônicos | `golden` | duas direções e três oráculos |
| Tempo, IDs e ordenação determinísticos | `clock`, `ids`, `stable` | seam controlável e comparação estável |
| Consumer adapter até Postgres e gesto do broker | `appkit` (`apps/backend/reservations`) | efeitos persistidos e Ack após commit |
| Redelivery com broker e isolamento real | `distkit` (`apps/backend/reservations`) | processos OS e `DMPF-R004` |
| Dependências e capacidades dos blocos | `fitness` | universo real e vetores sintéticos |
| Adaptação a `testing.TB`, fixtures e Postgres | `tb`, `tb/pg` | falha/skip explícito no ambiente de teste |

## `domainkit`

Autoridade de API:

- `domainkit/run.go`: `Subject[S,R]`, `Run`, `ReadTwice`;
- `domainkit/projection.go`: forma da projeção observável;
- `domainkit/verdict.go`: `Equal` e diagnósticos;
- `domainkit/fixture_test.go`: o agregado `counter` do próprio kit
  (`subjects_test.go`) dirigido pela fixture de projeção
  `testdata/counter.golden` — o molde para o teste de projeção do `domain` de
  cada contexto em `apps/backend`;
- `domainkit/violations_test.go`: red controls.

Use `tb.LoadProjection` para fixtures em
`contracts/fixtures/<ctx>/projection/v1/*.golden`. O codec permanece em `tb`
porque `encoding/json` não pertence ao bloco `domain`.

## `serviceskit`

Autoridade de API:

- `serviceskit/fakes.go`: `NewFakes`, adaptação da UoW, repository e publisher;
- `serviceskit/ledger.go`: gestos e posições;
- `serviceskit/verdict.go`: `Decide` e `Expect`;
- `serviceskit/verdict_test.go`: composição do service real e vetores negativos.

Execute o application service real sobre `Fakes.UnitOfWork`. Para uma recusa,
estabeleça o baseline imediatamente antes do comando recusado; do contrário o
veredicto pode atribuir efeitos aceitos anteriores ao caso atual.

## `providerkit`

Autoridade de API:

- `providerkit/uow.go`: `UnitOfWorkSubject` e `UnitOfWork`;
- `providerkit/inbox.go`: `InboxSubject` e `Inbox`;
- `providerkit/outbox.go`: `OutboxStore`, `OutboxSubject` e `Outbox`;
- `providerkit/memory_test.go`: candidato em memória;
- `postgres/conformance_test.go`: candidato Postgres.

A função passada à suíte cria e limpa um candidato para cada cláusula. Use
`ArmCommitFailure` somente quando a realização realmente consegue injetar a
falha; use `Concurrent` para declarar se a cláusula concorrente é exercitável.
Depois de `tb.Require`, confira a lista `Skipped` conforme a capacidade do
candidato.

## `golden`

Autoridade de API:

- `golden/fixture.go`: `Decode`, `StringScalars` e formato da fixture;
- `golden/catalog.go`: unicidade por contrato-major;
- `golden/roundtrip.go`: `Subject`, `Consumer`, `Producer` e `Evaluate`;
- `golden/oracle.go` e `report.go`: outcomes e relatório;
- `contracts/golden/golden_test.go`: uso contra contratos reais.

O módulo `contracts` continua dono das fixtures e do gerador. O testkit
carrega e julga. Consumidor roda casos canônicos e discriminadores; produtor
roda apenas casos canônicos. Atualização com `GOLDEN_UPDATE=1` exige revisão do
diff e não é uma correção automática de falha.

## `clock`, `ids` e `stable`

Confirme `clock/clock.go`, `ids/ids.go` e `stable/stable.go`. Na API atual:

- o relógio nasce com `clock.New(ports.Instant)`;
- uma sequência é `&ids.Sequence{Prefix: "..."}`; não presuma a existência de
  `ids.NewSequence`;
- `ids.NewSeeded` e `ids.NewClaimIDs` cobrem seed e IDs de claim;
- `stable.SortStrings`, `stable.SortBy` e `stable.Sequence[T]` removem ordem
  incidental das comparações.

## `appkit` (em `apps/backend/reservations/appkit`)

Autoridade de API:

- `apps/backend/reservations/appkit/harness.go`: `Harness`, `NewReservations`,
  `RawOrderPlaced`, `Ack` e `Effects`;
- `apps/backend/reservations/appkit/harness_test.go`: disposições, contenção,
  reentrega e Ack após commit.

O harness é uma composition root concreta do contexto `reservations` — unidade
`reservations/appkit`, bloco `app` —, não uma abstração genérica para qualquer
bounded context; por isso vive com o contexto e não no `testkit` (ADR-046).
Um contexto novo que precise do mesmo tipo de prova escreve o próprio harness
no molde deste, sobre `tb`, `tb/pg`, `clock` e `ids` do kit.

## `distkit` (em `apps/backend/reservations/distkit`)

Autoridade de API:

- `apps/backend/reservations/distkit/harness.go`: `New`, processos e espera;
- `apps/backend/reservations/distkit/roles.go`: papéis filhos;
- `apps/backend/reservations/distkit/verdict.go`: plano, efeitos e `DMPF-R004`;
- `apps/backend/reservations/distkit/harness_test.go`: positivo e consumer
  ingênuo.

Preserve `//go:build integration && distributed`. O processo pai inicia os
papéis `producer`, `consumer` e `consumer-naive` pelo próprio executável de
teste; por isso um fake compartilhado em memória invalida a prova. O target é
`reservations:test-distributed`.

## `fitness`

Autoridade de API:

- `fitness/cells.go` e `cells_test.go`: 36 células e pares positivo/negativo;
- `fitness/vectors.go`: `V13..V32`, `Lookup` e `Gaps`;
- `fitness/edges_test.go`: universo real;
- `fitness/domaintest_test.go`, `capabilities_test.go`, `v27_test.go` e
  `v31_test.go`: capacidades, assimetria e vedação de exactly-once.

O workspace real é construído pelo package exportado
`conformance/fitness`, com `Workspace(tb.RepoRoot(t), "")` seguido de
`Diagnostics`. Não presuma um helper `testkit/fitness.Workspace`.

## `tb` e infraestrutura

Use `tb.Require` para tipos que implementam `Failures() []string` e
`tb.RequireReport` para `golden.Report`. `tb.Env` faz skip local e falha no CI
quando falta a variável. `tb/pg.OpenPool` e `ResetTables` centralizam o acesso
Postgres; não replique bootstrap e limpeza em cada suíte.
