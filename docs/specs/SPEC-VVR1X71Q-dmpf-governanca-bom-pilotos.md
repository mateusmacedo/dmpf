---
id: SPEC-VVR1X71Q
slug: dmpf-governanca-bom-pilotos
title: DMPF — Governança, BOM, pilotos e backlog das próximas fases
stage: backlog
priority: P0
depends_on: [SPEC-K9H204F1]
ticket_url: https://lider-cap.atlassian.net/browse/ARQ-447
subtask_urls: []
created: 2026-08-13
---
# SPEC-VVR1X71Q: DMPF — Governança, BOM, pilotos e backlog das próximas fases

## Resumo

Entregar o item lógico **FND-10** do épico ARQ-436
(ARQ-447): produzir a evidência «Política de produto, template BOM, charter dos pilotos e épicos dependentes prontos».

## Contexto

- **Umbrella**: [SPEC-QG2N8STY](./SPEC-QG2N8STY-dmpf-foundation.md)
- **Issue**: [ARQ-447](https://lider-cap.atlassian.net/browse/ARQ-447)
- **ACs do épico**: AC-11, AC-12
- **Evidência §11**: Política de produto, template BOM, charter dos pilotos e épicos dependentes prontos

<constraints>
- [P0] Domínio sem I/O: domínio NÃO importa Protobuf, ORM, broker, SDK cloud, HTTP, logger ou framework
- [P0] Protobuf apenas no wire — NUNCA como modelo interno de domínio
- [P0] Semântica oficial at-least-once com efeitos idempotentes — NUNCA prometer exactly-once E2E
- [P0] Kernels Go/TS, adapters e providers de produção ficam FORA deste épico (só fundação normativa)
</constraints>

## Requisitos

### Funcionais

- [ ] **[P0] Governança**: SemVer, RFCs, release train, depreciação, suporte
- [ ] **[P0] BOM template**: runtimes, generators, drivers, SDKs, CVEs, combinações certificadas
- [ ] **[P0] Escape hatches**: exigem ADR, owner, justificativa e plano de convergência
- [ ] **[P0] Dois pilotos** reais com owners e métricas de sucesso
- [ ] **[P0] Épicos subsequentes** (kernel Go/TS, contratos, golden path, piloto) criados/vinculados e Ready
- [ ] **[P1] Fechamento**: consolidar após demais FND quando decisions críticas estiverem aceitas

### Não-funcionais

- [ ] **[P0] Escape hatch rastreável**: toda exceção concedida tem ADR, owner nomeado e prazo de convergência — exceção sem prazo é negada
- [ ] **[P0] Piloto mensurável**: cada piloto declara métrica de sucesso com valor de partida e alvo, não apenas intenção
- [ ] **[P0] Combinação certificada verificável**: o BOM declara versões testadas em conjunto, não faixas abertas
- [ ] **[P1] Cadência previsível**: o release train tem periodicidade declarada e política de depreciação com janela mínima

## Camadas afetadas

Esta é uma spec **normativa**: as camadas abaixo são as do DMPF que o
artefato descreve ou normatiza, não módulos de código a alterar.

| Camada | Normatizada? | Descrição |
|--------|--------------|-----------|
| Domínio (UPR, Decision, eventos) | [ ] | Não é objeto de governança de produto |
| Application service (UoW, orquestração) | [ ] | Idem |
| Port / Provider (adapters, drivers) | [x] | O BOM certifica drivers e SDKs por versão |
| Contract / wire (Protobuf, CloudEvents, OpenAPI) | [x] | O release train governa a evolução dos contratos |
| Transporte (REST, gRPC, Kafka, SNS/SQS) | [x] | O BOM certifica clientes de broker |
| Observabilidade e operação | [x] | Métricas de sucesso dos pilotos saem do catálogo de FND-08 |

## Localização de código

| Fase | Caminho |
|------|---------|
| Fontes | epic ARQ-436 §§11,20 |
| Draft de trabalho | `plans/references/` (charter e BOM) — local, fora do versionamento |
| Promoção (ao ser aprovado) | `docs/dmpf/governanca-bom-pilotos.md` — versionado e revisável por PR |

## Design

### Governança do produto DMPF

| Instrumento | Papel |
|-------------|-------|
| SemVer | versionamento do kernel e dos contratos |
| RFC | mudança normativa passa por RFC antes de virar norma |
| release train | cadência previsível de entrega |
| depreciação | janela mínima antes de remover algo suportado |
| suporte | quais versões recebem correção |

### BOM (Bill of Materials)

Declara **combinações certificadas** — o conjunto de versões testado em
conjunto, não faixas abertas por dependência:

runtimes (Go, Node) · generators (Buf e plugins) · drivers de banco ·
SDKs de cloud · clientes de broker · CVEs conhecidas e status.

### Escape hatches

Divergir do golden path é permitido e rastreado. Concessão exige quatro itens:
ADR registrando a decisão, owner nomeado, justificativa técnica e **plano de
convergência com prazo**. Sem os quatro, a exceção é negada.

### Pilotos

Dois fluxos reais, com owner e métrica de sucesso declarada (valor de partida e
alvo). São a evidência empírica de que a fundação funciona antes da adoção
ampla.

### Prontidão (AC-12)

Épicos subsequentes — kernel Go, kernel TS, contratos, golden path e piloto —
criados, vinculados e Ready, com riscos atribuídos a owner e decisão explícita
de continuidade.

## Decisões técnicas

- **Escape hatch com plano de convergência obrigatório**: exceção sem prazo
  vira padrão paralelo permanente. Alternativa descartada: conceder exceção por
  decisão de squad sem registro, porque o desvio deixa de ser visível e a
  fundação perde a função de padronizar.
- **BOM por combinação certificada, não por faixa de versão**: o que se testa é
  o conjunto. Alternativa descartada: declarar faixas semver amplas por
  dependência, porque a combinação específica que quebra nunca foi exercitada.
- **Dois pilotos, não um**: um único piloto confunde particularidade do fluxo
  com propriedade da fundação. Alternativa descartada: piloto único, porque não
  distingue o que generaliza do que era específico daquele contexto.
- **Métrica com valor de partida**: sem baseline não há como afirmar melhora.
  Alternativa descartada: métrica declarada só como alvo, porque o resultado
  fica ininterpretável.
- **Fechamento após as demais FND**: esta spec consolida, então encerra por
  último. Alternativa descartada: fechar em paralelo, porque a prontidão de
  AC-12 depende de decisões que só existem ao fim das outras sub-specs.

## Verificação e testes

### Critérios de aceite

- [ ] **[P0] Política de produto e BOM template promovidos para `docs/dmpf/governanca-bom-pilotos.md` e aprovados em PR**
- [ ] **[P0] Charter dos 2 pilotos assinado pelas squads**
- [ ] **[P0] AC-12 de prontidão atendido (riscos com owner; decisão de continuidade)**

### Cenários de teste (mínimo 3)

```
DADO uma squad que pede exceção ao golden path apresentando justificativa e owner
QUANDO a solicitação não traz plano de convergência com prazo
ENTÃO a exceção é negada até que o plano seja apresentado

DADO os dois pilotos com charter assinado e métrica de sucesso declarada
QUANDO o épico é avaliado contra AC-12
ENTÃO cada piloto exibe valor de partida e alvo, e cada risco aberto tem owner

DADO um consumidor que combina versões de runtime e driver não listadas como
     combinação certificada no BOM
QUANDO reporta incompatibilidade
ENTÃO o caso é tratado como fora do suporte, e a convergência para uma
     combinação certificada é a via de resolução
```

<critical_constraints>
- [P0] Domínio sem I/O: domínio NÃO importa Protobuf, ORM, broker, SDK cloud, HTTP, logger ou framework
- [P0] Protobuf apenas no wire — NUNCA como modelo interno de domínio
- [P0] Semântica oficial at-least-once com efeitos idempotentes — NUNCA prometer exactly-once E2E
- [P0] Kernels Go/TS, adapters e providers de produção ficam FORA deste épico (só fundação normativa)
</critical_constraints>

## Escopo fora

- **Execução dos pilotos**: aqui o charter; rodar o piloto é épico próprio.
- **Implementação dos épicos subsequentes**: aqui se garante que estão Ready.
- **Portal do desenvolvedor e generators**: pertencem aos épicos de tooling.
- **Preenchimento do BOM com versões reais**: aqui o template e a política; a
  primeira instância acompanha o kernel.
- **Migração dos serviços existentes**: pós-fundação.
