---
id: SPEC-C4JMX2WM
slug: normatizacao-bounded-contexts
title: DMPF — Normatização dos bounded contexts: layout, capacidades transversais e kits
stage: done
priority: P1
depends_on: []
ticket_url: null
subtask_urls: []
created: 2026-09-18
---

# SPEC-C4JMX2WM: Normatização dos bounded contexts

## Resumo

Os três bounded contexts de `apps/backend` divergem em layout de blocos, em
capacidades transversais e em harnesses de teste, e nenhum gate impede que a
divergência cresça. Esta spec fixa um layout canônico único, promove ao kernel o
código transversal hoje duplicado entre `orders` e `reservations`, dá a todo
contexto os kits de teste e a composition root que faltam, e cria o gate
mecânico que reprova contexto divergente.

## Contexto

### Problema

Levantamento sobre a árvore atual (2026-09-18) mediu três desvios independentes.

**1. Layout de blocos.** `bookings` põe o bloco `app` na subpasta `app/`, como o
generator emite (`tools/dmpf-plugin/src/generators/bounded-context/blocks.ts:52-118`,
`dirName: 'app'`) e como o guia e o ADR-045 fixam (`docs/guides/dmpf-composicao.md:84`,
`docs/adr/045-nomes-bare-e-contexto-em-modulo-unico.md:25`). `orders` e
`reservations` põem o mesmo bloco no package raiz, com `rpc/` e `cmd/` ao lado.
A documentação hoje descreve os dois layouts e chama ambos de `bookings`:
`docs/adr/046-libs-somente-kernel-de-reuso.md:38` afirma que `orders` e
`reservations` "seguem o layout do `bookings`: `domain/`, `application/`,
`provider/` e o bloco `app` no package raiz, com `rpc/` e `cmd/`" — descrição que
não corresponde ao `bookings` real, que não tem `rpc/` nem `cmd/`.

**2. Capacidades transversais duplicadas em dois contextos e ausentes no terceiro.**
`orders` e `reservations` carregam seis arquivos de raiz quase idênticos:

| arquivo | linhas (orders/reservations) | divergência medida |
|---|---|---|
| `telemetry.go` | 160 / 160 | 2 linhas (`package` e um import) |
| `dbtrace.go` | 40 / 40 | 1 linha (`package`) |
| `ports.go` | 39 / 39 | 2 linhas (`package` e a string do `panic`) |
| `config.go` | 258 / 256 | ~46%, proporcional ao número de papéis |
| `wiring.go` | 300 / 345 | ~35%, com 11 funções byte-idênticas |
| `catalog.go` | 80 / 105 | ~65%; `kafkaChannel` já existe em `reservations` e `orders` duplica o literal |

`bookings` não tem nenhum deles. Equalizar por cópia produziria um terceiro
clone de `telemetry.go`, `dbtrace.go` e `ports.go`.

**3. Lacunas reais do kernel expostas pela duplicação.** Não existe realização de
produção de `ports.Clock` nem de gerador de identificador: `testkit/ids/ids.go:36-39`
se declara "a test double, never a production generator", e a interface `ClaimIDs`
declarada em `libs/backend/go/app/relay/ports.go:44-49` — consumida por
`relay.go:62` — só tem realização nos `RandomClaimIDs` duplicados dos contextos.
`libs/backend/go/postgres` não tem `QueryTracer`. Nenhum ADR justifica a
duplicação; o critério para resolvê-la já existe e nunca foi aplicado a esses
arquivos: `docs/adr/046-libs-somente-kernel-de-reuso.md:40` promove ao kernel o
"genuinamente genérico" e mantém local o "harness/glue de um plano específico".

**4. Kits de teste só em um contexto.** `appkit/` e `distkit/` existem apenas em
`reservations`. `orders` e `bookings` reimplementam `openPool` à mão dentro de
arquivos `_test.go`, por impedimento de linguagem registrado em
`apps/backend/orders/provider/e2e_test.go:16-18`: "a `_test.go` file is never
importable, so `provider_test` cannot reuse `postgres_test`'s `openPool`".

**5. `bookings` enfileira na outbox e nada drena.**
`apps/backend/bookings/application/reserve_booking.go:42` e
`application/service.go:90-100` chamam `port.Outbox.Enqueue`, mas o contexto não
declara canal (`catalog.go` inexistente, nenhum `channel.Catalog`), não tem papel
`relay`, não tem `cmd/` e não tem target `serve*`. As entradas de outbox ficam
paradas.

