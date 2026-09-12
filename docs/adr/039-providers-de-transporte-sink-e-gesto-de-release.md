# ADR-039: Realizar os providers de transporte em Go com um módulo de primitivas compartilhadas, ponte por `Sink` com o adapter e gesto de `Release` assimétrico entre Kafka e SQS

## Status

Aceito — 2026-09-06. Implementa SPEC-EAGAXQN1.

## Contexto

O ADR-024 decidiu o transporte síncrono — REST/JSON na borda externa, gRPC entre
serviços nossos, com o tempo governado pela borda — e o ADR-025 decidiu o
assíncrono: Kafka como transporte-alvo do evento de domínio e SNS/SQS
normatizado. O FND-06 (`docs/dmpf/politicas-transporte.md`) fixou as regras:
governo do tempo por método (`GRP-04` a `GRP-18`), retry só onde a idempotência
é declarada (`GRP-08`, `RST-02`), byte-preservação do payload com a matriz de
hops (`TRP-13`, §5.2), a ordem do ACK depois do commit local (`TRP-26` a
`TRP-32`), a DLQ antes do avanço do offset (`TRP-30`) e a catalogação de canal
sem a qual o provider não opera (`ASY-01`, `ASY-02`).

Nenhuma dessas regras tinha realização. O relay do `KRN-08` (ADR-038) só
publicava em memória; `dmpfports.Acknowledger.Release` estava reservado ao
`KRN-10` no próprio godoc; o `KRN-09` (ADR-037) entregara os decorators de
resiliência e o governo do tempo (`resilience.EffectiveDeadline`, `Timeout`,
`NewRetry`) sem transporte que os compusesse; e o `KRN-05` (ADR-033) fixara o
`payload_hash` que todo hop precisa preservar.

Restrições herdadas: a matriz de blocos veda `provider → application` e
`provider → app` (células 26 e 27, RFC §7.3), então o provider não pode importar
o consumer adapter do `dmpf-app`; a cadeia Go do workspace exige um projeto Nx
por commit e mudança normativa do baseline em commit próprio (`DMPF-T002`); e o
CI roda em `act_runner`, onde o bloco `services:` não resolve DNS (ADR-035).

## Decisão

**Cinco módulos, não quatro.** Além de `dmpf-provider-grpc`, `-http`, `-kafka`
e `-sqs`, existe `dmpf-transport`, bloco `provider`, com as primitivas que gRPC
e HTTP compartilham (o orçamento de prazo por método, `deadline`), as que Kafka
e SQS compartilham (a catalogação de canal e as fórmulas de `janela_redelivery`,
`channel`; o metadado de tentativa, `attempt`), a admissão por rota e tenant
(`admission`) e as três posições de observabilidade da composição (`observe`).
Nenhum dos quatro providers era lugar neutro para isso, e `dmpf-observability` é
nomeado por FND-08, não por transporte.

**O governo do tempo é o do `KRN-09`; `deadline` só acrescenta o que faltava.**
`deadline.Budget` embrulha um `resilience.Operation`; `deadline.Outgoing` é
`EffectiveDeadline` menos a folga do hop (`GRP-17`); `deadline.Require` recusa o
contexto sem prazo (`GRP-04`). Não há segunda fórmula de prazo nem segundo
backoff — o retry dos providers é `resilience.NewRetry` com um classificador por
transporte, e o retry nativo do gRPC é desligado (`WithDisableRetry`) porque não
vê idempotência. No gRPC a composição de `RES-22` é **uma `Call` por método**,
porque o classificador depende dos códigos transientes de cada método; no HTTP é
uma por cliente, porque o status transiente vira erro dentro da tentativa.

