---
id: SPEC-AHPRBZCT
slug: bookings
title: Bounded context — bookings
stage: building
priority: P2
depends_on: [SPEC-XMNBMY50]
ticket_url: null
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
  - `status: reserved | cancelled`
  - `reservedAt: instant`
- **Estados**: `reserved`, `cancelled`
- **Transições permitidas**:
  - (novo) → `reserved` — por `Reserve`
  - `reserved` → `cancelled` — por `Cancel`
- **Invariantes de estado**:
  - `status` é sempre um dos dois valores; uma reserva cancelada não sai de
    `cancelled` — `Cancel` sobre `cancelled` rejeita com
    `resource-scheduling/booking/not-reserved`.
  - `quantity` está sempre em `[1, 100]` — garantido na criação, porque
    nenhum comando a altera depois.

### Resource

- **Identidade**: `natural` por `code`
- **Campos**:
  - `code: string`
  - `registeredAt: instant`
- **Estados**: nenhum
- **Transições permitidas**: nenhuma
- **Invariantes de estado**:
  - `code` nunca é vazio — `Register` rejeita com
    `resource-scheduling/resource/code-empty`.

## Comandos (UPRs)

### Reserve

- **Agregado**: `Booking`; **cria**
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
- **Entrada**: nenhuma além da identidade do agregado
- **Pré-condições**:
  - `status = reserved` → `resource-scheduling/booking/not-reserved`
- **Efeitos sobre o estado**:
  - `status ← cancelled`
- **Eventos emitidos**: `BookingCancelled`
- **Resposta**: `bookingId: id`

### Register

- **Agregado**: `Resource`; **inicializa se ausente**
- **Entrada**:
  - `code: string`
- **Pré-condições**:
  - `code` não vazio → `resource-scheduling/resource/code-empty`
- **Efeitos sobre o estado**:
  - quando ausente: `registeredAt ← at`
  - quando presente: nenhum — aceita sem alterar e sem emitir
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
  - `at: instant ← at`

## Consultas

- `FindBookingByResource` — filtros: `resourceId`; cardinalidade: `many`

## Relações entre agregados

- `Booking.resourceId` → `Resource`

## Integração

- **Publica**: `BookingReserved` — vira
  `contracts/proto/company/bookings/event/v1/booking_reserved.proto`, package
  `company.bookings.event.v1`.
- **Consome**: nenhum.

## Políticas transversais

- **Idempotência**: o comando de criação (`Reserve`) entra por `POST` com chave
  de idempotência declarada; os demais são idempotentes por identidade do
  agregado (RST-02; FND-04).
- **Autorização**: pelo gancho de autorização do bloco `application`, antes da
  transação (FND-04 §3.2).
- **Auditoria**: pela trilha de auditoria do kernel (FND-08;
  `dmpf-observability/audit`).

## Critérios de aceite

- [x] Os gates do workspace verdes para `bookings-domain-go`,
  `bookings-ports-go`, `bookings-application-go`,
  `bookings-provider-postgres-go` e `bookings-app-go` (`fmt-check`, `vet`,
  `build`, `lint`, `test-race` com Postgres, verificador com `--base`).
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

DADO duas reservas do mesmo recurso e uma de outro
QUANDO FindBookingByResource consulta pelo primeiro recurso
ENTÃO devolve exatamente as duas, fora de qualquer UoW
```

## Escopo fora

- **Capacidade do recurso**: o contexto não valida disponibilidade nem
  estoque; `quantity` é só um limite de forma.
- **Consumo de eventos**: `bookings` não consome nada; sem inbox, sem
  consumer.
- **Publicação de `BookingCancelled` e `ResourceRegistered`**: ficam como
  eventos de domínio; só `BookingReserved` vira integration event nesta versão.
- **Composition root** (`cmd/` com `--role`): copiar `dmpf-reference`, fora do
  harness.