**6. Nenhum gate cobre nada disso.** O verificador não valida presença de bloco
por bounded context — `U004` (`tools/dmpf-conformance/internal/rule/universe.go:68-74`)
só reprova módulo com código e zero manifesto, e `M002`
(`internal/rule/matrix.go:8-14`) valida `block` como valor individual. Não há
check de capacidades transversais. O layout de diretório não é objeto do
verificador: a chave canônica da unidade é o import path
(`internal/rule/universe.go:33-36`), nunca o nome da pasta.

### Impacto

Contexto novo nasce sem inicialização, sem harness e sem gate que o mantenha
alinhado; contexto existente diverge sem sinal. A documentação normativa aponta
para dois layouts com o mesmo nome, então o autor não tem resposta única.

### Links relevantes

- `docs/adr/041-sdk-de-referencia-generator-e-bom-certificado.md:17` — configuração só por ambiente com `exit 2`, papéis por flag, um runtime OTel por processo
- `docs/adr/044-bff-rest-e-contextos-grpc-de-referencia.md:5,20` — três composition roots, seis processos; preserva as decisões transversais do ADR-041
- `docs/adr/045-nomes-bare-e-contexto-em-modulo-unico.md:25,31` — um contexto é um módulo Go, um package por bloco
- `docs/adr/046-libs-somente-kernel-de-reuso.md:38,40` — contextos são apps; critério de promoção ao kernel
- `docs/guides/dmpf-composicao.md:45-47,84` — `bookings` é o golden; cinco subpastas por bloco
- `SPEC-VZ16X0MS:320-321` e `SPEC-H1A190Y8:397-398` — excluem `cmd/` do escopo do generator; esta spec reverte a exclusão

<constraints>
- [P0] NUNCA quebrar a regra de dependência DMPF: toda mudança mantém o verificador `conformance` aprovando (`DMPF-D001`/`D002`) sobre os imports reais.
- [P0] Toda criação, remoção ou remapeamento de unidade é mudança normativa e segue o rito de `docs/guides/dmpf-manifesto.md:224-234`: editar `dmpf-units.json` e `tools/dmpf-baseline/units-baseline.json`, em **commit próprio e separado do commit de código**, com revisor diferente do autor. O `conformance --write-baseline` é passo humano, nunca do agente (ADR-028). Divergência entre manifesto e baseline emite `DMPF-T001`; mudança sem autorização emite `DMPF-T002` (`tools/dmpf-conformance/internal/baseline/authorization.go:13-20`).
- [P0] NUNCA alterar comportamento observável dos seis processos da topologia de referência: a promoção ao kernel é refatoração, e o e2e caixa-preta do `bff` precisa passar antes e depois, sem alteração no teste.
- [P0] Preservar a configuração só por variável de ambiente, validada na partida com `exit 2` nomeando a ausente (ADR-041:17).
- [P1] Manter `nx affected` e o cache íntegros: não redeclarar target que `targetDefaults` ou o plugin já fornecem.
</constraints>

## Requisitos

### Funcionais

#### Layout canônico

- [ ] **[P0] Fixar o layout canônico de bounded context**: todo contexto em
  `apps/backend/<ctx>` tem `domain/`, `application/`, `provider/`, `app/`,
  `cmd/<ctx>/`, `appkit/` e `distkit/`; `ports/` existe apenas quando o contexto
  declara porta própria que os genéricos do kernel não expressam.
- [ ] **[P0] Migrar `orders` e `reservations` para o layout canônico**: mover para
  `app/` o que restar na raiz após a promoção (`config.go`, `wiring.go`,
  `catalog.go`, `doc.go`) e mover `rpc/` para `app/rpc/`.
- [ ] **[P0] Manter `bookings` no layout canônico**: `app/` já está correto; mover a
  borda HTTP para `app/http/` quando `app/rpc/` passar a existir em outros contextos.
- [ ] **[P1] Corrigir `docs/adr/046-...:38`**: o texto descreve o layout de
  `orders`/`reservations` e o atribui a `bookings`. Registrar a correção no ADR novo.

#### Promoção ao kernel

- [ ] **[P0] Promover `dbtrace.go` para `libs/backend/go/postgres`**: mover
  `dbSpanKey`, `dbTracer`, `TraceQueryStart` e `TraceQueryEnd` sem parametrizar, e
  declarar `go.opentelemetry.io/otel/trace` no `external[]` do `dmpf-units.json`
  do módulo (hoje só declara `io.storage` e `wire.codec`).
