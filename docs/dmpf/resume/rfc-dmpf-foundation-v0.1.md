# RFC DMPF Foundation v0.1 — Fundação normativa | Resumo

> **Fonte:** [`docs/dmpf/rfc-dmpf-foundation-v0.1.md`](../rfc-dmpf-foundation-v0.1.md) | **Status:** Normativo (aceito 2026-08-14) | **Linhas:** ~1.882 → ~450
>
> **Propósito:** Fundação arquitetural do DMPF — 6 blocos, regra de dependência, critérios P0 e contrato do verificador.

---

## Categorias de conteúdo

Todo bloco neste documento pertence a uma das quatro:

| Rótulo | Obriga? | Significado |
|--|--|--|
| `normativo` | ✓ **Sim** | Regra que trabalho novo deve cumprir |
| `rationale` | — | Justificativa de uma regra (contexto, alternativas descartadas) |
| `evidência` | — | Fatos datados do estado atual do código |
| `registro de extensão` | — | Ponto estável onde sub-spec futura adiciona conteúdo |

**Convenção de referência:** `§N` sem prefixo = seção desta RFC. `Parte-1 §N` = base conceitual (fora do repo). Sem qualificação, é ambíguo.

---

## Os quatro constraints P0 (inegociáveis)

São as únicas restrições que **não podem ser relaxadas** por sub-spec:

| P0 | Enunciado | Cláusula |
|--|--|--|
| **P0-1** | Domínio sem I/O — nada de Protobuf, ORM, broker, SDK, HTTP, logger, tracing | §6 (capabilities) + §7 (regra) |
| **P0-2** | Protobuf apenas no wire — nunca modelo interno | §4 (blocos) + §7 (regra) |
| **P0-3** | At-least-once com efeitos idempotentes — **nunca** exactly-once E2E | §2.3 + documentação |
| **P0-4** | Kernels e providers de produção fora da RFC — norma só, sem código | §1.4 (fronteira) |

---

## Os 6 blocos arquiteturais

Classificação **total**: todo arquivo de produção pertence a exatamente um bloco.

| Bloco | Responsabilidade | Exemplos de pertencimento | Exemplos de exclusão |
|--|--|--|--|
| **domain library** | Regra de negócio pura: estado, invariantes, decisões, UPR | Modelo de domínio, agregados, value objects, policies | Qualquer I/O, tipo gerado de wire, entidade de ORM, decorator de framework |
| **application service** | Orquestração: sequência, autorização, transação, persistência por porta | Caso de uso, delegação a portas, transação local | Regra de negócio (é domain), conhecimento de driver/broker/fila |
| **port** | Capacidade requerida: interface declarada pelo consumidor | Repositório, outbox, cache, cliente externo | Implementação, tipo de driver, tipo de wire |
| **provider** | Realização de porta com tecnologia concreta | Driver de BD, SDK, cliente HTTP, repositório concreto | Regra de negócio, decisão de caso de uso |
| **contract package** | Wire versionado: schemas, tipos gerados, envelopes | Protobuf, OpenAPI, AsyncAPI, tipos de wire | Lógica de negócio |
| **app** | Adapter de protocolo: autentica, valida, mapeia, composition root | Handler de endpoint, middleware, wiring | Regra de negócio, orquestração (é application) |

**Precedência de desempate** (nesta ordem):
1. Exclusão vence pertencimento
2. Responsabilidade dominante
3. Se mesmo assim empatado → divida a unidade (está mal dimensionada)

---

## Regra de dependência: matriz 6×6

**C1 — Bloco** (matriz abaixo) **E** **C2 — Contexto** (condição extra)

### Matriz (C1 — Bloco)

Linha = origem; coluna = destino. `P` = permitida, `✗` = proibida.

```
            domain  application  app  port  provider  contract
domain        P          ✗        ✗     ✗      ✗        ✗
application   P          P        ✗     P      ✗        ✗
app           P          P        P     P      P        P
port          P          ✗        ✗     P      ✗        ✗
provider      P          ✗        ✗     P      P        P
contract      ✗          ✗        ✗     ✗      ✗        P
```

**Leitura:** `domain → domain` é permitido (P) + contexto; `domain → provider` é proibido sempre (P0-1).

### Condição C2 — Contexto

Aresta permitida por C1 também exige **C2**:

- `same_bounded_context` (origem e destino têm `bounded_context` idêntico) **OU**
- `public_integration_surface` (destino é `contract package` **ou** declara `public_integration_surface: true`)

**Regra crítica:** Uma `domain library` **nunca** pode ser superfície pública. Portanto, `domain → domain` entre contexts é **sempre proibida**.

---

## Descoberta do universo

### Unidade arquitetural (verification_unit)

- **Em Go:** um **package** (diretório). Import path = canonical_key.
- **Em TypeScript:** um **root declarado no manifesto** `dmpf-units.json`. Chave = `<pacote>#<id>`.

Invariantes das 6 (I1–I6):
1. Cobertura total
2. Não sobreposição
3. Independência de tooling
4. Opacidade a alias
5. Move/rename explícito
6. Determinismo

### Bounded context

Declarado na mesma unidade, obrigatório, string estável. Razão: a mesma aresta de blocos tem veredicto diferente dentro/fora do context.

