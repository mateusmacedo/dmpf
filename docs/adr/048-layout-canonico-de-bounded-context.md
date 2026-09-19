# ADR-048: Layout canônico de bounded context, kernel de composição e gate de estrutura

## Status

Aceito — 2026-09-19. Evolui o ADR-045 e o ADR-046 e supersede parcialmente os dois: do ADR-046, a frase "cada um segue o layout do `bookings`: (…) o bloco `app` no package raiz" — o bloco `app` passa a ser subpasta `app/`, e a raiz do contexto deixa de ter código Go — e a exclusão de `cmd/` e de target `serve` em `bookings`, que é revertida. Do ADR-045, o layout `cmd/<binário>/main.go`, que passa a `cmd/main.go`. O nome bare, o alias pelo papel, o módulo único por contexto (ADR-045) e o critério de consumo que reserva `libs/backend/go` ao kernel (ADR-046) permanecem.

Implementa [SPEC-C4JMX2WM](../specs/SPEC-C4JMX2WM-normatizacao-bounded-contexts.md).

## Contexto

Os três bounded contexts de `apps/backend` divergiam entre si em quatro eixos, e nenhum gate impedia que a divergência crescesse.

**Layout.** `orders` e `reservations` mantinham seis arquivos no package raiz do módulo — `config.go`, `wiring.go`, `catalog.go`, `telemetry.go`, `doc.go` e o consumidor — com a borda em `rpc/` ao lado. `bookings`, que o generator emitiu, punha o bloco `app` em subpasta `app/` e a borda HTTP solta dentro dela. O generator, o guia de composição (`dmpf-composicao.md`) e o ADR-045 descreviam a subpasta; os dois contextos mais antigos, a raiz.

**Capacidades transversais duplicadas.** Onze funções eram byte-idênticas entre `orders` e `reservations`: `NewPool`, `assertOwnOutbox`, `serveGRPC`, `drainGRPC`, `shutdownTelemetry`, `NewRelay`, `healthServices`, `serverTLS`, `apiServerConfig`, `NewAdmission` e `NewKafkaConfig`. Os cinco helpers de leitura de ambiente (`orDefault`, `hostname`, `splitList`, `parseBool`, `parsePositive`) estavam **triplicados** — o `bff` também os tinha. `dbtrace.go` diferia apenas no nome do package.

**Composition root ausente.** `bookings` enfileirava na outbox sem que nada drenasse: não tinha `config.go`, `catalog.go`, `wiring.go`, binário nem target. A exclusão estava registrada como decisão ("é borda HTTP e outbox, sem binário"), mas deixava o golden do harness sem exercitar metade do que um contexto faz.

**Kits de teste.** `appkit` e `distkit` existiam só em `reservations`. `orders` e `bookings` reimplementavam `openPool` à mão em cada package de teste, com um comentário que registrava o impedimento: "duplicado de propósito: um `_test.go` nunca é importável, então `provider_test` não pode reusar o `openPool` de `postgres_test`".

## Decisão

### O layout canônico

Todo bounded context em `apps/backend/<ctx>` tem:

```text
<ctx>/
  domain/        application/      provider/
  ports/         (condicional)
  app/           o bloco app; a borda do transporte em subpacote
    rpc/  ou  http/
  appkit/        distkit/
  cmd/main.go
```

A **raiz do contexto não tem código Go** — só `go.mod`, `project.json`, `dmpf-units.json`, `README.md` e `Dockerfile`. A borda do transporte fica em **subpacote de `app/`**: `app/rpc/` nos contextos gRPC, `app/http/` no de borda HTTP. O package da borda HTTP chama-se `httpedge`, não `http`, porque `http` colidiria com `net/http` em todo arquivo que importe os dois.

`ports/` permanece condicional, como o ADR-046 fixou: um contexto só o tem quando declara consulta que os genéricos do kernel não expressam. `bookings` o tem; `orders` e `reservations`, não.