- [ ] **[P0] Promover as realizações de produção de `ports.Clock` e dos geradores
  de identificador**: `SystemClock`, `RandomClaimIDs` e `RandomMessageIDs` vão ao
  kernel, parametrizando `randomHex` pelo componente que nomeia o processo na
  mensagem de `panic`. Isso preenche a interface `ClaimIDs` de
  `libs/backend/go/app/relay/ports.go:44-49`, hoje sem realização de produção.
- [ ] **[P0] Promover os helpers de leitura de ambiente**: `orDefault`, `hostname`,
  `splitList`, `parseBool` e `parsePositive` — byte-idênticos — mais o laço de
  validação que nomeia a variável ausente. O contexto segue declarando quais
  variáveis cada papel exige.
- [ ] **[P0] Promover `telemetry.go` para `libs/backend/go/observability`**:
  `auditRecord`, `classify` e `subject` vão como estão; `NewTelemetry`,
  `requestFields`, `NewAuditSink` e `Emit` recebem como parâmetros o tenant e a
  função de campos. Preservar a decisão de modo de desenvolvimento (sem
  `DMPF_OTLP_ENDPOINT`, exportador em memória) e o envelope enriquecido que faz os
  dois canais caírem juntos no Loki (ADR-041:74).
- [ ] **[P0] Promover os helpers de canal para `libs/backend/go/transport`**:
  `kafkaChannel`, `NewKafkaConfig` e `EventTypeOf`; `orders` passa a chamar
  `kafkaChannel` em vez de duplicar o literal (`orders/catalog.go:30-47`).
- [ ] **[P0] Promover as onze funções genéricas de `wiring.go`**: `Run`, `NewPool`,
  `NewAdmission`, `serverTLS`, `apiServerConfig`, `healthServices`, `serveGRPC`,
  `drainGRPC`, `NewRelay`, `shutdownTelemetry` e `assertOwnOutbox`. Permanecem no
  contexto o dispatch de papel (`RunWith`, `serveAPI`) e o construtor do serviço
  tipado.
- [ ] **[P0] Fazer o `bff` consumir o que for promovido**: os helpers de leitura de
  ambiente estão triplicados, não duplicados — `bff/config.go:126-150` tem
  `orDefault`, `hostname`, `splitList` e `parseBool`. O `bff` passa a consumir os
  helpers promovidos e as funções genéricas que lhe couberem, sem alterar o layout
  dele, para que não sobre clone do código que esta spec deduplica.
- [ ] **[P1] Registrar o critério de promoção**: o ADR novo declara que o critério
  do ADR-046:40 se aplica às capacidades transversais, e não apenas ao harness.

#### Composition root de `bookings`

- [ ] **[P0] Dar a `bookings` config, wiring, telemetria e catálogo** no layout
  canônico, consumindo o kernel — sem recriar localmente o que foi promovido.
- [ ] **[P0] Criar `apps/backend/bookings/cmd/bookings/main.go`** no padrão dos
  demais: `exitOK`/`exitFailure`/`exitUsage`, `signal.NotifyContext` com `SIGTERM`
  e `SIGINT`, configuração por `FromEnv` devolvendo `exitUsage` quando faltar
  variável.
- [ ] **[P0] Declarar os papéis de `bookings`**: `--role api|relay`. O papel `api`
  serve a borda HTTP existente; o papel `relay` drena a outbox que hoje ninguém
  drena.
- [ ] **[P0] Declarar o canal de `bookings`** em `app/catalog.go`, com destino,
  DLQ e grupo, para que o relay tenha para onde drenar.
- [ ] **[P0] Adicionar os targets `serve-api` e `serve-relay`** ao
  `apps/backend/bookings/project.json`.

#### Kits em todo contexto

- [ ] **[P0] Criar `appkit/` em `orders` e em `bookings`**, no padrão de
  `reservations/appkit`: unidade `<ctx>/appkit`, bloco `app`, mesmo
  `bounded_context`, build tag `integration`, com abertura de pool, contagem de
  efeitos por tabela e construção de entrada.
- [ ] **[P0] Eliminar o `openPool` duplicado**: `orders/provider/testing_test.go` e
  `bookings/provider/testing_test.go` passam a consumir o `appkit` do próprio
  contexto, em vez de reimplementar a função.
- [ ] **[P0] Criar `distkit/` em `orders` e em `bookings`**, com build tag
  `integration && distributed` e target `test-distributed`. O vetor provado
  depende do papel: contexto com consumidor prova reentrega sem duplicar efeito
  (`DMPF-R004`, como `reservations`); contexto apenas produtor prova que o relay
  drena a outbox uma única vez e preserva o `payload_hash`.
