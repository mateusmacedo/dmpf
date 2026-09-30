# app

Bloco `app` do kernel DMPF: o consumer adapter que fica entre a entrega do transporte e o application service de consumo (FND-04 §6.3, passos 1, 2 e 7). Primeiro módulo do bloco `app` no workspace.

Criado por `KRN-07` (ARQ-526, `docs/specs/SPEC-ANZX2WPG-dmpf-inbox-consumo.md`; decisões em `docs/adr/036-classificacao-de-recepcao-e-fronteira-pending.md`). A reconstrução do contexto de execução no adapter é da SPEC-9B6SHEH8 (ADR-049, ADR-052).

## Por que um módulo próprio

O adapter precisa de duas arestas ao mesmo tempo: `contract` (decodificar o envelope e calcular o `payload_hash`) e `application` (invocar o caso de uso). `application → contract` (célula 12) e `provider → application` (célula 26) são proibidas pela matriz de blocos e têm vetor negativo em `tools/dmpf-cell-check.sh`. Só a linha `app` permite ambas.

## Unidades do manifesto

| Unidade | Bloco | Package |
| --- | --- | --- |
| `kernel/app-consumer` | `app` | raiz do módulo |
| `kernel/app-relay` | `app` | `relay` — o relay da outbox de `KRN-08` (ADR-038) |

A composition root do consumidor de exemplo, que este módulo carregava em
`example/reservations`, é hoje o bloco `app` do contexto `reservations` — o
package raiz de `apps/backend/reservations` (unidade `reservations/app`,
ADR-046). Aqui fica só o que todo contexto consumidor importa.

Desde o `KRN-12` (ADR-041), `Consumer.Consume` põe no contexto do `Handler` o contexto de mensagem do envelope — `correlationid`, o `id` recebido como `causationid` e `traceparent` — por `ports.WithMessageContext`, para que o application service de consumo o copie em cada `OutboxEntry` que autorar (FND-07 §8.6 item 3). Quem cabeia este adapter e o relay em processos reais são os composition roots `apps/backend/reservations` (relay e consumer) e `apps/backend/orders` (relay), conforme o ADR-044.

O módulo não declara dependência externa (`external: []`): só importa módulos irmãos do workspace. O código de produção não importa `google.golang.org/protobuf` — a decodificação de wire fica em `contracts/envelope.Unmarshal`, e o adapter recebe os bytes brutos em `Delivery.Raw`.

## O adapter

`Consumer` declara o que a spec exige (`ErrIncompleteConsumer` recusa a partida sem qualquer um): `Name`, `MaxAttempts` (`<= 0` desliga o teto), `Handle` (o `Handler` — o application service de consumo), `Containment`, `Clock`, `Timeout` (o orçamento de prazo por tentativa, `CTX-28`), `Boundary` e `Locale` (a declaração de `CTX-01` que uma consumição não tem caller para fornecer).

`Consumer.Consume(ctx, Delivery, Acknowledger)` realiza os passos 1, 2 e 7 da sequência de consumo (FND-04 §6.3):

1. `envelope.Unmarshal(Raw)` valida o envelope. Envelope inválido **não** recebe classificação de recepção: vai para a contenção e o `Handler` nunca roda (`INB-10`).
2. `Boundary.admits(env.Source)` recusa a mensagem cuja origem não está na fronteira confiável declarada — `Boundary{Transport, Sources}`, onde `Transport` é `verified` (o broker autentica o workload produtor, mTLS/SASL, ADR-052) ou `development-only` (o opt-out de desenvolvimento do transporte). Fora da fronteira vai para a contenção como `untrusted-boundary`, sem produzir contexto (`CTX-27`, `IDN-04`).
3. `executionOf` reconstrói o `ports.ExecutionContext` da consumição a partir do envelope: tenant, correlação, causação e trace vêm de `env` (`CTX-24`); **nenhum sujeito** entra, porque proveniência não é identidade (`CTX-25`); `RequestID` e `Deadline` são do próprio consumidor — cunhados por tentativa (`Attempt`) e derivados de `Timeout` — porque a consumição é a execução do consumidor, não a do produtor (`CTX-28`). Se a reconstrução falhar, a mensagem é contida como `invalid-envelope` em vez de subir o erro sozinho, o que a deixaria para reentrega eterna.
4. Monta o `Receipt` com `payload_hash = payloadhash.Sum(Payload)` sobre os bytes transportados (`ENV-18`), deposita o contexto de execução e o de mensagem no `ctx` (`ports.WithExecutionContext`, `ports.WithMessageContext`) e invoca o `Handler`, que devolve a disposição.
5. Aplica o efeito de broker **depois** do retorno do `Handler` (`INB-08`), conforme a tabela abaixo.

