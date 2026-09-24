# Playbook QA → Dev: insumos DMPF para verificação automatizada

Guia **derivado e não normativo**. A norma mora em [FND-09](../dmpf/testes-interop.md).
O Dev implementa o SUT e adapta o código ao kit; o QA versiona o **esperado**.

Workshop com o exemplo já no repositório: [dmpf-qa-workshop-orders-reservations.md](./dmpf-qa-workshop-orders-reservations.md).
Levantamento de bounded context: [dmpf-implementation.md](./dmpf-implementation.md) §2.

---

## 1. O que o QA entrega

Três artefatos no repositório (não planilha solta, não caso no Jira sem arquivo):

| Artefato | Caminho | Prova |
| --- | --- | --- |
| Spec de domínio | `docs/specs/` | UPRs, rejeições, eventos, consumidores |
| Fixture de **projeção** | `contracts/fixtures/<ctx>/projection/v1/<agregado>.golden` | `domainkit` (`ORA-30`) |
| Fixture de **wire** + `.proto` | `contracts/proto/…` e `contracts/fixtures/<ctx>/event/v1/<evento>.golden` | `golden` + gates Buf |

OpenAPI em `contracts/openapi/` **não** substitui a golden de evento.

---

## 2. Regras de forma (projeção)

- JSON, `format_version` `"1"`. Carregador que não conhece a versão **falha** (`FIX-09`).
- **Todo escalar é string** — número, booleano, timestamp, decimal (`FIX-07`).
- Valores **fixos e literais**. Nada de “agora”, UUID aleatório ou cálculo na hora do teste (`FIX-08`).
- O `doc` de cada caso explica o que o caso discrimina. JSON não tem comentário.
- Código de rejeição: `contexto/motivo` (`DEC-09`). Não coloque HTTP/gRPC na UPR.
- Ramo `rejected`: `events` vazio e `state_after` **igual** a `state_before` (`ORA-38`).
- Pelo menos um caso **positivo** e um **negativo** por UPR material. Sem o negativo, o oráculo invertido passa.

O Dev **não** inventa o esperado. Se o teste falhar, ou o código está errado ou a fixture precisa de revisão explícita no PR — nunca `GOLDEN_UPDATE=1` para “fazer passar”.

---

## 3. Template de projeção

Copie para `contracts/fixtures/<contexto>/projection/v1/<agregado>.golden`.
Troque os placeholders. Mantenha as aspas em todo escalar.

```json
{
  "format_version": "1",
  "identity": {
    "context": "<contexto>",
    "aggregate": "<agregado>"
  },
  "cases": [
    {
      "name": "<upr>-accepted",
      "doc": "Ramo Accepted de <UPR>: <o que muda e qual evento entra na sequência>.",
      "state_before": {
        "id": "<id-estável>",
        "status": "<estado>"
      },
      "command": {
        "upr": "<upr-em-kebab>",
        "at": "1755432000"
      },
      "expected": {
        "branch": "accepted",
        "response": {
          "id": "<id-estável>"
        },
        "events": [
          {
            "name": "<contexto>.<evento-no-passado>",
            "fields": {
              "id": "<id-estável>",
              "at": "1755432000"
            }
          }
        ],
        "state_after": {
          "id": "<id-estável>",
          "status": "<estado-depois>"
        }
      }
    },
    {
      "name": "<upr>-<motivo>",
      "doc": "Ramo Rejected de <UPR>: <invariante>; sequência vazia e estado idêntico (ORA-38).",
      "state_before": {
        "id": "<id-estável>",
        "status": "<estado>"
      },
      "command": {
        "upr": "<upr-em-kebab>",
        "at": "1755432000"
      },
      "expected": {
        "branch": "rejected",
        "rejection": {
          "code": "<contexto>/<motivo>",
          "message": "<frase estável em inglês, a mesma do domínio>",
          "details": {
            "limit": "1"
          }
        },
        "events": [],
        "state_after": {
          "id": "<id-estável>",
          "status": "<estado>"
        }
      }
    }
  ]
}
```

