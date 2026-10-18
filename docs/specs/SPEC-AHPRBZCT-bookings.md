---
id: SPEC-AHPRBZCT
slug: bookings
title: Bounded context — bookings
stage: done
priority: P2
depends_on: [SPEC-XMNBMY50]
ticket_url: https://linear.app/mmda/issue/DEVS-40/spec-ahprbzct-bounded-context-bookings
subtask_urls: []
created: 2026-09-10
---

# SPEC-AHPRBZCT: Bounded context — bookings

## Identidade

- **bounded_context**: `resource-scheduling`
- **name**: `bookings`
- **Linguagem ubíqua**:
  - *Recurso* (`Resource`) — algo reservável, conhecido pelo seu código.
  - *Reserva* (`Booking`) — uma quantidade de um recurso tomada num instante.
  - *Registro* — o ato de tornar um recurso conhecido do contexto.
  - *Cancelamento* — o ato de desfazer uma reserva; uma reserva cancelada não
    volta a valer.

## Agregados

### Booking

- **Identidade**: `generated`
- **Campos**:
  - `resourceId: id`
  - `quantity: int`
  - `status: new | reserved | cancelled`
  - `reservedAt: instant`
- **Estados**: `new` (antes do primeiro `Reserve`), `reserved`, `cancelled`
- **Transições permitidas**:
  - (novo) → `reserved` — por `Reserve`
  - `reserved` → `cancelled` — por `Cancel`
- **Invariantes de estado**:
  - `status` é sempre um dos três valores; uma reserva cancelada não sai de
    `cancelled` — `Cancel` sobre `cancelled` rejeita com
    `resource-scheduling/booking/not-reserved`.
  - `quantity` está sempre em `[1, 100]` — garantido na criação, porque
    nenhum comando a altera depois.

### Resource

- **Identidade**: `natural` por `code`
- **Campos**:
  - `code: string`
  - `name: string`
  - `registeredAt: instant`
- **Estados**: nenhum
- **Transições permitidas**: nenhuma
- **Invariantes de estado**:
  - `code` nunca é vazio — `Register` rejeita com
    `resource-scheduling/resource/code-empty`.
  - `name` nunca é vazio — `Register` rejeita com
    `resource-scheduling/resource/name-empty`.
  - `name` é único por tenant, por igualdade exata — a unicidade é declarada no
    `Table` e levantada pelo serviço de aplicação a partir do `Save`, não pela
    UPR, como `resource-scheduling/resource/duplicate-name` (ADR-066).

## Comandos (UPRs)

### Reserve

- **Agregado**: `Booking`; **cria**
- **Comando**: `ReserveBooking`
- **Entrada**:
  - `resourceId: id`
  - `quantity: int`
- **Pré-condições**:
  - `1 ≤ quantity ≤ 100` → `resource-scheduling/booking/quantity-out-of-range`
- **Efeitos sobre o estado**:
  - `status ← reserved`
  - `quantity ← cmd.quantity`
  - `resourceId ← cmd.resourceId`
  - `reservedAt ← at`
- **Eventos emitidos**: `BookingReserved`
- **Resposta**: `bookingId: id`

### Cancel

- **Agregado**: `Booking`; exige existente
- **Comando**: `CancelBooking`
- **Entrada**: nenhuma além da identidade do agregado
- **Pré-condições**:
  - `status = reserved` → `resource-scheduling/booking/not-reserved`
- **Efeitos sobre o estado**:
  - `status ← cancelled`
- **Eventos emitidos**: `BookingCancelled`
- **Resposta**: `bookingId: id`

### Register

- **Agregado**: `Resource`; **inicializa se ausente**
- **Comando**: `RegisterResource`
- **Entrada**:
  - `code: string`
  - `name: string`
- **Pré-condições**:
  - `code` não vazio → `resource-scheduling/resource/code-empty`
  - `name` não vazio → `resource-scheduling/resource/name-empty`
- **Efeitos sobre o estado**:
  - quando ausente: `name ← cmd.name`, `registeredAt ← at`
  - quando presente: nenhum — aceita sem alterar, inclusive o nome original, e sem emitir
- **Eventos emitidos**: `ResourceRegistered` (só quando inicializa)
- **Resposta**: `code: string`

## Eventos de domínio

### BookingReserved