**Os três decorators de observabilidade vivem em `dmpf-transport/observe`, e a
montagem da composição em `dmpf-transport/compose`.** O `KRN-09` exige as
posições de tracing, métricas e log em toda composição (`RES-23`) e não as
construiu. Elas nascem aqui uma vez, parametrizadas só pela função de categoria
de falha de cada transporte, e os quatro providers as consomem. A ordem dos
decorators, a reserva do `Timeout` e a regra "`Retry` declarado `false` recebe
identidade" também eram quatro cópias; o code review as recolheu em
`compose.Build`, `compose.Shared`, `compose.Retry` e `compose.Operation`, e cada
provider passa só a sheet, o classificador e a categoria. Consequência aceita:
`dmpf-transport` declara a API do OpenTelemetry (`otel/trace`, `otel/metric`)
como dependência externa — deixa de ser um módulo sem dependência externa, mas
continua sem I/O.

**A ponte com o adapter é uma interface do provider.** O consumidor de Kafka e o
de SQS entregam cada mensagem a um `Sink` — `Handle(ctx, raw, attempt, ack)` —
que o consumer adapter do `dmpf-app` realiza; o provider nunca importa o `app`.
A contenção é a porta `dmpfports.Containment`, realizada aqui como DLQ (tópico
ou fila de contenção do canal), e é o adapter quem a chama, antes do gesto. Os
tetos são um contrato declarado, não inferido: em Kafka,
`dmpfapp.Consumer.MaxAttempts` **igual** a `Channel.Retry.MaxAttempts`, e o
composition root os iguala; em SQS, o teto do adapter **maior** que o
`maxReceiveCount` da fila, para que redrive gerenciado e publicação explícita
nunca se apliquem à mesma disposição (`SQS-11b`).

**`Release` é assimétrico entre os dois transportes, e isso é a norma.** Em
Kafka, não commitar não reentrega (`TRP-47`): só o `Release` explícito repete o
mesmo registro em laço, com backoff e a partição pausada, até o limite do canal.
Atingido o limite, ou quando o `Sink` retorna **sem gesto** — erro sem `Ack` nem
`Release`, ou retorno silencioso —, o registro fica **pendente** e a partição
**para** (`stalled`): a fila do worker não avança, porque processar o registro
seguinte aplicaria efeitos fora de ordem (`KFK-09`), o offset não avança e o log
diz por quê; só a revogação devolve a partição, a quem a receber — e devolve
**com a pausa de fetch levantada**, porque o kgo mantém partições pausadas
entre rebalances e a mesma instância poderia recebê-la de volta muda. Repetir
um registro sem gesto seria tratar erro como `Release`, e o code review mostrou
o custo: uma falha entre a quarentena e o `Ack` publicaria a mesma contenção
duas vezes. Um panic do `Sink` — código do bloco `app` decodificando bytes de
terceiros — tem o mesmo desfecho de um retorno sem gesto, nos dois transportes:
um registro envenenado para uma partição, não o processo. Em SQS, o gesto é encurtar a visibilidade para o backoff e deixar a fila
reentregar (`SQS-10`); não há laço inline, e um consumidor que o declare é
recusado (`SQS-11b`). Um worker persistente por partição em Kafka, com fila
própria e pausa de fetch quando ela atinge a capacidade declarada, commit apenas
do prefixo contíguo (`TRP-29`) e cancelamento cooperativo na revogação, que
fecha o commit do worker antes de cancelá-lo (`TRP-48`); em SQS, o receive só
pede ao broker o que há de slot livre, cada mensagem recebida tem worker
imediato, e o heartbeat de visibilidade — cada tick limitado pelo próprio
intervalo — é parado e aguardado antes de qualquer gesto (`SQS-08`).

**O que trafega é o que entrou.** O `Observer` do publisher Kafka recebe cópias
e não tem como substituir o registro (`TRP-17`); o SDK do SQS codifica o
envelope em Base64 uma única vez (`TRP-19`) e o decodificador reconhece e recusa
a envoltória de notificação do SNS (`SQS-02`). O publisher SNS **lista ele
mesmo** as assinaturas do tópico (`ListSubscriptionsByTopic`) na construção e
recusa o tópico com assinatura SQS sem raw delivery, com filtro em tópico FIFO
(`SQS-03c`) ou sem nenhuma assinatura SQS — uma lista passada pelo chamador, a
versão que o code review descartou, verificaria só o que o chamador lembrasse
de declarar. As linhas Kafka, SQS e SNS da matriz de hops de FND-06 §5.2
viraram suíte, com par positivo e negativo por linha.

