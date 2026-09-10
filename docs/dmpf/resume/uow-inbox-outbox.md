# UoW, Inbox, Outbox — FND-04 | Resumo

> **Fonte:** [`docs/dmpf/uow-inbox-outbox.md`](../uow-inbox-outbox.md) | **Âncora:** `ANC-02` | **Status:** Promovido para revisão | **Linhas:** ~1.800 → ~320
>
> **Propósito:** Transação local (`UoW`), outbox (drenagem de mensagens) e inbox (recepção idempotente).

---

## Unit of Work (`UoW`)

Transação local, **visível no service** (em código). Não é ORM-específica; é contrato:

```
UoW<R>:
  Within<R>(fn: (tx) => Promise<R>): Promise<R>
```

Semântica:
- Envolve a função em transação (`BEGIN` / `COMMIT` / `ROLLBACK`)
- A função recebe o identificador da TX (`tx`)
- Se lançar exceção → `ROLLBACK`
- Se retornar → `COMMIT` antes de devolver ao caller

**Invariante:** A UoW **não sabe** quem chama; ela não coordena entre serviços — é local, ponto.

---

## Outbox: escrita e drenagem

Padrão para at-least-once com idempotência.

### Escrita (dentro da UoW)

```
1. application service abre UoW
2. executa regra de negócio
3. grava estado novo (clientes, pedidos, etc.)
4. **grava mensagem na tabela `outbox`**
5. transação commita atomicamente (estado + mensagem)
```

**Crítico:** Escrita da outbox é **dentro** da mesma transação do estado. Se uma falhar, a outra falha junto.

**Bloco responsável:** `application service` (ela decide o que gravar).

### Drenagem (fora da UoW)

Processo separado (pode ser outro service, outro app, scheduler):

```
1. SELECT * FROM outbox WITH (SKIP LOCKED) LIMIT N
2. Para cada mensagem:
   a. Publicar no broker (Kafka, SQS, etc.)
   b. DELETE FROM outbox WHERE id = …
   c. commit incrementalmente
```

**Bloco responsável:** `app` (é um processo próprio, tem lifecycle).

**Garantia:** Se o DELETE falhar → mensagem fica na outbox → será retentada.  
Se o publicação falhar → DELETE não ocorre → mensagem permanece → será retentada.

---

## Inbox: recepção idempotente

Padrão para consumidor não reprocessar a mesma mensagem duas vezes.

### Estrutura

```
CREATE TABLE inbox (
  message_id UUID PRIMARY KEY,
  payload BYTEA,
  created_at TIMESTAMP,
  processed_at TIMESTAMP,
  status ENUM('pending', 'processed', 'quarantine')
)
```

### Fluxo

1. Mensagem chega de Kafka/SQS/...
2. INSERT INTO inbox (message_id, payload) ON CONFLICT DO NOTHING
   - Se `message_id` já existe → linha 2 não faz nada (idempotência!)
   - Se nova → insere e prossegue
3. SELECT * FROM inbox WHERE status = 'pending'
4. Para cada mensagem: processar (invocar case de uso)
5. UPDATE inbox SET status = 'processed' WHERE message_id = …

**Garantia:** Mesmo que a mensagem chegue 100 vezes, o `message_id` único força a inserção uma só vez, e o status garante que apenas um processador a trata.

---

## Garantias entregues (e **não** entregues)

| O que promete FND-04 | O que **não** promete |
|--|--|
| ✓ At-least-once (mensagem chega ≥1 vez) | ✗ Exactly-once (nunca, por §2.3 P0-3) |
| ✓ Deduplicação por `message_id` | ✗ Ordem global (apenas por partição em Kafka) |
| ✓ Idempotência (mesma mensagem → mesmo efeito) | ✗ Replay inteligente (aplicativo decide) |
| ✓ Transacionalidade local (UoW) | ✗ Distribuída (fica com FND-06) |

---

## Os 5 blocos (BLK)

Responsabilidade de cada um:

| Bloco | O que | Regra |
|--|--|--|
| **application** | Gravar outbox na mesma TX do estado | `OBX-*` |
| **provider (persistência)** | Tabela outbox + drenagem (SKIP LOCKED) | `OBX-*` |
| **app (drenagem)** | Publicar mensagens e remover da outbox | `OBX-*` |
| **application (recepção)** | Lógica do caso de uso após chegar a mensagem | `INB-*` |
| **provider (inbox)** | Tabela inbox + deduplicação | `INB-*` |

---

## Failure modes e sinais

FND-04 enumera 12 failure modes (o que pode dar errado). Cada um tem **sinal observável** em FND-08 (observabilidade) e **cenário de teste** em FND-09:

- Outbox não comitou → mensagem perdida
- Drenagem travou → backlog acumula
- Inbox cheia → rejeição por capacidade
- Processamento duplicado → idempotência falha
- ... (8 mais)

---

## Três adições por ADR (pendentes)

- `ADR-DMPF-K`: Relay — mecanismo de polling vs. CDC (delegado a FND-11)
- `ADR-DMPF-L`: Serialização — quando e onde (delegado a FND-11)
- `ADR-DMPF-M`: Mapeamento — bloco e momento (delegado a FND-11)

---

## Pendências registradas

- `REC-002`: 12 failure modes em cenários — **quitada por FND-08** + FND-09
- `REC-003`: 20 termos do FND-04 para glossário RFC — **aberta**, owner RFC
- `REC-007`: Tabela de sucessão RFC §14.4 — **aberta**, consolidação

---

## O que este artefato **não** resolve

- Transporte específico (REST, gRPC, Kafka, SQS) → FND-06
- Observabilidade e métricas → FND-08
- Testes executáveis → FND-09
- Governança do schema → FND-10

---

**Próximos passos:** FND-05 (contratos), FND-06 (transportes).
