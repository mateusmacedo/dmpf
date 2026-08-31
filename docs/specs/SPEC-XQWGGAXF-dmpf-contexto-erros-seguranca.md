---
id: SPEC-XQWGGAXF
slug: dmpf-contexto-erros-seguranca
title: DMPF — Execution context, erros, segurança e multi-tenancy
stage: done
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

- [x] **[P0] Execution context** imutável, criado no adapter, passado ao application service
- [x] **[P0] Identidade/tenant** só após autenticação
- [x] **[P0] Taxonomia de erros**: código estável, categoria, retryability, mapeamento por transporte
- [x] **[P0] Threat model**: adapters, desserialização, contratos, mensageria, secrets, PII, permissões
- [x] **[P0] Multi-tenancy** na autorização e no acesso ao dado, com isolamento
      fail-closed como resultado normatizado — o mecanismo de persistência que o
      impõe é encaminhado (ver «Escopo fora»)

### Não-funcionais

- [x] **[P0] Imutabilidade do contexto**: uma vez criado, o execution context não é alterado por nenhuma camada a jusante
- [x] **[P0] Estabilidade do código de erro**: o código é contrato público; mudar seu significado é breaking change
- [x] **[P0] Isolamento fail-closed**: sem tenant resolvido, ou diante de acesso a dado de outro tenant, a operação falha de forma observável — nunca devolve resultado vazio ou parcial —, e a imposição não depende de convenção de código
- [x] **[P0] Ausência de PII em log**: dados sensíveis são redigidos na origem, não filtrados no destino
- [x] **[P1] Cobertura do threat model**: os sete vetores listados têm mitigação nomeada e owner

### Obrigações herdadas de artefatos irmãos

FND-03 ([SPEC-8MNDEWDP](./SPEC-8MNDEWDP-dmpf-upr-decision-mensagens.md)), FND-04
([SPEC-7PJ5WVCS](./SPEC-7PJ5WVCS-dmpf-uow-inbox-outbox.md)) e FND-05
([SPEC-7H08RZDG](./SPEC-7H08RZDG-dmpf-cloudevents-protobuf-buf.md)) — todos
mergeados — delegaram **24 obrigações atômicas** a este item. Seis não constavam
desta spec e passam a constar:

| Obrigação | Assunto | Fonte |
|-----------|---------|-------|
| EC-2 | Semântica de cancelamento propagado pela cadeia | `upr-decision-mensagens.md` |
| MP-2 | Mapeamento de borda da application response, incluído o caminho de sucesso | `upr-decision-mensagens.md` |
| TX-4 | Formato do campo de diagnóstico de erro (`last_error`) | `uow-inbox-outbox.md` |
| SEC-3 | Minimização do dado de negócio | `uow-inbox-outbox.md`, `cloudevents-protobuf-buf.md` |
| SEC-4 | Cifra em repouso do dado de negócio | `uow-inbox-outbox.md`, `cloudevents-protobuf-buf.md` |
| SEC-6 | Teto de retenção do dado de negócio, que prevalece sobre prazo operacional quando mais estrito | `uow-inbox-outbox.md` |

Três delas — SEC-3, SEC-4 e SEC-6 — fecham a fronteira que o FND-04 deixou
declarada em texto: «declara onde o dado vive e não o protege». A matriz completa
das 24, com cadeia de prova por linha, é seção do artefato promovido.

## Camadas afetadas

Esta é uma spec **normativa**: as camadas abaixo são as do DMPF que o
artefato descreve ou normatiza, não módulos de código a alterar.

| Camada | Normatizada? | Descrição |
|--------|--------------|-----------|
| Domínio (UPR, Decision, eventos) | [x] | Define como a rejeição de domínio se converte em erro tipado |
| Application service (UoW, orquestração) | [x] | Recebe o execution context e aplica autorização por tenant |
| Port / Provider (adapters, drivers) | [x] | Cria o execution context e mapeia erro para o transporte |
| Contract / wire (Protobuf, CloudEvents, OpenAPI) | [x] | Normatiza a origem e a propagação do valor de identidade e tenant; a forma dos campos do envelope é do FND-05 (`ENV-11` fecha o conjunto) ou da especificação CloudEvents |
| Transporte (REST, gRPC, Kafka, SNS/SQS) | [x] | Define o mapeamento de erro por transporte |
| Observabilidade e operação | [x] | Define redaction, auditoria separada e o que nunca vai a log |

