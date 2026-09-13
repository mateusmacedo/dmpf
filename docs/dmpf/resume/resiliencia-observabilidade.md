# Resiliência e Observabilidade — FND-08 | Resumo

> **Fonte:** [`docs/dmpf/resiliencia-observabilidade.md`](../resiliencia-observabilidade.md) | **Âncora:** `ANC-06` | **Status:** Promovido para revisão | **Linhas:** ~2.200 → ~260
>
> **Propósito:** Timeout, retry, observabilidade, runbooks e catálogo de métricas.

---

## Resiliência: os 4 pilares

| Pilar | Regra |
|--|--|
| **Timeout** | Prazo = orçamento do caller − folga operacional |
| **Retry** | Conjunção: transiente **E** idempotente **E** orçamento **E** prazo remanescente |
| **Circuito** | Falhas em cascata → circuit breaker (open → half-open → closed) |
| **Contenção** | Backpressure, DLQ, limite de tentar |

---

## Retry: condiçõ es conjuntivas (RES-27)

Todas as 4 devem ser verdadeiras:

```
if (error.retryable AND operation.idempotent AND budget.remaining AND deadline.remaining) {
  retry();
} else {
  fail();
}
```

**Crítico:** Orçamento por tentativa, **não** total. Cada tentativa reduz o orçamento remanescente.

---

## Observabilidade: 3 posições

Cada transição de tentativa tem 3 sinais:

1. **Admissão** — Recurso aceito ou rejeitado (429)
2. **Tentativa** — Em voo (latência, tenta)
3. **Resultado** — Sucesso, falha, retry (código, classe, modo)

---

## Catálogo de métricas (MET): por failure mode

12 failure modes de FND-04, cada um com sinal observável:

| Failure Mode | Sinal |
|--|--|
| UoW não comitou | Gauge / erro de categoria |
| Outbox não publicou | Counter (`outbox.messages_pending`) |
| Inbox cheia | Rejection rate (`inbox.admission.rejected`) |
| Conexão perdida | Latência spike + retry count |
| ... (8 mais) | |

**MET-05:** Limiar é **condicional** — derivado de invariante onde possível; parâmetro local senão. Não inventar números.

---

## Tracing (TRC): ponta a ponta

Rastreabilidade de requisição entre serviços:

- `correlationid` liga toda a cadeia
- `causationid` acrescenta sequência
- `traceparent` (W3C) carrega contexto de trace
- Cada serviço emite span com `operation_name`

---

## Log estruturado (LOG): JSON

Todos os logs estruturados, um evento por linha:

```json
{
  "timestamp": "2026-09-10T15:30:00Z",
  "level": "INFO",
  "service": "orders",
  "correlation_id": "corr-456",
  "message": "Order placed",
  "order_id": "123",
  "tenant": "acme"
}
```

---

## Runbook (RUN): 20 regras

Procedimento operacional para situações comuns:

- Quando circuit está open?
- Como esvaziar a DLQ?
- Como aumentar o timeout?
- Como pausar um consumidor?
- ... (16 mais)

Executável por quem está de plantão — sem necessidade de engenheiro.

---

## O que este artefato **não** cobre

- SLO por serviço (delegada a operação)
- Dashboards (delegada a operação)
- Alertas em produção (delegada a operação)
- Cache (FND-08 não toca)

---

## Pendências

- Validação do runbook por SRE — sem owner nomeado
- Aceite de `ADR-DMPF-Q` (OpenTelemetry baseline)
- Tabela de sucessão RFC §14.4

---

**Próximos passos:** FND-09 (testes), FND-10 (governança).