**Emuladores no CI: Redpanda para Kafka e floci para SQS e SNS.** Ambos sobem
no job por `docker run --network container:$(hostname)`, o gesto do ADR-035. O
floci foi decisão do usuário sobre o ElasticMQ que a spec propunha, porque um só
emulador cobre SQS **e** SNS e torna o hop SNS → SQS testável nas duas linhas;
LocalStack está vetado neste workspace.

**Pendência declarada — `TRP-09`/`TRP-46`, binding persistido por mensagem.**
Fora do escopo desta entrega; o binding é resolvido por processo pelo catálogo.
Owner: o time de arquitetura DMPF (épico ARQ-519). Prazo: antes do `KRN-12`
(composition root de exemplo), que é a primeira entrega em que dois transportes
coexistem no mesmo processo e o binding por mensagem passa a importar.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Fórmula própria de prazo em `dmpf-transport/deadline` | Criaria uma segunda autoridade sobre `RES-05`/`RES-06`/`RES-07`, que o `KRN-09` já realiza; a leitura por analogia de `TRP-53` veda a segunda tabela |
| Confiar no retry nativo do gRPC (`retryPolicy`) | Não distingue idempotência (gRFC A6); `GRP-08` exige que o retry derive dela |
| Quatro módulos, com as primitivas dentro do gRPC ou do Kafka | Nenhum é lugar neutro: HTTP dependeria do gRPC pelo prazo, SQS do Kafka pela catalogação |
| Decorators de tracing/métricas/log em cada provider | Quatro cópias do mesmo código; a divergência viria na primeira manutenção |
| `SetOffsets` para trás e novo poll no retry Kafka | Reentrega registros posteriores já bufferizados e complica o cursor |
| Tópico de retry em Kafka | Vedado em canal ordenado (`KFK-11`, `TRP-32`) |
| `CommitOffsets` com callback tipado | Exige o módulo `kmsg`; `CommitRecords` é síncrono e commita `offset + 1` (doc do kgo) |
| Laço inline de retry em SQS | `SQS-11b`: redrive gerenciado e laço na mesma disposição permitem duplicata na contenção; a redelivery é da fila |
| `kfake` para o laço real sem Docker | Módulo separado e opcional; a integração contra Redpanda cobre o laço e foi ela que encontrou o deadlock de `Close` sob `BlockRebalanceOnPoll` |
| ElasticMQ para SQS, fixture para SNS | Não cobre SNS; decisão do usuário pelo floci. LocalStack vetado |

## Consequências

**Positivas:**

- As seis regras estruturais de FND-06 que mais se violam por acidente —
  deadline reiniciado, retry sem idempotência, `auto.commit`, offset
  ultrapassando pendente, DLQ depois do avanço, SNS sem raw — têm teste
  executável, quatro deles contra broker ou emulador real.
- Um módulo de primitivas evita quatro cópias de prazo, catalogação, admissão e
  observabilidade, e dá aos providers TypeScript futuros o mesmo desenho.
- A integração real encontrou dois defeitos que os fakes não veriam: o deadlock
  de `Close` com `BlockRebalanceOnPoll` e a ambiguidade da derivação de
  deduplicação por separador.
- O code review em duas frentes (interno e modelo externo) encontrou treze
  pontos, todos aplicados: os de contrato — corpo HTTP cancelado antes da
  leitura, `Release` implícito em erro sem gesto, commit após revogação,
  heartbeat sem cancelamento do tick, assinatura SNS não declarada — mudaram
  este ADR; os demais ficaram nos módulos e nos READMEs.