- **Campos**:
  - `bookingId: id ← Booking.id`
  - `resourceId: id ← Booking.resourceId`
  - `quantity: int ← Booking.quantity`
  - `at: instant ← at`

### BookingCancelled

- **Campos**:
  - `bookingId: id ← Booking.id`
  - `at: instant ← at`

### ResourceRegistered

- **Campos**:
  - `code: string ← Resource.code`
  - `name: string ← Resource.name`
  - `at: instant ← at`

## Consultas

- `FindBooking` — filtros: `bookingId`; cardinalidade: `one`
- `FindBookingByResource` — filtros: `resourceId`; cardinalidade: `many`

## Relações entre agregados

- `Booking.resourceId` → `Resource`

## Integração

- **Publica**: `BookingReserved`, `BookingCancelled` e `ResourceRegistered` —
  viram `booking_reserved.proto`, `booking_cancelled.proto` e
  `resource_registered.proto` em `apps/backend/bookings/contract/proto/company/bookings/event/v1/`,
  package `company.bookings.event.v1`, no tópico do contexto.
- **Consome**: nenhum.

## Borda e persistência

- **Borda**: só gRPC, `company.bookings.service.v1.BookingsService`
  (`ReserveBooking`, `CancelBooking`, `RegisterResource`, `FindBooking`,
  `FindBookingsByResource`), atrás da cadeia de interceptors do kernel. O REST
  público é do `bff`: `POST /bookings/booking`, `GET /bookings/booking`,
  `GET /bookings/booking/{id}`, `POST /bookings/booking/{id}/cancel` e
  `POST /bookings/resource`, descritos em `apps/backend/bookings/contract/openapi/v1/`
  (ADR-044, ADR-053).
- **Forma do `name` na borda**: o gRPC (`INVALID_ARGUMENT`) e o `bff` (400
  `invalid-request`) aplicam a mesma regra, antes do caso de uso: de 1 a 200
  runas de UTF-8 válido, sem caractere de controle (categoria Unicode `Cc`), de
  formatação (categoria `Cf`, como U+200B, U+202E, U+2060, U+2066 a U+2069 e
  U+FEFF), separador de linha ou de parágrafo (U+2028, U+2029), seletor de
  variação (U+FE00 a U+FE0F, U+E0100 a U+E01EF e os mongóis U+180B a U+180F)
  nem outro ponto de código ignorável por padrão (como U+034F e os
  preenchedores Hangul U+115F, U+1160, U+3164 e U+FFA0), e com ao menos um
  caractere que não seja espaço Unicode nem marca combinante (categoria `M`):
  U+0301 sozinho é recusado, e o "é" decomposto (U+0065 U+0301) é aceito. O
  nome não é normalizado nem aparado: espaços no início, no meio ou no fim são
  aceitos e fazem parte da igualdade exata. Escrita não latina, como CJK
  ("会议室"), é aceita, e o espaço interno ("Sala 1 Anexo") é preservado.
- **Consequência deliberada do seletor de variação**: todo seletor é recusado,
  inclusive o U+FE0F depois de emoji. "❤️" (U+2764 U+FE0F), como muitos
  teclados de celular o digitam, é recusado; "❤" sem seletor e emoji que não
  usam seletor, como "🎉", são aceitos. A sequência de emoji com U+200D (ZWJ,
  categoria `Cf`), como as de família, também é recusada.
- **Limites conhecidos**: U+2800 (braille em branco), U+FFFC e homóglifos
  visíveis (como o "а" cirílico no lugar do "a" latino) são aceitos e contam
  como nomes distintos na igualdade exata.
- **Banco**: database e role `bookings`, schema `public`, tabelas `bookings` e
  `resources`, com `tenant_id` à frente da chave. Persistência híbrida:
  `resource_id` é coluna tipada em `bookings` porque `FindBookingByResource`
  filtra por ela; o resto do estado vai em `snapshot` `jsonb` (ADR-053). Em
  `resources`, `name` é coluna gerada do snapshot (`GENERATED ALWAYS AS ...
  STORED`), com `CONSTRAINT resources_tenant_id_name_key UNIQUE (tenant_id,
  name)` declarada no `Table` (ADR-066).
- **Instante**: inteiro de nanossegundos no domínio.

## Políticas transversais