| Disposição | Efeito no broker | Contenção |
| --- | --- | --- |
| R1×D1, R1×D2, R2, R3 | `Ack` | — |
| R1×D3 com `Attempt < MaxAttempts` | `Release` (redelivery a cargo do transporte) | — |
| R1×D3 com `Attempt ≥ MaxAttempts` | `Ack` após conter | `attempts-exhausted` (`GAR-08`) |
| R1×D4 | `Ack` após conter | `terminal-failure` |
| R4 | `Ack` após conter | `collision` |
| envelope inválido | `Ack` após conter | `invalid-envelope` |
| origem fora da fronteira | `Ack` após conter | `untrusted-boundary` |

A contenção sempre grava `Delivery.Raw` — nunca um re-marshal do envelope decodificado, que descartaria campos desconhecidos e quebraria a identidade byte a byte exigida por `GAR-07`. Se a quarantine falhar, a mensagem **não** é confirmada: contida ou nada.

`Attempt{RequestID, Number}` (`attempt.go`) é o identificador que o adapter cunha por tentativa e a contagem de entrega do transporte; nenhum dos dois vem do envelope. Só o `Handler` o lê, para montar o contexto de execução — dali para baixo o contexto viaja pelo `ctx`, nunca por parâmetro explícito (`CTX-03`).

## Mapeamento situação → mecanismo (`GAR-11`)

Declarado em código como dado revisável (`ContainmentMap`) e conferido por teste de totalidade.

| Situação (`ports.Reason`) | Mecanismo nesta entrega | Quando o `KRN-10` existir |
| --- | --- | --- |
| `invalid-envelope` | quarantine | quarantine |
| `untrusted-boundary` | quarantine | quarantine |
| `terminal-failure` (R1×D4) | quarantine | quarantine |
| `collision` (R4) | quarantine | quarantine |
| `attempts-exhausted` (R1×D3 esgotado) | quarantine | dead-letter queue do transporte |

Quarantine e DLQ são mecanismos distintos: a primeira retém para inspeção sem reentrega automática; a segunda é o destino terminal do transporte. O `KRN-07` realiza apenas a quarantine (sobre Postgres, em `postgres`); a DLQ depende de broker e é do `KRN-10`.

## O relay

`relay/postgres.go` traz `NewOverPostgres(pool, publisher, component, config)`, que monta o dreno de FND-04 §5.4 sobre a outbox de `postgres` com a identidade do kernel (`idclock.SystemClock`, `idclock.NewClaimIDs(component)`) — antes, `orders` e `reservations` repetiam essa montagem e cada um trazia o próprio `RandomClaimIDs`. O relay drena de todos os tenants sem distinção: a outbox fica fora do escopo de tenant (ADR-050), e é `record.go` quem lê o `tenantid` da `metadata` que `postgres` escreveu e o coloca no envelope publicado, para que o consumidor o leia de volta em `executionOf`.

## Referências

- `docs/specs/SPEC-ANZX2WPG-dmpf-inbox-consumo.md` — spec do `KRN-07`
- `docs/adr/036-classificacao-de-recepcao-e-fronteira-pending.md` — decisões do adapter e da inbox
- `docs/adr/038-drenagem-da-outbox-lease-e-envelope-na-publicacao.md` — o relay
- `docs/adr/049-contexto-de-execucao-viaja-no-context-context.md` — o carrier que `executionOf` popula
- `docs/adr/050-tabelas-de-infraestrutura-fora-do-escopo-de-tenant.md` — por que o relay drena sem tenant
- `docs/adr/052-identidade-de-workload-no-grpc-e-no-kafka.md` — `Boundary.Transport` e a fronteira confiável
- `libs/backend/go/ports/README.md` — `ExecutionContext`, `Acknowledger`, `Containment`, `MessageContext`
- `libs/backend/go/postgres/README.md` — a outbox, a inbox e a quarantine que este adapter usa