Referência viva: `contracts/fixtures/orders/projection/v1/order.golden`.

---

## 4. Fixture de wire (quando há evento)

A golden de evento **acompanha o PR do `.proto`** (`FIX-10`, `INT-02`). Um arquivo por contrato-major (`FIX-11`).

O QA descreve, em linguagem de negócio + campos:

- identidade (`type` CloudEvents, `dataschema`, package/message Protobuf);
- casos canônicos (condicionais presentes e ausentes);
- discriminadores (enum zero/`UNSPECIFIED`, inteiro que estoura `double`, nanos no timestamp, campo extra).

O Dev/plataforma preenche `payload_bytes_hex` e `payload_hash` com o gerador do módulo de contratos. QA **revisa o diff**; não gera bytes à mão.

Detalhe de formato: FND-09 §5 (`FIX-05`…`FIX-13`) e `contracts/README.md`.

---

## 5. Checklist de handoff (PR de insumos)

### QA preenche antes de pedir implementação

- [ ] Agregado, chave estável e estados nomeados na spec.
- [ ] Cada entrada de API/mensagem aponta para **uma** UPR.
- [ ] Cada UPR material tem caso `accepted` e pelo menos um `rejected`.
- [ ] Códigos de rejeição estáveis `contexto/motivo`; `details` só com o que o domínio já expõe.
- [ ] Eventos no passado, sem versão e sem nome de transporte (`MSG-N`).
- [ ] Consumidor de cada evento declarado (quem, chave natural, o que é idempotente).
- [ ] Projeção em `contracts/fixtures/…/projection/v1/` com escalares-string e `at` literal.
- [ ] Se publica evento: `.proto` + esboço dos casos da golden de wire no mesmo PR ou no PR imediatamente seguinte, nunca “depois do código”.
- [ ] Controle negativo: se inverter `accepted`/`rejected` no arquivo, o teste do Dev **tem** de quebrar.

### Dev devolve no PR de implementação

- [ ] `domainkit` + `tb.LoadProjection` contra o arquivo do QA (`testkit/domainkit/fixture_test.go` é o molde; o teste vive no `domain` do contexto).
- [ ] `ReadTwice` / determinismo em toda UPR.
- [ ] Service no `serviceskit` se a UPR atravessa UoW/outbox.
- [ ] Golden de wire no `golden.Evaluate` se o evento foi publicado.
- [ ] `appkit`/`distkit` (harnesses do contexto, em `apps/backend/reservations`) só quando o aceite exige persistência + broker, não para “cobrir domínio com integração”.

### Fora do escopo do QA de negócio

Fitness/`conformance`, fakes de relógio, harness Postgres, `Skipped` de provider. Isso é plataforma.

---

## 6. Como o Dev prova o que o QA escreveu

| Insumo | Package | Comando típico |
| --- | --- | --- |
| Projeção | `domainkit` | `pnpm nx run testkit:test-race` (`TestTheCounterMatchesTheProjectionFixture`, o molde) e `pnpm nx run <ctx>:test-race` (o teste de projeção do `domain` do contexto) |
| Wire | `golden` | mesmo target; dono das fixtures: `contracts` |
| `.proto` | Buf | `pnpm nx run contracts:buf-lint` (e demais gates do módulo) |
| Consumo / reentrega | `appkit`, `distkit` (em `apps/backend/reservations`) | `pnpm nx run reservations:test-race` com `DMPF_PG_DSN`; `reservations:test-distributed` com Redpanda |

Veredicto por valor: lista de diagnósticos com o ID da regra. `tb.Require` converte em falha de teste.

---

## 7. O que não fazer

- Caso só no Jira, esperado implícito no teste Go.
- Número JSON sem aspas (`"item_limit": 3`).
- Rejeição com status HTTP na fixture de domínio.
- Evento no ramo recusado.
- Atualizar golden sem revisar campo a campo.
- Duas cópias da fixture (uma “do QA” e uma “do Go”).