- O checklist de qualidade (segurança, robustez, performance) encontrou
  dezessete pontos, todos aplicados: pausa de fetch que sobrevivia à revogação,
  fila drenada pelo `Sink` após o cancelamento, label de rota HTTP com o path
  cru, short polling e receive sem backoff em SQS, cliente kgo vazado na
  construção, assignment antes do `run`, heartbeat que não cabia duas vezes na
  visibilidade, endpoint SQS sem TLS, panic do `Sink`, atributos de métrica
  reconstruídos por chamada, retenção de registros já processados, `Operation`
  por mensagem, piso de TLS, URL da fila em log, valores externos sem corte na
  DLQ, erro junto do `Ack` sem log e o timer do stream gRPC.

**Negativas:**

- **Custo aceito:** `dmpf-transport` passa a depender da API do OpenTelemetry;
  a spec e o README foram ajustados, e a dependência é só de tipos e interfaces.
- Cinco módulos novos são cinco commits normativos de baseline (`DMPF-T002`)
  além dos de código, e o verificador reprova até que existam.
- Os containers do CI sobem em todo PR com projeto Go afetado, mesmo quando só
  o Postgres seria necessário; condicioná-los ao `affected` por módulo exige
  mover a detecção de `ci.yml` — registrado como melhoria, não incluído.
- `kadm` e `aws-sdk-go-v2/config` entram como dependências só de teste; o
  `go.mod` os lista mesmo que o manifesto não os declare.

## Referências

- `docs/specs/SPEC-EAGAXQN1-dmpf-providers-transporte.md` — a spec desta entrega.
- `docs/dmpf/politicas-transporte.md` (FND-06) — §5.2, §6, §9, §10, §11, §12, §16.
- `docs/dmpf/resiliencia-observabilidade.md` (FND-08) — `RES-16`, `RES-17`, `RES-22`, `RES-23`, `RES-31`, `MET-11`, `MET-12`.
- `docs/adr/024-rest-externo-grpc-interno-governo-do-tempo.md`, `docs/adr/025-kafka-transporte-alvo-sns-sqs-acervo.md` — as decisões de transporte.
- `docs/adr/035-realizacao-postgres-da-outbox.md` — a rede do job no CI.
- `docs/adr/037-observabilidade-otel-e-retry-por-conjuncao-em-go.md` — o que este ADR compõe.
- `docs/adr/038-drenagem-da-outbox-lease-e-envelope-na-publicacao.md` — o relay que estes providers servem.
- READMEs de `libs/backend/go/dmpf-transport`, `dmpf-provider-grpc`, `dmpf-provider-http`, `dmpf-provider-kafka`, `dmpf-provider-sqs`.

## Addendum — 2026-09-12

A pendência `TRP-09`/`TRP-46` registrada na `## Decisão` tinha prazo declarado:
"antes do `KRN-12` (composition root de exemplo)". O prazo venceu com a sub-spec
1 do `KRN-12` (`SPEC-6QT9SBAS`), e o desfecho foi outro. O `dmpf-reference` sobe
**um transporte assíncrono por processo** — Kafka —, decisão do usuário fixada
na guarda-chuva `SPEC-8HWBWJCB`. Com um só canal assíncrono por processo, o
binding segue resolvido pelo catálogo de `dmpf-transport/channel`, e o binding
persistido por mensagem nunca é exercido: deixa de ser pré-requisito da release.

A pendência não foi resolvida — foi empurrada com endereço. Vira a task
sucessora **ARQ-549** (`KRN-13`), no épico ARQ-519.

- **Owner**: o time de arquitetura DMPF (épico ARQ-519), como já declarado acima.
- **Dependência**: um processo que hospede dois canais assíncronos ao mesmo
  tempo. É a primeira composição em que a resolução por processo deixa de bastar,
  e não existe nenhuma no workspace até aqui.
- **Aceite**: o binding de canal é decidido por mensagem, não por processo — um
  processo com dois canais assíncronos entrega cada mensagem ao canal que o
  vínculo persistido declara, e a reentrega da mesma mensagem cai no mesmo canal.
