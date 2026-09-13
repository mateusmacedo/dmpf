# Diagramas e Metodologia — Guia resumido

> **Fonte:** [`docs/dmpf/diagramas-metodologia.md`](../diagramas-metodologia.md) | **Status:** Guia derivado (não normativo) | **Linhas:** ~1.600 → ~280
>
> **Propósito:** Visualizar a metodologia construtiva, os seis blocos em runtime, e como o DMPF transforma evidência em norma.

---

## O sistema de transformação

O DMPF opera em **duas passagens**:

1. **Fundação (normatização):** Evidência + necessidades → Regras decidíveis com ID estável
2. **Execução (certificação):** Kernels, verticais, testes → Softwares auditáveis

Não há "implementação sem norma" nem "norma sem implementação verificada".

---

## Fluxo das transformações

```
Entradas                 Fundação                    Execução                   Saídas auditáveis
   ↓                       ↓                            ↓                            ↓
Inventário AS-IS    → Classificação            → Implementar kernels       → Sistemas com
Problema/NFRs          Modelagem UPR                Verticais                   fronteiras explícitas
P0/RFC/Âncoras        Definir UoW+outbox         Testes/certificação         Contratos versionados
Plataforma            Definir wire (Protobuf)     Pilotos                    Evidência por regra
Operação              Escolher transporte         Métricas                   BOM + runbooks
                      Derivar diagnóstico
```

---

## Uma vertical pelos 6 blocos

Fluxo simplificado de dados em **runtime**:

```
Entradas externas
 │ REST (JSON)
 │ gRPC (Protobuf)
 │ Kafka (CloudEvent)
 ↓
┌─────────────────────────────────────┐
│ app — Adapter                       │
│ • Autentica, valida, mapeia         │
│ • Composition root                  │
└──────────────┬──────────────────────┘
               ↓
┌─────────────────────────────────────┐
│ contract — Wire types              │
│ • CloudEvents, Protobuf            │
│ • Versionamento                    │
└──────────────┬──────────────────────┘
               ↓
┌─────────────────────────────────────┐
│ port — Capacidades requeridas      │
│ • Repository, Outbox, Cache        │
│ • Interface sem implementação       │
└──────────────┬──────────────────────┘
               ↓
┌─────────────────────────────────────┐
│ application — Orquestração          │
│ • Sequência de passos               │
│ • Transação local (UoW)             │
│ • Delegação a portas                │
└──────────────┬──────────────────────┘
               ↓
┌─────────────────────────────────────┐
│ domain — Regra pura                │
│ • UPR: recebe entrada, devolve      │
│   Decision (exaustiva, exclusiva)   │
│ • Zero I/O                          │
└──────────────┬──────────────────────┘
               ↓
┌─────────────────────────────────────┐
│ provider — Realização              │
│ • BD, cache, Kafka, HTTP            │
│ • Implementa portas                 │
└──────────────┬──────────────────────┘
               ↓
Saídas (efeitos)
 │ Outbox
 │ Métricas
 │ Logs
```

---

## Estrutura dos diagramas no artefato

O arquivo [`diagramas-metodologia.md`](../diagramas-metodologia.md) contém:

1. **Metodologia como sistema** — como evidência vira norma, norma vira código
2. **Uma vertical pelos 6 blocos** — UPR e fluxo de dados
3. **Fluxo síncrono** — REST ou gRPC, request-response
4. **Fluxo assíncrono** — Kafka (publish-subscribe), outbox-inbox
5. **Estados de domínio** — máquina de estados executável
6. **Decomposição dos failure modes** — causa raiz, sinal, contenção
7. **Matriz de rastreabilidade** — cada diagrama aponta para ID estável na fonte

---

## Quando ler

- **Comece aqui se:** quer entender como UPR flui pelas camadas, ou quer ver a relação entre síncrono/assíncrono
- **Pule direto se:** já sabe a norma e quer só conferir coerência entre representações

**Lembre:** Os diagramas são visuais derivados. Se divergirem de um artefato com ID estável, prevalece a norma. Toda caixa aponta de volta à fonte.
