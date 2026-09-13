# CloudEvents, Protobuf, Buf — FND-05 | Resumo

> **Fonte:** [`docs/dmpf/cloudevents-protobuf-buf.md`](../cloudevents-protobuf-buf.md) | **Âncora:** `ANC-03` | **Status:** Promovido para revisão | **Linhas:** ~1.800 → ~290
>
> **Propósito:** Envelope padrão (`CloudEvents`), serialização (`Protobuf`), governança de schema (`Buf`).

---

## CloudEvents: envelope padronizado

Formato único de envelope para toda mensagem assíncrona do DMPF.

### Estrutura

```
{
  "id": "order-123",
  "source": "https://orders.example.com/orders",
  "type": "com.example.orders.OrderPlaced",
  "datacontenttype": "application/protobuf",
  "time": "2026-09-10T15:30:00Z",
  "correlationid": "corr-456",
  "traceparent": "00-...",
  "dataschema": "https://schema-registry.../orders/v1",
  "data": <base64-encoded protobuf bytes>
}
```

### Campos obrigatórios

- `id`: Identificador único (chave de deduplicação)
- `source`: Origem (service que publicou)
- `type`: Categoria semantic (ex: `com.example.orders.OrderPlaced`)
- `data`: Payload (Protobuf bytes, único formato permitido — não JSON, não XML)
- `time`: Timestamp ISO8601
- `correlationid`: Rastreamento ponta a ponta
- `traceparent`: OpenTelemetry trace header

---

## Protobuf: wire format único

**Regra P0-2:** Protobuf **apenas no wire** — nunca como modelo interno de domínio.

### Decisões

1. **Modalidade de payload:** `proto_data` com `google.protobuf.Any`
   - `binary_data` e `text_data` do `oneof` oficial → **vedadas**
   - Nenhuma como fallback, nenhuma como exceção

2. **Versionamento:** semver de wire por `.proto`
   - Nova versão = novo diretório (`v1/`, `v2/`)
   - Schemas antigos permanecem e são suportados

3. **Geração:** Protoc + plug-ins (Go, TypeScript, etc.)
   - Código gerado: `gen/go/`, `gen/typescript/`
   - **Nunca** editado à mão (§4.5 regra 2)

---

## Buf: governança de repositório

Ferramenta Buf: lint, versionamento, registry de schemas.

### Estrutura do diretório `contracts/`

```
contracts/
  buf.yaml              # configuração Buf
  buf.work.yaml         # workspace multi-módulo
  <bounded_context>/v1/
    <topic>.proto       # schemas
  fixtures/             # golden fixtures para testes
```

### Gates Buf

Executados a cada PR (fail-closed):

1. `buf format` — formatação padronizada
2. `buf lint STANDARD` — regras de estilo
3. `buf breaking` — compatibilidade com versão anterior
4. Dupla geração — código idêntico em ambas as linguagens

**Violação em qualquer gate:** PR não mergea.

---

## Payload hash: fórmula

Hash SHA-256 dos **bytes do `Any`** do integration event, sem reserialização.

```
payload_hash = SHA-256(Any.value)  // os bytes transportados, como estão
```

**Invariante:** Byte-preservação entre transportes (FND-06 garante).

---

## O que este artefato **requer** de outros

FND-03 e FND-04 delegaram 10 obrigações:

| O quê | Status | Onde resolvido |
|--|--|--|
| Envelope obrigatório | ✓ Quitada | Este artefato §3–4 |
| Profobuf apenas wire | ✓ Quitada | Regra P0-2 |
| Payload hash fórmula | ◐ Parcial | Este artefato; byte-preservação é FND-06 |
| Atributos obrigatórios | ✓ Quitada | Este artefato §4.1 |
| Registry de escolha | ✗ Encaminhada | ADR-DMPF-N (delegada a FND-11) |
| OpenAPI / AsyncAPI | ◐ Parcial | Parte-1 §7.6–7 vigentes sem sucessor; ADR pendente |

---

## Diagramas: matriz de hops

Documenta cada caminho (transporte) e se preserva bytes:

| Via | Preserva bytes | Status |
|--|--|--|
| Kafka (native) | ✓ Sim | Conforme |
| SQS (raw mode) | ✓ Sim | Conforme |
| SNS raw + SQS | ✓ Sim | Conforme |
| gRPC (JSON transcoding) | ✗ Não | **Não conforme** |

---

## Pendências

- `REC-004`: Critério de round-trip e ownership de fixture — **quitada por FND-09**
- `REC-005`: Byte-preservação pelos transportes — **quitada por FND-06**
- 10 mais em §10.4 (localização de validador, realinhamento de termos, etc.)

---

## Próximos passos

- **Transporte:** FND-06 (políticas por caminho)
- **Testes:** FND-09 (golden fixtures, oráculos)
- **ADRs:** FND-11 (registry, codecs)