### O binário é `cmd/main.go`, sem subdiretório

`apps/backend/orders/cmd/orders/main.go` repetia o nome do contexto sem ganho: cada contexto tem **um** binário, e os papéis vêm de `--role`. O layout passa a `cmd/main.go` nos quatro. O Dockerfile já nomeia o artefato por `-o`, e `go run ./cmd` funciona igual.

### `bookings` ganha composition root

A exclusão registrada em ADR-046 é revertida. `bookings` recebe `app/config.go`, `app/catalog.go`, `app/wiring.go`, `app/telemetry.go`, `cmd/main.go` e os targets `serve-api` e `serve-relay`. O motivo é o papel dele: `bookings` é o **golden** contra o qual a regeneração compara, e um golden que não exercita o relay não prova o relay. A dívida apareceu na primeira execução — o `Config` declarava o campo `Relay` e nunca o preenchia, e o papel recusava a partida com `invalid configuration: source`; nenhum gate havia acusado, porque o processo nunca tinha subido.

### O que sobe ao kernel, e o que fica

Sobe o que é **igual por natureza** e desce o que é **igual por coincidência**. Onze promoções, cada uma no módulo cuja responsabilidade ela já era:

| Promovido | Destino | Forma |
|---|---|---|
| `dbTracer` | `postgres.NewQueryTracer` | construtor, struct privada |
| `NewPool`, `assertOwnOutbox` | `postgres` | `AssertOwnOutbox` recebe `[]string` |
| `serveGRPC`, `drainGRPC` | `grpc.Serve`, `grpc.Drain` | assinatura preservada |
| `healthServices`, `serverTLS`, `apiServerConfig` | `grpc` | `APIServerConfig` recebe struct nomeado |
| `NewAdmission` | `admission.NewController` | tenant e limits explícitos |
| `NewRelay` | `relay.NewOverPostgres` | componente por parâmetro |
| `kafkaChannel`, `EventTypeOf` | `transport/channel` | com `InlineAttempts` e `RetentionByTime` |
| `NewKafkaConfig` | `kafka.NewConfig` | brokers, service e insecure explícitos |
| helpers de ambiente | `observability/envconfig` | sentinel `ErrInvalidVariable` no kernel |
| `SystemClock`, geradores de ID | `observability/idclock` | componente por construtor |
| `NewTelemetry`, `shutdownTelemetry` | `observability/boot`, `otelboot` | `Boot` e `ShutdownGracefully` |

Fica no contexto o que **diverge por papel**: `requirements()`, `RunWith`, `serveAPI`, `classify`, `subject` e os construtores de serviço tipado.

Três destinos mudaram em relação ao planejado, por impossibilidade técnica verificada:

1. **`AssertOwnOutbox` recebe `[]string`, não `channel.Catalog`.** A função só lia as chaves do mapa, e a assinatura original faria o provider de storage depender do módulo de transporte para isso.
2. **`shutdownTelemetry` virou método de `Runtime`, e `ShutdownGrace` foi para o raiz de `observability`.** O `grpc` já depende de `observability`, então `observability` não pode importar `grpc`.
3. **`Boot` e `StartTelemetry` vivem em `observability/boot`, package novo.** `otelboot/otlp` importa `otelboot`, e `otelboot` importa o raiz: nenhum dos dois alcança exportadores e log handler ao mesmo tempo.

Uma promoção foi **revertida**: `classify` e `subject` para `observability/usecase`. O verificador reprovou com `DMPF-D001` — a unidade `kernel/observability` é bloco `provider`, e provider não depende de `application`. O `go.mod` do módulo já trazia `application`, o que enganou a análise; a regra é por bloco, não por módulo. As duas voltaram ao bloco `app` de cada contexto, onde a dependência é legítima.

### Kits obrigatórios e o kit de Postgres extensível

