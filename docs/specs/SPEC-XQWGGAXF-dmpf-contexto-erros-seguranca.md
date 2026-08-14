---
id: SPEC-XQWGGAXF
slug: dmpf-contexto-erros-seguranca
title: DMPF — Execution context, erros, segurança e multi-tenancy
stage: backlog
priority: P0
depends_on: [SPEC-8YVF0RR5]
ticket_url: https://lider-cap.atlassian.net/browse/ARQ-444
subtask_urls: []
created: 2026-08-13
---
# SPEC-XQWGGAXF: DMPF — Execution context, erros, segurança e multi-tenancy

## Resumo

Entregar o item lógico **FND-07** do épico ARQ-436
(ARQ-444): produzir a evidência «Modelos comuns, mapeamentos de erro e threat model aprovados».

## Contexto

- **Umbrella**: [SPEC-QG2N8STY](./SPEC-QG2N8STY-dmpf-foundation.md)
- **Issue**: [ARQ-444](https://lider-cap.atlassian.net/browse/ARQ-444)
- **ACs do épico**: AC-08
- **Evidência §11**: Modelos comuns, mapeamentos de erro e threat model aprovados

<constraints>
- [P0] Domínio sem I/O: domínio NÃO importa Protobuf, ORM, broker, SDK cloud, HTTP, logger ou framework
- [P0] Protobuf apenas no wire — NUNCA como modelo interno de domínio
- [P0] Semântica oficial at-least-once com efeitos idempotentes — NUNCA prometer exactly-once E2E
- [P0] Kernels Go/TS, adapters e providers de produção ficam FORA deste épico (só fundação normativa)
</constraints>

## Requisitos

### Funcionais

- [ ] **[P0] Execution context** imutável, criado no adapter, passado ao application service
- [ ] **[P0] Identidade/tenant** só após autenticação
- [ ] **[P0] Taxonomia de erros**: código estável, categoria, retryability, mapeamento por transporte
- [ ] **[P0] Threat model**: adapters, desserialização, contratos, mensageria, secrets, PII, permissões
- [ ] **[P0] Multi-tenancy** em autorização, queries e constraints — não só em logs

### Não-funcionais

- [ ] **[P0] Imutabilidade do contexto**: uma vez criado, o execution context não é alterado por nenhuma camada a jusante
- [ ] **[P0] Estabilidade do código de erro**: o código é contrato público; mudar seu significado é breaking change
- [ ] **[P0] Isolamento verificável**: o vazamento entre tenants é detectável por teste, não apenas evitado por convenção
- [ ] **[P0] Ausência de PII em log**: dados sensíveis são redigidos na origem, não filtrados no destino
- [ ] **[P1] Cobertura do threat model**: os sete vetores listados têm mitigação nomeada e owner

## Camadas afetadas

Esta é uma spec **normativa**: as camadas abaixo são as do DMPF que o
artefato descreve ou normatiza, não módulos de código a alterar.

| Camada | Normatizada? | Descrição |
|--------|--------------|-----------|
| Domínio (UPR, Decision, eventos) | [x] | Define como a rejeição de domínio se converte em erro tipado |
| Application service (UoW, orquestração) | [x] | Recebe o execution context e aplica autorização por tenant |
| Port / Provider (adapters, drivers) | [x] | Cria o execution context e mapeia erro para o transporte |
| Contract / wire (Protobuf, CloudEvents, OpenAPI) | [x] | Define os campos de identidade e tenant no envelope |
| Transporte (REST, gRPC, Kafka, SNS/SQS) | [x] | Define o mapeamento de erro por transporte |
| Observabilidade e operação | [x] | Define redaction, auditoria separada e o que nunca vai a log |

## Localização de código

| Fase | Caminho |
|------|---------|
| Fontes | `plans/references/Parte-1-conceitual.md` §§11–14 |
| Draft de trabalho | `plans/references/` — local, fora do versionamento |
| Promoção (ao ser aprovado) | `docs/dmpf/contexto-erros-seguranca.md` — versionado e revisável por PR |

## Design

### Execution context

Criado no adapter, na borda, a partir da requisição autenticada. Imutável.
Atravessa o application service até os providers.

| Campo | Origem | Observação |
|-------|--------|------------|
| identidade | token validado | nunca vem de header não autenticado |
| tenant | token validado | obrigatório em toda query e constraint |
| correlation / causation | envelope ou borda | propagados sem alteração |
| tracing | propagação OpenTelemetry | |
| deadline | política de FND-06 | decresce ao longo da cadeia |

### Taxonomia de erros

Todo erro carrega quatro dimensões:

1. **Código estável** — contrato público, não muda de significado.
2. **Categoria** — validação, autorização, conflito de estado, indisponibilidade, interno.
3. **Retryability** — se repetir a operação pode ter desfecho diferente.
4. **Mapeamento por transporte** — como cada categoria aparece em REST, gRPC e mensageria.

Rejeição de domínio (a `Rejection` da `Decision`, de FND-03) é erro de negócio
e mapeia para categoria própria — nunca para erro interno.

### Threat model — vetores cobertos

Adapters, desserialização, contratos, mensageria, secrets, PII e permissões.
Cada vetor recebe ameaça, mitigação e owner.

### Multi-tenancy

O tenant participa da **autorização**, das **queries** e das **constraints de
banco**. Aparecer apenas em log é insuficiente e explicitamente proibido.

## Decisões técnicas

- **Contexto imutável criado na borda**: elimina a classe de bug em que uma
  camada intermediária troca o tenant. Alternativa descartada: contexto mutável
  enriquecido ao longo da cadeia, porque a origem de cada campo deixa de ser
  auditável.
- **Tenant em constraint de banco, não só em `WHERE`**: a constraint é a última
  linha de defesa quando o filtro é esquecido. Alternativa descartada: confiar
  no filtro da query, porque um único caminho sem filtro vaza dados entre
  tenants.
- **Código de erro estável como contrato público**: consumidores tratam por
  código. Alternativa descartada: mensagem de erro como identificador, porque
  qualquer melhoria de texto viraria breaking change.
- **Retryability explícita no erro**: o cliente não precisa adivinhar pela
  categoria. Alternativa descartada: inferir do código HTTP, porque o mesmo
  status cobre casos retentáveis e definitivos.
- **Redaction na origem**: o dado sensível nunca chega ao pipeline de log.
  Alternativa descartada: filtrar no agregador, porque a PII já transitou e
  pode ter sido persistida em buffer intermediário.

## Verificação e testes

### Critérios de aceite

- [ ] **[P0] Modelo de contexto e erros promovido para `docs/dmpf/contexto-erros-seguranca.md` e aprovado em PR**
- [ ] **[P0] Threat model revisado por Segurança no artefato promovido**
- [ ] **[P0] Mapeamentos REST/gRPC/mensageria documentados**

### Cenários de teste (mínimo 3)

```
DADO uma requisição autenticada do tenant A que produz um execution context
QUANDO o application service executa uma query sem filtro explícito de tenant
ENTÃO a constraint de banco impede o acesso a linhas do tenant B, e a violação
     é observável como erro, não como resultado vazio

DADO uma regra de negócio violada que gera Rejection na Decision
QUANDO o adapter REST mapeia o erro para a resposta HTTP
ENTÃO resulta em erro de negócio com código estável e retryability falsa,
     nunca em 500 interno

DADO um payload contendo campo classificado como PII
QUANDO o adapter registra a operação em log
ENTÃO o campo aparece redigido na origem, e o valor original não chega ao
     pipeline de log em nenhuma etapa
```

<critical_constraints>
- [P0] Domínio sem I/O: domínio NÃO importa Protobuf, ORM, broker, SDK cloud, HTTP, logger ou framework
- [P0] Protobuf apenas no wire — NUNCA como modelo interno de domínio
- [P0] Semântica oficial at-least-once com efeitos idempotentes — NUNCA prometer exactly-once E2E
- [P0] Kernels Go/TS, adapters e providers de produção ficam FORA deste épico (só fundação normativa)
</critical_constraints>

## Escopo fora

- **Implementação de autenticação e do provedor de identidade**: aqui se define
  o que o contexto carrega, não como o token é emitido.
- **Políticas de resiliência e observabilidade**: são de FND-08 (SPEC-E15TBHCD);
  aqui só o que toca segurança e redaction.
- **Gestão de secrets em produção**: o vetor entra no threat model; a operação é
  pós-fundação.
- **Classificação de dados da organização**: consumida como entrada, não
  produzida aqui.
