# Governança, BOM, Pilotos — FND-10 | Resumo

> **Fonte:** [`docs/dmpf/governanca-bom-pilotos.md`](../governanca-bom-pilotos.md) | **Âncora:** `ANC-08`, `ANC-09` | **Status:** Promovido para revisão | **Linhas:** ~2.100 → ~240
>
> **Propósito:** Quando e como um artefato vira norma; BOM (bill of materials); pilotos e prontidão.

---

## Governança: 4 estados observáveis

| Estado | Significado | Ação |
|--|--|--|
| **definido** | Artefato redigido e em review | Aguarda | 
| **vigente** | Artefato aceito (norma aplicável) | Aplicar |
| **âncora fechada** | Todas as sub-specs completadas | Nenhuma |
| **AC fechado** | Acceptance criteria do épico satisfeito | Nenhuma |

**GOV-02:** Uma regra não pode invocar as duas âncoras (ANC-08 e ANC-09) como se fossem um único passaporte.

---

## Autoridade (AUT): rito de controle

Mudança de `block` ou `bounded_context` é **mudança normativa** e exige autorização:

| Ato | Owner | Processo |
|--|--|--|
| Criar unidade nova | Titular do artefato + revisor independente | PR + aprovação em commit separado |
| Reclassificar unidade | Titular + revisor independente | Idem |
| Remover unidade | Titular + revisor independente | Idem |
| Mover arquivo (reclassifica) | Titular + revisor | Idem |

**Até FND-10 (agora):** Mecanismo mínimo = commit próprio + aprovação distinta.

---

## BOM: 6 itens

Bill of Materials do DMPF — o que está pronto e o que falta:

| Item | Status | O quê |
|--|--|--|
| **Norma** | ✓ Completa | RFC + 9 sub-specs (FND-03 a FND-10) |
| **Kernels** | ◐ Parcial | Go ~80%, TypeScript ~20% |
| **Verificador** | ✗ Ausente | Linter de dependências (FND-11) |
| **Pilotos** | ◐ 2/3 | Go (`shared-titulos`), TypeScript (`legado-live`) |
| **Métrica de adoção** | ◐ Parcial | Cobertura em 3/10 repos do baseline |
| **Runbooks operacionais** | ✗ Ausente | Delegada a operação |

---

## Pilotos (PIL): 3 + métricas

| Piloto | Stack | Escopo | Status |
|--|--|--|--|
| **`legado-titulos-shared-services`** | Go (híbrido) | Domínio + transação + outbox + Kafka | Ativo |
| **`legado-live-services`** | TypeScript (Nest) | Domínio + SQS | Ativo |
| **`legado-sync-services`** | Go | Sync + SNS/SQS + Postgres | Ativo |

**Métricas coletadas:**
- Tempo de implementação por camada
- Taxa de conformidade (lint)
- Taxa de regressão
- Velocidade de test

---

## Prontidão (RDY): 7 itens de DoR

Checklist pré-piloto:

1. ✓ Norma completa e revisada
2. ◐ Kernels implementados (partial)
3. ✗ Verificador pronto (falta)
4. ✓ Casos de uso mapeados
5. ◐ Ambiente de teste (partial)
6. ✓ Métricas definidas
7. ◐ Time treinado (ongoing)

---

## Estados abertos (não fechados)

| Abertura | Status | Dona |
|--|--|--|
| **G1** | Âncora ANC-08 — 5 gates externos | Governance  |
| **G2** | Âncora ANC-09 — Cronograma de migração | Governance |
| **G3** | BOM item 3 — Verificador | FND-11  |
| **G4** | BOM item 6 — Runbooks operacionais | Operação |
| **G5** | RFC versão 0.2 — Revisão por áreas (pós-0.1) | RFC owner |

Nenhuma destas pode ser saltada ou declar satisfeita por ausência.

---

## Escape hatches: quando sair

**E1:** Conformidade não alcança 60% após 3 sprints → avaliar viabilidade  
**E2:** Bloqueador crítico não resolvido em 2 sprints → escalar  
**E3:** Custo organizacional excede ROI → interromper

**GOV-26:** Escape hatch E1–E3 são exclusivos — nenhum outro serve como saída.

---

## Dilataçõ es (não decisões)

Tópicos que faltam da ANC-09 mas foram encaminhados por irmãos:

- Adoção organizacional (delegada a operação)
- Faseamento (delegada a operação)
- Governança de feature flags (fora de escopo)

**GOV-01:** `ADO-00` suspende a força normativa de `ADO-01`..`ADO-08` até a ampliação explícita.

---

## Contradição pré-existente (registrada)

`SPEC-DBTRMM3X` reserva 15 slots em `docs/adr/010`–`024` e cobra 15 ADRs mínimos. O acervo tem 19 acionamentos (`ADR-DMPF-A`..`ADR-DMPF-S`).

**Resolução:** Faixa ampliada para `010`–`028` (19 IDs) pelo FND-11, sob autorização de RFC §13.2.

---

## Pendências (P1–P13)

Tabela viva em §8.4: cada uma com dona, condição de fechamento e status.  
Ver [`docs/dmpf/governanca-bom-pilotos.md`](../governanca-bom-pilotos.md) para lista completa.

---

**Próximos passos:** FND-11 (ADRs), monitoramento de prontidão (RDY-*).