- [ ] **[P1] Padronizar o consumo do `testkit` do kernel**: todo `appkit` usa
  `tb/pg` para o pool e `clock`/`ids` para determinismo, que já são superfície
  pública (`public_integration_surface: true`).

#### Generator e gate

- [ ] **[P0] Estender o generator `bounded-context`** para emitir o layout canônico
  completo: blocos, `cmd/<ctx>/main.go`, os arquivos de `app/` que consomem o
  kernel, `appkit/`, `distkit/`, as unidades no `dmpf-units.json` e os targets
  `serve-*` e `test-distributed` no `project.json`.
- [ ] **[P0] Criar o gate de estrutura de contexto**: script em `tools/` que
  reprova contexto em `apps/backend` sem as pastas canônicas, sem `cmd/`, sem as
  unidades `<ctx>/appkit` e `<ctx>/distkit` no manifesto, ou sem os targets
  `serve-*`. Generalizar `conferir_golden_presente()` de
  `tools/dmpf-harness-check.sh`, hoje restrito ao golden.
- [ ] **[P0] Ligar o gate ao CI**: o script roda em `.github/workflows/ci.yml` como
  os demais gates de `tools/`, e reprova o job.
- [ ] **[P1] Provar o gate por vetor negativo**: o script tem modo de autoteste que
  remove uma pasta ou unidade em cópia descartável e exige a reprovação, no padrão
  de `tools/dmpf-cell-check.sh`.
- [ ] **[P1] Atualizar `AGENTS.md`**: o inventário descreve `bookings` como sem
  `cmd/` e sem target `serve`, o que deixa de valer.

### Não-funcionais

- [ ] **Compatibilidade**: a topologia de referência segue com seis processos e dois
  bancos; nenhuma variável `DMPF_*` existente muda de nome ou de semântica.
- [ ] **Desempenho**: o e2e caixa-preta do `bff` não regride em tempo de execução
  além de 10% da medição atual.
- [ ] **Rastreabilidade**: cada promoção ao kernel é commit próprio de código,
  seguido de commit separado de classificação (manifesto e baseline), conforme o
  rito de `docs/guides/dmpf-manifesto.md:224-234`.
- [ ] **Determinismo**: a evidência de release (`testkit:evidence`) continua
  reproduzível byte a byte após a reorganização.

## Localização de código

**Contextos** — `apps/backend/{bookings,orders,reservations}/`: `app/`
(config, wiring, catalog, doc, rpc, http), `cmd/<ctx>/main.go`, `appkit/`,
`distkit/`, `dmpf-units.json`, `project.json`.

**Kernel** — `libs/backend/go/postgres/` (tracer de query),
`libs/backend/go/observability/` (telemetria, relógio e identificadores de
produção, leitura de ambiente), `libs/backend/go/transport/` (helpers de canal),
`libs/backend/go/app/` (funções genéricas de composition root).

**Tooling** — `tools/dmpf-plugin/src/generators/bounded-context/` (generator),
`tools/` (gate de estrutura), `tools/dmpf-baseline/units-baseline.json`
(baseline), `.github/workflows/ci.yml` (execução do gate).

**Documentação** — `docs/adr/` (ADR novo), `docs/guides/dmpf-composicao.md`,
`AGENTS.md`.

## Design

### Layout canônico

```text
apps/backend/<ctx>/
  domain/              agregados e UPRs
  ports/               apenas quando há porta própria
  application/         casos de uso
  provider/            realização Postgres, schema e mapeadores
  app/                 config.go  wiring.go  catalog.go  doc.go
      rpc/             transporte gRPC, quando há
      http/            borda REST, quando há
  cmd/<ctx>/main.go    composition root com --role
  appkit/              harness borda a borda sobre Postgres
  distkit/             harness distribuído sobre broker
  go.mod  project.json  dmpf-units.json
```

### Ordem de execução

A promoção precede a migração de layout, para que o código mova uma única vez.

1. Promover ao kernel, contexto a contexto, mantendo a raiz onde está.
2. Regravar o baseline e provar o verificador.
3. Migrar o que restou da raiz para `app/` nos três contextos.
4. Dar a `bookings` a composition root, o canal e os papéis.
5. Criar `appkit` e `distkit` onde faltam.
6. Estender o generator e criar o gate.
7. Atualizar documentação e registrar o ADR.