Todo contexto tem `appkit/` e `distkit/`. O harness difere pelo papel: um contexto que **consome** injeta redelivery deliberada e decide `DMPF-R004`; um que só **produz** não tem inbox para decidir isso, e prova outra coisa — dois relays competindo por uma outbox publicam cada registro **uma** vez, e o payload que chega ao tópico ainda hasheia ao que a outbox guardou. Os diagnósticos do produtor são próprios: `DMPF-P001` (drenado duas vezes), `DMPF-P002` (liquidado e nunca publicado) e `DMPF-P003` (payload alterado).

Para eliminar o `openPool` reimplementado, `pg.OpenPool` e `pg.ResetTables` passam a aceitar as tabelas do contexto por variádico — mudança retrocompatível, porque o kit não pode conhecer um schema que não é dele.

### O gate

`tools/dmpf-context-check.sh` descobre contexto pela presença de `domain/`: um contexto tem domínio, uma borda como o `bff` não tem. Isso separa os dois sem lista fixa e passa a valer para contexto novo no dia em que ele nasce. O gate verifica os packages de bloco, a raiz sem código, o binário, a borda em subpacote, os dois kits, as duas unidades de kit no manifesto e os três targets. A fase `self-test` sabota a fixture em sete pontos e exige recusa em cada um — sem isso, um gate que parasse de morder passaria por gate.

O generator emite o layout completo, e `tools/dmpf-generator-check.sh` prova que o esqueleto compila e é conforme.

## Consequências

**O `observability` dilui.** Ele já era o módulo mais largo do kernel e ganha três packages (`envconfig`, `idclock`, `boot`). A alternativa — um módulo por capacidade — custaria três `go.mod`, três `project.json` e três entradas de baseline para código que nenhum contexto consome isoladamente. A diluição é aceita; o critério para reverter é o dia em que um consumidor quiser `envconfig` sem querer OpenTelemetry.

**Quatro atos de classificação.** A entrega remapeia membership em nove unidades: `kernel/observability` (três packages novos), `orders/app`, `reservations/app` e `resource-scheduling/app` (migração de layout e `cmd/`), e as quatro unidades de kit novas. Cada ato exige commit próprio com revisor diferente do autor, e `--write-baseline` é passo humano (ADR-028).

**A evidência diverge por path, em dois dos sete subjects.** Medido contra a linha de base tirada antes da primeira mudança: `app`, `dist`, `domain`, `golden` e `services` mantêm o digest; `provider` e `reference` divergem. São exatamente os que nomeiam packages que mudaram de caminho — `reference` varre `apps/backend/...`, onde a migração para `app/` reescreveu todo import path, e `provider` cobre o `postgres` e o `testkit/tb/pg`, que ganharam o tracer promovido e o reset extensível. A divergência é consequência documentada da reorganização, como no ADR-046, não regressão.

**Dois packages de produção nasceram sem teste e só a evidência acusou.** `bookings/ports` (pré-existente, do ADR-046) e `bookings/app/http` (criado por esta entrega). Um package sem arquivo de teste faz o `go test -json` emitir `skip`, e `JudgeResults` reprova skip sob CI — o que trava a geração da evidência, e não o commit que criou o package. Os dois ganharam teste aqui; o buraco de processo permanece aberto, e o candidato natural a fechá-lo é o próprio `dmpf-context-check.sh`.

**O `bff` consome o kernel sem mudar de layout.** Ele não é bounded context — não tem domínio — e segue como borda. Mas os helpers de ambiente, o `Boot`, o `ShutdownGrace` e o `NewController` que ele duplicava agora vêm do kernel.

**O alias pelo papel ganha um caso.** Com o contexto em `app/`, o package do contexto chama-se `app`, como o bloco `app` do kernel. Onde os dois se encontram, o kernel recebe alias: `kernelapp` quando `kernel` já nomeia o `domain`. É a regra do ADR-045 aplicada a uma colisão que o layout novo criou.