---

## Capabilities externas: política por bloco

| Bloco | Política |
|--|--|
| `domain` | **Deny-default:** apenas `pure` |
| `port` | **Deny-default:** apenas `pure` |
| `application` | **Deny-default com exceção:** `pure` e algumas `observability` (logger: **não**) |
| `contract` | **Restrita:** `pure` + `wire.codec` |
| `provider` | **Permissiva** (mas restrita à porta que implementa) |
| `app` | **Permissiva** (o composition root conhece tudo) |

**Observabilidade:** nunca permitida em `domain` nem `port`, mesmo sendo tecnicamente pura.

**Allowlist:** dependência externa só é utilizável se constar de allowlist com (pacote, versão, entrypoints, capability). **Pureza transitiva:** se o fechamento de transitividade importar capability fora de `pure`, a entrada deixa de ser `pure`.

---

## Bloco especial: outbox

Duas responsabilidades distintas, dois blocos:

| O que | Bloco | Por quê |
|--|--|--|
| **Escrita** na mesma TX | `application` | Faz parte do caso de uso |
| **Persistência** da tabela | `provider` | É tecnologia (SKIP LOCKED, backoff) |
| **Drenagem** e publicação | `app` | É processo próprio com lifecycle |

Resultado: `application → provider` permanece proibida, e a escrita é **por porta**.

---

## Domínio executável em memória

Critério com duas metades, ambas necessárias:

| Metade | Enunciado | Modo |
|--|--|--|
| **Estática** | Fechamento transitivo contém só `pure` | Import-verifiable |
| **Dinâmica** | Testes não iniciam processo, socket, disco nem dependem de hora | Runtime-testable |

**Mocks **não** satisfazem.** Substituir I/O por mock não elimina o acoplamento — a dependência é a violação.

---

## Contrato do verificador (§10)

### Schema do metadado

Cada módulo com produção: `dmpf-units.json` (TS) ou equivalente Go:

```json
{
  "schema": "dmpf/units@1",
  "units": [
    {
      "id": "pedidos/domain",
      "block": "domain",
      "bounded_context": "pedidos",
      "public_integration_surface": false,
      "include": ["src/domain/**"]
    }
  ],
  "external": [{
    "package": "decimal.js",
    "versions": ">=10 <11",
    "capability": "pure"
  }],
  "exceptions": [{
    "unit": "pedidos/application",
    "dependency": "legacy-sdk",
    "reason": "migração",
    "owner": "<team>",
    "review_by": "2026-12-31"
  }]
}
```

**Ausência reprova.** Valores desconhecidos reprovam. **Herança não existe.**

### Trust model

Proteção contra reclassificação oportunista:

- `block` e `bounded_context` são **mudança normativa** (exigem autorização distinta da autoria)
- Baseline independente: cópia do manifesto, guardado fora do módulo
- Divergência reprova (`DMPF-T001`)
- Sem evidência de autorização → reprova (`DMPF-T002`)

Até FND-10 definir o processo: commit próprio + aprovação por revisor distinto.

### Diagnósticos estáveis

Cada classe de erro tem código rastreável:

- `DMPF-U001`–`U004`: Unidade (cobertura, sobreposição, chave, manifesto)
- `DMPF-M001`–`M003`: Manifesto (obrigatório, valor, duplicidade)
- `DMPF-T001`–`T002`: Trust (divergência, autorização)
- `DMPF-D001`–`D002`: Dependência (bloco, contexto)
- `DMPF-E001`–`E004`: Externals (capability, pureza, resolução, dinâmico)

---

## O que a RFC **não** decide

Delegado a sub-specs (ANC-01 a ANC-09):

| Tema | Sub-spec |
|--|--|
| UPR, Decision, mensagens | FND-03 (ANC-01) |
| UoW, inbox, outbox | FND-04 (ANC-02) |
| Protobuf, CloudEvents, Buf | FND-05 (ANC-03) |
| Transportes (REST, gRPC, Kafka, SQS) | FND-06 (ANC-04) |
| Contexto, erros, segurança | FND-07 (ANC-05) |
| Resiliência, observabilidade | FND-08 (ANC-06) |
| Testes | FND-09 (ANC-07) |
| Governança, BOM, pilotos | FND-10 (ANC-08, ANC-09) |
| ADRs | FND-11 (ANC-11) |

---

## Evidência vs. Norma

**Inventário não cria a norma.** Presença = ocorrência, não correção. Ausência = não encontrada, não inexistente.

- **Conformidade observada:** 11 de 36 células (exemplos reais encontrados)
- **Violação observada:** 5 células (distância do parque à norma conhecida)
- **Não observado:** 20 células (esperado — metade descreve arestas ninguém tentaria)

---

## Próximas etapas

- **Implementação:** Verificador Go/TS (FND-11)
- **Adoção:** Aplicar manifesto em repos existentes (épicos kernel)
- **Pilotos:** Certificar verticais ponta a ponta
- **Baseline:** Governança do metadado (FND-10)

**Consulte a norma completa** em [`docs/dmpf/rfc-dmpf-foundation-v0.1.md`](../rfc-dmpf-foundation-v0.1.md) para a matriz de 36 células, diagramas derivados, critérios mecânicos e restrições completas.