### Viabilidade pela matriz de células

Os blocos `provider` e `app` são permissivos quanto a capability
(`tools/dmpf-conformance/internal/rule/capability.go:51-64`), e `provider→provider`
é permitida (`internal/rule/matrix.go:54,61`). Nenhuma promoção desta spec exige
ampliar a política de capability; apenas `dbtrace.go → postgres` exige uma entrada
nova em `external[]`. O bloco `app` é o único que alcança os seis blocos, o que
sustenta `appkit` e `distkit` como unidades de bloco `app`.

## Decisões técnicas

**Layout em subpasta `app/`, não na raiz.** Alinha com o que o generator já emite,
com `docs/guides/dmpf-composicao.md:84` e com o ADR-045:25, e evita emendar os
três para acomodar a raiz. Após a promoção, restam três arquivos curtos por
contexto, o que torna a migração de `orders` e `reservations` pequena.

**Promoção completa, incluindo `wiring.go`.** As onze funções têm corpo
byte-idêntico e só tocam tipos do kernel. Deixá-las fora produziria um terceiro
clone quando `bookings` ganhasse a composition root.

**`ports/` condicional, não obrigatório.** Mantém a decisão do ADR-046:38: quando
as portas são todas do kernel, não há unidade a classificar. A RFC exige `block`
por unidade existente, não um bloco por contexto
(`docs/dmpf/rfc-dmpf-foundation-v0.1.md:1252`).

**`distkit` com vetor conforme o papel.** Exigir `DMPF-R004` de contexto sem
consumidor seria exigir prova de comportamento inexistente. O gate cobra a
presença do kit; o vetor provado acompanha o papel declarado.

**Spec independente das deferred do generator.** `SPEC-8FSD8505`, `SPEC-VZ16X0MS` e
`SPEC-F7S5B6KV` desenham cinco módulos Go irmãos por contexto, fundação
substituída pelo ADR-045:31. Declarar `depends_on` para elas bloquearia esta spec
por tempo indeterminado, já que a condição de retomada registrada é outra: "pode
ser retomada se a geração por agentes não sustentar os gates"
(`SPEC-8FSD8505:17-22`).

**Reversão explícita de escopo.** `SPEC-VZ16X0MS:320-321` e `SPEC-H1A190Y8:397-398`
excluem `cmd/` do generator. Esta spec reverte a exclusão e registra a reversão no
ADR, para não deixar decisões contraditórias sem rastro.

## Verificação e testes

### Critérios de aceite

- [x] Os três contextos têm a mesma árvore de pastas, conferida pelo gate novo.
- [x] `telemetry.go`, `dbtrace.go` e `ports.go` não existem mais na raiz de nenhum contexto.
- [x] Das onze funções genéricas de `wiring.go`, dez saíram dos contextos para o
  kernel. `Run` permanece local nos três: é o adaptador de quatro linhas que fecha
  sobre o `Config` e o `RunWith` de cada contexto, e o que ele tem de genérico já
  é `boot.Boot`. Promovê-lo exigiria genéricos sobre o `Config`, custo que a
  duplicação de quatro linhas não paga.
- [x] O kernel tem a realização de produção de `ClaimIDs` —
  `observability/idclock.RandomClaimIDs`, ligada por `app/relay/postgres.go` —, e
  nenhum contexto a reimplementa.
- [x] `apps/backend/bookings/cmd/main.go` existe e aceita `--role api|relay`
  (caminho fixado pelo ADR-048 e conferido pelo gate de estrutura).
- [x] `project.json` de `bookings` declara `serve-api`, `serve-relay` e `test-distributed`.
- [x] `bookings` declara canal em `app/catalog.go` e o relay drena a outbox.
- [x] Os três contextos declaram as unidades `<ctx>/appkit` e `<ctx>/distkit` no `dmpf-units.json`.
- [x] `openPool` não aparece reimplementado em nenhum `_test.go` de contexto.
- [x] O generator emite o layout canônico completo, e a saída passa os gates sem edição manual.
- [x] O gate de estrutura roda no CI e reprova contexto divergente em modo de autoteste.
- [x] `go run ./tools/dmpf-conformance/cmd/conformance --root . --base develop` aprova.
- [x] `pnpm nx run-many -t fmt-check,vet,lint,typecheck,test,build` passa.
- [x] O e2e caixa-preta do `bff` passa sem alteração no arquivo de teste.
- [x] `AGENTS.md`, `docs/guides/dmpf-composicao.md` e o ADR novo descrevem um único layout.