## Localização de código

| Fase | Caminho |
|------|---------|
| Fontes | `Parte-1 §§11–14`, na convenção que a RFC fixa em §1.3; a referência versionada é `docs/dmpf/rfc-dmpf-foundation-v0.1.md` |
| Draft de trabalho | área local — fora do versionamento, não citável como fonte |
| Promoção (ao ser aprovado) | `docs/dmpf/contexto-erros-seguranca.md` — versionado e revisável por PR |

## Design

### Execution context

Criado no adapter, na borda, a partir da requisição autenticada. Imutável.
Atravessa o application service até os providers.

| Campo | Origem | Observação |
|-------|--------|------------|
| identidade | token validado | nunca vem de header não autenticado |
| tenant | token validado | obrigatório em toda leitura e escrita de dado de negócio |
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

O tenant participa da **autorização** e do **acesso ao dado**. Aparecer apenas em
log é insuficiente e explicitamente proibido.

O que este artefato normatiza é o **resultado**: acesso a dado de outro tenant é
impedido de forma fail-closed, e a imposição não depende de convenção de código.
O **mecanismo** que o realiza na persistência — constraint, RLS, chave composta —
é escolha do provider e do kernel; a **verificação executável** do isolamento é
entregável do FND-09. Ambos aparecem em «Escopo fora».

## Decisões técnicas

- **Contexto imutável criado na borda**: elimina a classe de bug em que uma
  camada intermediária troca o tenant. Alternativa descartada: contexto mutável
  enriquecido ao longo da cadeia, porque a origem de cada campo deixa de ser
  auditável.
- **Isolamento normatizado como resultado, não como mecanismo**: o artefato exige
  que o acesso a dado de outro tenant falhe de forma fail-closed, sem prescrever
  como a persistência o impõe. Alternativa descartada: exigir constraint de banco
  nominalmente, porque isso normatizaria escolha de provider — adição fora do
  escopo permitido da ANC-05, que M4 invalida ainda que tecnicamente correta. A
  defesa em profundidade permanece exigida pelo resultado: um único caminho sem
  filtro de tenant não pode devolver linha de outro tenant.
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

- [x] **[P0] Modelo de contexto e erros promovido para `docs/dmpf/contexto-erros-seguranca.md` e aprovado em PR** — PR #10, mergeado em `develop`
- [x] **[P0] Threat model revisado por Segurança no artefato promovido** — revisão executada em [ARQ-488](https://lider-cap.atlassian.net/browse/ARQ-488), sob [SPEC-DK8QQSDQ](./SPEC-DK8QQSDQ-revisao-seguranca-fnd-07.md). Titular do papel atribuído pelo [ADR-029](../adr/029-titular-da-revisao-de-seguranca-fnd-07.md), sem independência entre autor e revisor e com re-revisão condicionada à constituição da área de Segurança. Parecer em [`revisao-seguranca-fnd-07.md`](../dmpf/revisao-seguranca-fnd-07.md), desfecho **aprovado com ressalvas**
- [x] **[P0] Mapeamentos REST/gRPC/mensageria documentados**

### Cenários de teste (mínimo 3)

```
DADO uma requisição autenticada do tenant A que produz um execution context
QUANDO o application service executa uma query sem filtro explícito de tenant
ENTÃO nenhuma linha do tenant B é devolvida, e a tentativa falha de forma
     observável como erro — não como resultado vazio — qualquer que seja o
     mecanismo de persistência escolhido

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
- **Mecanismo de persistência do isolamento por tenant**: constraint, RLS ou
  chave composta são escolha do provider e do kernel. Aqui se normatiza o
  resultado fail-closed, não a técnica que o realiza.
- **Verificação executável do isolamento**: o instrumento de teste e o oráculo são
  de FND-09 ([ARQ-446](https://lider-cap.atlassian.net/browse/ARQ-446)), sob
  ANC-07. Aqui se declara o resultado que o teste de lá deve constatar.
- **Taxonomia corporativa de classificação de dados**: consumida como entrada
  quando existir. Este artefato produz o critério técnico de sensibilidade
  aplicável aos campos do próprio mecanismo — exigência que FND-04 e FND-05 lhe
  encaminharam —, e não a política corporativa.
