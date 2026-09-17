# app

Bloco `app` do kernel DMPF: o consumer adapter que fica entre a entrega do transporte e o application service de consumo (FND-04 §6.3, passos 1, 2 e 7). Primeiro módulo do bloco `app` no workspace.

Criado por `KRN-07` (ARQ-526, `docs/specs/SPEC-ANZX2WPG-dmpf-inbox-consumo.md`; decisões em `docs/adr/036-classificacao-de-recepcao-e-fronteira-pending.md`).

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

`Consumer.Consume(ctx, Delivery, Acknowledger)` realiza os passos 1, 2 e 7 da sequência de consumo (FND-04 §6.3):

1. `envelope.Unmarshal(Raw)` valida o envelope. Envelope inválido **não** recebe classificação de recepção: vai para a contenção e o `Handler` nunca roda (`INB-10`).
2. Monta o `Receipt` com `payload_hash = payloadhash.Sum(Payload)` sobre os bytes transportados (`ENV-18`) e invoca o `Handler` — o application service de consumo, que devolve a disposição.
3. Aplica o efeito de broker **depois** do retorno do `Handler` (`INB-08`), conforme a tabela abaixo.

| Disposição | Efeito no broker | Contenção |
| --- | --- | --- |
| R1×D1, R1×D2, R2, R3 | `Ack` | — |
| R1×D3 com `Attempt < MaxAttempts` | `Release` (redelivery a cargo do transporte) | — |
| R1×D3 com `Attempt ≥ MaxAttempts` | `Ack` após conter | `attempts-exhausted` (`GAR-08`) |
| R1×D4 | `Ack` após conter | `terminal-failure` |
| R4 | `Ack` após conter | `collision` |
| envelope inválido | `Ack` após conter | `invalid-envelope` |

A contenção sempre grava `Delivery.Raw` — nunca um re-marshal do envelope decodificado, que descartaria campos desconhecidos e quebraria a identidade byte a byte exigida por `GAR-07`. Se a quarantine falhar, a mensagem **não** é confirmada: contida ou nada.

## Mapeamento situação → mecanismo (`GAR-11`)

Declarado em código como dado revisável (`ContainmentMap`) e conferido por teste de totalidade.

| Situação (`ports.Reason`) | Mecanismo nesta entrega | Quando o `KRN-10` existir |
| --- | --- | --- |
| `invalid-envelope` | quarantine | quarantine |
| `terminal-failure` (R1×D4) | quarantine | quarantine |
| `collision` (R4) | quarantine | quarantine |
| `attempts-exhausted` (R1×D3 esgotado) | quarantine | dead-letter queue do transporte |

Quarantine e DLQ são mecanismos distintos: a primeira retém para inspeção sem reentrega automática; a segunda é o destino terminal do transporte. O `KRN-07` realiza apenas a quarantine (sobre Postgres, em `postgres`); a DLQ depende de broker e é do `KRN-10`.
