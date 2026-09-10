# Políticas de Transporte — FND-06 | Resumo

> **Fonte:** [`docs/dmpf/politicas-transporte.md`](../politicas-transporte.md) | **Âncora:** `ANC-04` | **Status:** Promovido para revisão | **Linhas:** ~2.600 → ~380
>
> **Propósito:** Definir políticas de cada transporte (REST, gRPC, Kafka, SNS/SQS) e garantir byte-preservação do payload.

---

## Os 7 transportes

| Transporte | Síncrono? | Interno? | Politica principal |
|--|--|--|--|
| **REST** | ✓ Sim | Não | JSON via HTTP, Swagger, idempotência por chave |
| **gRPC** | ✓ Sim | ✓ Sim | Protobuf, HTTP/2, deadline, health check |
| **Kafka** | ✗ Não | ✓ Sim | Particionado, ordenado, **alvo da fundação** |
| **SQS** | ✗ Não | Não | Batch, deduplicação FIFO, visibilidade timeout |
| **SNS** | ✗ Não | Não | Fan-out, FilterPolicy, precedência com SQS |
| **AsyncAPI** | — | — | Catálogo obrigatório (não é transporte, é documentação) |
| **Coexistência** | — | — | Um canal = um transporte (nunca os dois) |

---

## REST: políticas

| Política | Regra |
|--|--|
| **Idempotência** | POST com chave declarada no contrato; PATCH nunca |
| **Timeout** | Deadline = prazo do caller − folga (RES-03) |
| **Retry** | Apenas idempotente + código transiente; corpo repetível (`GetBody`) |
| **Admission** | `429 Too Many Requests` **antes** de ler corpo (RES-17) |

---

## gRPC: políticas

| Política | Regra |
|--|--|
| **Prazo** | Multiplicar hops (A → B → C): cada camada reduz prazo de chamada (GRP-04) |
| **Retry** | Idempotente + código transiente declarado; nativo do gRPC desligado |
| **Health** | `grpc.health.v1.Check` por serviço, healthcheck em health check |
| **Admission** | Por método + tenant; recusa com `RESOURCE_EXHAUSTED` antes do handler (MET-12) |

---

## Kafka: alvo da fundação

**Decisão:** Kafka é o transporte **assíncrono-alvo** do DMPF. SQS/SNS continuam suportados, mas Kafka tem prioridade normativa.

| Aspecto | Regra |
|--|--|
| **Tópico** | Endereço lógico; partição dada pelo envelope (KFK-01) |
| **Partição** | Chave = envelope `id` (para ordenação por entidade) |
| **Offset** | Consumidor persiste offset; `CommitRecords` do prefixo contíguo |
| **Retry** | Partição pausada até limite do canal (TRP-47) |
| **DLQ** | Envelope intacto quando contenção atingida |

---

## SQS: políticas

| Aspecto | Regra |
|--|--|
| **Envelope** | Base64 uma única vez; SNS sem raw delivery → recusada (SQS-02) |
| **Grupo/dedup** | FIFO: grupo = contexto limitado; dedup = SHA-256 do envelope (SQS-05/06) |
| **Visibilidade** | Heartbeat pausado antes de qualquer gesto; teto 12h (SQS-08/08b) |
| **Ack** | Delete pelo receipt handle **após** commit |
| **Release** | Encurta visibilidade (SQS-09/10) |

---

## Byte-preservação: matriz de conformidade

Caminho de reentrega de um envelope sem reserializar:

| Via | Preserva | Status | Nota |
|--|--|--|--|
| Kafka native | ✓ Sim | Conforme | Bytes intactos |
| SQS raw mode | ✓ Sim | Conforme | Base64 uma vez |
| SNS raw + SQS | ✓ Sim | Conforme | SNS encapsula, SQS extrai |
| SNS sem raw | ✗ Não | **Não conforme** | SNS reserializa JSON |
| gRPC JSON transcode | ✗ Não | **Não conforme** | Transcodifica proto→JSON |

**Crítico:** FND-05 `payload_hash` depende de byte-preservação. Se via falhar → hash diverge → consumidor rejeita.

---

## Coexistência: um canal, um transporte

Uma mesma mensagem **não pode** sair simultaneamente por Kafka e SQS.

Regra: `same_channel AND single_transport`.

```
canais permitidos:
  - pedidos:kafka
  - pagamentos:sqs
  - notificacoes:kafka

canais proibidos:
  - X:kafka,sqs (mesma mensagem por dois)
```

---

## Prazo (deadline) e retry

**GRP-04:** Cada hop (A→B→C) reduz o prazo de quem chama.  
**RES-27:** Retry é conjunção: erro retentável **E** idempotência **E** orçamento **E** prazo remanescente.

---

## Observabilidade por transporte

Cada via tem **três posições** (RES-23):

1. Admissão (recusa?)
2. Tentativa (em voo)
3. Desfecho (sucesso, falha, retry)

---

## Pendências

- **FND-05 §10.4:** Localização do validador de perfil — encaminhada a FND-06 + co-decisão
- **Tabela sucessão RFC §14.4:** Falta consolidar todas as quatro linhas

---

## Próximos passos

- **Erro:** FND-07 (categorias, mapeamento para código HTTP)
- **Resiliência:** FND-08 (retry, observabilidade, runbook)
- **Testes:** FND-09 (cenários por transporte)