### Cenários de teste

**Cenário 1 — caminho feliz: contexto novo nasce conforme**
DADO um contexto criado pelo generator estendido
QUANDO a cadeia de gates roda sobre a árvore gerada
ENTÃO o contexto tem as pastas canônicas, `cmd/`, `appkit/`, `distkit/` e os
targets `serve-*`, e o gate de estrutura, o verificador e a cadeia Go aprovam sem
edição manual.

**Cenário 2 — borda: contexto sem porta própria**
DADO um contexto cujas portas são todas do kernel, sem `ports/`
QUANDO o gate de estrutura roda
ENTÃO o gate aprova, porque `ports/` é condicional, e o verificador não emite
diagnóstico de unidade ausente.

**Cenário 3 — erro: contexto divergente é reprovado**
DADO um contexto em cópia descartável do qual se remove `appkit/` ou o target
`serve-api`
QUANDO o gate de estrutura roda em modo de autoteste
ENTÃO o gate reprova nomeando o contexto e o item ausente, e o job do CI falha.

**Cenário 4 — regressão: a promoção não muda comportamento**
DADO os seis processos da topologia de referência antes da promoção
QUANDO a promoção ao kernel e a migração de layout são aplicadas
ENTÃO o e2e caixa-preta do `bff` passa sem alteração no arquivo de teste, e a
cadeia de contexto até `ReservationConfirmed` segue provada.

**Cenário 5 — erro: mudança normativa sem autorização**
DADO a criação das unidades `<ctx>/appkit` e `<ctx>/distkit`
QUANDO o baseline não é regravado no commit
ENTÃO o verificador emite `DMPF-T002` e o CI reprova.

**Cenário 6 — borda: relay de `bookings` drena a outbox**
DADO `bookings` com o papel `relay` e canal declarado
QUANDO uma reserva é aceita e enfileira entrada na outbox
ENTÃO o relay drena a entrada uma única vez, preservando o `payload_hash`, e o
`distkit` do contexto prova o vetor de publicação.

<critical_constraints>
- [P0] NUNCA quebrar a regra de dependência DMPF: toda mudança mantém o verificador `conformance` aprovando (`DMPF-D001`/`D002`) sobre os imports reais.
- [P0] Toda criação, remoção ou remapeamento de unidade é mudança normativa e segue o rito de `docs/guides/dmpf-manifesto.md:224-234`: manifesto e baseline em **commit próprio e separado do commit de código**, com revisor diferente do autor; `--write-baseline` é passo humano, nunca do agente (ADR-028).
- [P0] NUNCA alterar comportamento observável dos seis processos da topologia de referência: a promoção ao kernel é refatoração, e o e2e caixa-preta do `bff` precisa passar antes e depois, sem alteração no teste.
- [P0] Preservar a configuração só por variável de ambiente, validada na partida com `exit 2` nomeando a ausente (ADR-041:17).
- [P1] Manter `nx affected` e o cache íntegros: não redeclarar target que `targetDefaults` ou o plugin já fornecem.
</critical_constraints>

## Escopo fora

- **Reativar as specs deferred do generator determinístico** (`SPEC-8FSD8505`,
  `SPEC-VZ16X0MS`, `SPEC-F7S5B6KV`): partem de cinco módulos Go irmãos por
  contexto, fundação substituída pelo ADR-045:31, e a condição de retomada
  registrada é outra.
- **Gerar código de negócio por agregado a partir de DSL**: o generator desta spec
  emite o esqueleto e as capacidades transversais; o conteúdo de domínio segue a
  cargo do harness de agentes (`SPEC-VDP9XX65`).
- **Alterar a topologia de processos**: seguem os seis processos e dois bancos do
  ADR-044; esta spec não cria nem remove processo, apenas dá a `bookings` os
  papéis que faltavam.
- **Promover ao kernel o que diverge por papel**: `requirements()` por papel,
  `RunWith`, `serveAPI` e os construtores de serviço tipado permanecem no
  contexto, porque dependem do domínio.
- **Unificar o layout do `bff` com o dos contextos**: o `bff` não tem domínio,
  banco nem outbox, e segue com layout próprio; o gate de estrutura o trata como
  caso à parte. O `bff` entra nesta spec apenas como consumidor do que for
  promovido ao kernel, sem reorganização de pastas.
- **Endurecimento de borda e kernel** (`SPEC-Z2HM6NAP`): amostragem, catálogo
  multi-evento e orçamento operacional são outro recorte.
