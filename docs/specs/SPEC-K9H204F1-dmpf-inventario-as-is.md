---
id: SPEC-K9H204F1
slug: dmpf-inventario-as-is
title: DMPF — Inventariar AS-IS e baseline
stage: done
priority: P0
depends_on: []
ticket_url: https://lider-cap.atlassian.net/browse/ARQ-438
subtask_urls: []
created: 2026-08-13
---
# SPEC-K9H204F1: DMPF — Inventariar AS-IS e baseline

## Resumo

Entregar o item lógico **FND-01** do épico ARQ-436
(ARQ-438): produzir a evidência «Inventário de padrões, libs existentes, NFRs, restrições e métricas atuais aprovado».

## Contexto

- **Umbrella**: [SPEC-QG2N8STY](./SPEC-QG2N8STY-dmpf-foundation.md)
- **Issue**: [ARQ-438](https://lider-cap.atlassian.net/browse/ARQ-438)
- **ACs do épico**: AC-01 (inputs), AC-11 (candidatos piloto)
- **Evidência §11**: Inventário de padrões, libs existentes, NFRs, restrições e métricas atuais aprovado

<constraints>
- [P0] Domínio sem I/O: domínio NÃO importa Protobuf, ORM, broker, SDK cloud, HTTP, logger ou framework
- [P0] Protobuf apenas no wire — NUNCA como modelo interno de domínio
- [P0] Semântica oficial at-least-once com efeitos idempotentes — NUNCA prometer exactly-once E2E
- [P0] Kernels Go/TS, adapters e providers de produção ficam FORA deste épico (só fundação normativa)
</constraints>

## Requisitos

### Funcionais

- [x] **[P0] Inventário de padrões**: documentar práticas atuais de mensageria, contratos, UoW e observabilidade
- [x] **[P0] Libs existentes**: listar componentes Go/TS relevantes e ownership
- [x] **[P0] NFRs e restrições**: capturar baseline e gaps vs. golden path
- [x] **[P1] Métricas atuais**: registrar o que já é medido (ou ausência) para calibrar pilotos
- [x] **[P1] Candidatos a piloto**: rascunho inicial (formalização em FND-10)

### Não-funcionais

- [x] **[P0] Rastreabilidade**: cada item do inventário nomeia owner e data de levantamento _(via defaults canônicos no cabeçalho do inventário + colunas onde aplicável)_
- [x] **[P0] Falseabilidade**: toda afirmação sobre o AS-IS aponta repositório, arquivo ou dashboard verificável
- [x] **[P0] Ausência explícita**: lacuna encontrada é registrada como "não medido / não existe", nunca omitida
- [x] **[P1] Estabilidade de referência**: seções numeradas para que FND-02…11 possam citá-las sem ambiguidade

## Camadas afetadas

Esta é uma spec **normativa**: as camadas abaixo são as do DMPF que o
artefato descreve ou normatiza, não módulos de código a alterar.

| Camada | Normatizada? | Descrição |
|--------|--------------|-----------|
| Domínio (UPR, Decision, eventos) | [x] | Levanta como o domínio é modelado hoje e onde há vazamento de I/O |
| Application service (UoW, orquestração) | [x] | Levanta fronteiras transacionais praticadas hoje |
| Port / Provider (adapters, drivers) | [x] | Lista libs, drivers e SDKs em uso, com ownership |
| Contract / wire (Protobuf, CloudEvents, OpenAPI) | [x] | Levanta contratos existentes e como evoluem hoje |
| Transporte (REST, gRPC, Kafka, SNS/SQS) | [x] | Levanta brokers, tópicos e convenções vigentes |
| Observabilidade e operação | [x] | Registra o que já é medido e o que não é |

Cobertura ampla por natureza: FND-01 é o levantamento que alimenta todas as
demais sub-specs.

## Localização de código

| Item | Caminho |
|------|---------|
| Asset de execução | [`SPEC-K9H204F1/prompt-inventario-repositorio.md`](./SPEC-K9H204F1/prompt-inventario-repositorio.md) — prompt que o agente executa em cada repositório para produzir o relatório de inventário |
| Draft de trabalho | `plans/references/inventario-as-is.md` — local, fora do versionamento (`plans/` está no `.gitignore`) |
| Promoção (ao ser aprovado) | `docs/dmpf/inventario-as-is.md` — versionado e revisável por PR |

## Design

### Estrutura do artefato

O inventário é um documento único, com cinco blocos numerados e estáveis:

1. **Padrões vigentes** — mensageria, contratos, UoW e observabilidade, por serviço.
2. **Libs e componentes** — Go e TypeScript, com owner e estado de manutenção.
3. **NFRs e restrições** — organizacionais (Kafka, AWS, segurança) e técnicas.
4. **Métricas atuais** — o que é medido hoje, onde, e o que não é medido.
5. **Candidatos a piloto** — rascunho para FND-10 consolidar.

### Fluxo

1. Coletar por serviço, a partir do código e dos dashboards existentes.
2. Consolidar no documento, marcando cada item com owner e data.
3. Submeter a Plataforma e Arquitetura para revisão.
4. Publicar como baseline; divergências posteriores viram ADR em FND-11.

## Decisões técnicas

- **Draft local, promoção ao ser aprovado**: o inventário nasce em
  `plans/references/inventario-as-is.md`, que é área local e **não versionada**
  (`plans/` está no `.gitignore`), e é promovido para `docs/dmpf/` quando
  aprovado. FND-02…11 citam seções numeradas dele, então a referência estável
  passa a existir a partir da promoção — é o artefato promovido que serve de
  baseline, não o draft. Alternativa descartada: manter o baseline apenas em
  `plans/`, porque um artefato não versionado não pode ser revisado por PR nem
  citado de forma estável pelas dependentes.
- **Ausência é dado**: lacunas entram como item explícito com owner. Alternativa
  descartada: omitir o não medido, porque produziria um baseline otimista e
  calibraria mal os pilotos de FND-10.
- **Sem julgamento de valor**: FND-01 descreve o AS-IS; a prescrição do
  golden path é de FND-02. Alternativa descartada: já propor o TO-BE aqui,
  porque antecipa decisões que a RFC ainda vai revisar.

## Verificação e testes

### Critérios de aceite

- [ ] **[P0] Inventário promovido para `docs/dmpf/inventario-as-is.md` e aprovado em PR pelos reviewers de Plataforma/Arquitetura** _(documento promovido; aprovação externa = gate pós-PR)_
- [x] **[P0] Libs e restrições organizacionais (Kafka/AWS/segurança) listadas com owner**
- [x] **[P0] Documento promovido sem TBD bloqueante**

### Cenários de teste (mínimo 3)

```
DADO o inventário com os cinco blocos preenchidos e cada item com owner
QUANDO submetido à revisão de Plataforma e Arquitetura
ENTÃO é aprovado sem nenhum TBD marcado como bloqueante

DADO um serviço em produção que não expõe métrica alguma de consumo
QUANDO esse serviço é inventariado no bloco 4
ENTÃO consta como "não medido" com owner e data, e a revisão prossegue

DADO um item de inventário que afirma um padrão vigente sem apontar
     repositório, arquivo ou dashboard que o comprove
QUANDO o revisor pede a evidência
ENTÃO o item é rejeitado até receber a referência verificável
```

<critical_constraints>
- [P0] Domínio sem I/O: domínio NÃO importa Protobuf, ORM, broker, SDK cloud, HTTP, logger ou framework
- [P0] Protobuf apenas no wire — NUNCA como modelo interno de domínio
- [P0] Semântica oficial at-least-once com efeitos idempotentes — NUNCA prometer exactly-once E2E
- [P0] Kernels Go/TS, adapters e providers de produção ficam FORA deste épico (só fundação normativa)
</critical_constraints>

## Escopo fora

- **Prescrição do TO-BE**: definir limites e regra de dependência é de FND-02
  (SPEC-8YVF0RR5); aqui só se descreve o estado atual.
- **Correção dos gaps encontrados**: o inventário aponta, não corrige — a
  remediação pertence aos épicos de kernel e migração.
- **Formalização dos pilotos**: charter e métricas de sucesso são de FND-10
  (SPEC-VVR1X71Q); aqui só o rascunho de candidatos.
- **Implementação de instrumentação**: registrar que uma métrica não existe é
  escopo; criá-la não é.