- **Idempotência**: o comando de criação (`Reserve`) entra pelo `bff` por `POST`
  com chave de idempotência declarada, que segue na metadata gRPC só para o log;
  os demais são idempotentes por identidade do agregado (RST-02; FND-04).
- **Autorização**: pelo gancho de autorização do bloco `application`, antes da
  transação (FND-04 §3.2).
- **Auditoria**: pela trilha de auditoria do kernel (FND-08;
  `observability/audit`).

## Critérios de aceite

- [x] Os gates do workspace verdes para `bookings` (`fmt-check`, `vet`,
  `build`, `lint`, `test-race` e `test-distributed` com Postgres, verificador
  com `--base` e `dmpf-context-check`).
- [x] Cenários:

```text
DADO um recurso registrado
QUANDO Reserve chega com quantity 10
ENTÃO a reserva nasce reserved, com reservedAt = at, e BookingReserved vai à outbox com bookingId, resourceId, quantity e at

DADO qualquer recurso
QUANDO Reserve chega com quantity 0 (ou 101)
ENTÃO rejeita com resource-scheduling/booking/quantity-out-of-range e nada é escrito

DADO uma reserva reserved
QUANDO Cancel chega
ENTÃO a reserva vai a cancelled e BookingCancelled é emitido

DADO uma reserva cancelled
QUANDO Cancel chega
ENTÃO rejeita com resource-scheduling/booking/not-reserved e o estado não muda

DADO um código ainda desconhecido
QUANDO Register chega
ENTÃO o recurso é inicializado com registeredAt = at e ResourceRegistered é emitido

DADO um código já registrado
QUANDO Register chega
ENTÃO aceita sem alterar o recurso e sem emitir evento

DADO qualquer entrada
QUANDO Register chega com code vazio
ENTÃO rejeita com resource-scheduling/resource/code-empty

DADO qualquer entrada
QUANDO Register chega com name vazio
ENTÃO rejeita com resource-scheduling/resource/name-empty

DADO qualquer entrada
QUANDO RegisterResource chega à borda com name de caractere de formatação invisível (U+200B, U+202E, U+2060, U+2066 a U+2069, U+FEFF) ou só de espaços
ENTÃO a borda recusa com INVALID_ARGUMENT (400 no bff) e o caso de uso não é chamado

DADO qualquer entrada
QUANDO RegisterResource chega à borda com name de seletor de variação (U+FE00 a U+FE0F, U+E0100 a U+E01EF, U+180B a U+180F), separador U+2028 ou U+2029, U+034F ou preenchedor Hangul (U+115F, U+1160, U+3164, U+FFA0)
ENTÃO a borda recusa com INVALID_ARGUMENT (400 no bff) e o caso de uso não é chamado

DADO qualquer entrada
QUANDO RegisterResource chega à borda com name só de espaços e marcas combinantes, como U+0301 sozinho
ENTÃO a borda recusa com INVALID_ARGUMENT (400 no bff) e o caso de uso não é chamado

DADO qualquer entrada
QUANDO RegisterResource chega à borda com name "Sala ❤️" (U+2764 U+FE0F)
ENTÃO a borda recusa com INVALID_ARGUMENT (400 no bff), enquanto "Sala ❤" e "Sala 🎉" são aceitos

DADO um código já registrado com um nome
QUANDO Register chega com o mesmo código e outro nome
ENTÃO aceita sem alterar o nome original e sem emitir evento

DADO um recurso registrado com um nome no tenant
QUANDO Register chega com outro código e o mesmo nome, concorrente ou não
ENTÃO rejeita com resource-scheduling/resource/duplicate-name pelo desfecho, sem evento, e o replay da mesma chave devolve a mesma rejeição sem nova auditoria

DADO duas reservas do mesmo recurso e uma de outro
QUANDO FindBookingByResource consulta pelo primeiro recurso
ENTÃO devolve exatamente as duas, fora de qualquer UoW
```

## Escopo fora

- **Capacidade do recurso**: o contexto não valida disponibilidade nem
  estoque; `quantity` é só um limite de forma.
- **Consumo de eventos**: `bookings` não consome nada; sem inbox, sem
  consumer.
- **Composition root**: o esqueleto de `app/` (config, wiring, telemetria,
  catálogo e `app/rpc`) e o `cmd/` com `--role` nascem do generator; o harness
  escreve o conteúdo de domínio dentro deles.
