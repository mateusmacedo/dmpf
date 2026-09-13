# Reconciliação — Estado das pendências | Guia resumido

> **Fonte:** [`docs/dmpf/reconciliacao.md`](../reconciliacao.md) | **Status:** Guia derivado (não normativo) | **Linhas:** ~1.200 → ~240
>
> **Propósito:** Rastreabilidade viva das pendências cruzadas do acervo — qual foi quitada (quando, por quem) e o que segue aberto.

---

## O que é este documento

Artefatos promovidos não são editados retroativamente. Quando um irmão posterior quita uma pendência registrada num artefato anterior, o ledger (livro de registro) carrega o estado vigente. É assim que se lê: "FND-03 registrou pendência X; foi quitada por FND-09 em seção Y; veja lá".

---

## Pendências: resumo visual

| ID | Assunto | Registrada em | Estado | Quitada por |
|--|--|--|--|--|
| `REC-001` | Vetores de equivalência do desfecho da `UPR` (`Decision`) | FND-03 (`upr-decision-mensagens.md`) | ✓ Quitada | FND-09 (testes) — 54 regras com par de cenários |
| `REC-002` | 12 failure modes do `UoW` em cenários executáveis | FND-04 (`uow-inbox-outbox.md`) | ✓ Quitada | FND-08 (resiliência) — cada um com sinal observável |
| `REC-003` | Incorporar 20 termos do FND-04 no glossário da RFC | FND-04 | ✘ Aberta | Owner da RFC `rfc-dmpf-foundation-v0.1.md` |
| `REC-004` | Critério de round-trip e ownership da fixture | FND-05 (`cloudevents-protobuf-buf.md`) | ✓ Quitada | FND-09 — reconciliada antes da redação; critério são os 3 oráculos |
| `REC-005` | Byte-preservação do payload pelos transportes | FND-05 | ✓ Quitada | FND-06 (`politicas-transporte.md`) — matriz de 4 caminhos não conforme |
| `REC-006` | Localização do validador de perfil de envelope | FND-09 (testes) | ◐ Parcial | Instrumento quitado; localização encaminhada (co-decisão com FND-06) |
| `REC-007` | Atualização da tabela de sucessão (RFC §14.4) | FND-04, FND-06, FND-07, FND-08 | ✘ Aberta | Owner da RFC — consolidar as 4 numa passagem única |
| `REC-008` | Reconciliação FND-10 × FND-09 (colisão de prefixos?) | FND-10 (`governanca-bom-pilotos.md`) | ✓ Quitada | Verificação registrada — sem colisão |
| `REC-009` | Ampliação da faixa de ADRs reservada (010–024 → 010–028) | FND-10 | ◐ Parcial | Faixa ampliada; ratificação encaminhada ao owner da RFC |
| `REC-010` | Reconciliação de cardinalidade dos 19 acionamentos `ADR-DMPF-A`..`S` | FND-10 | ✓ Quitada | FND-11 (`docs/adr/010`–`028`, um por acionamento) |
| `REC-011` | Owner da revisão de Segurança (gate `THR-03`) | FND-07 (`contexto-erros-seguranca.md`) | ◐ Parcial | ADR-029: papel atribuído ao author; re-revisão pendente (constituição de área) |

---

## Padrão: como ler

Para cada entrada:

1. **ID:** `REC-NNN` — identificador vivo deste ledger
2. **Pendência:** o que foi registrado (ex.: "20 termos faltam ao glossário")
3. **Registrada em:** artefato + seção que a declarou
4. **Estado:** `✓ Quitada` (fechada) / `✘ Aberta` (segue pendente) / `◐ Parcial` (parte fechada, parte encaminhada)
5. **Quitada por:** qual irmão, seção e qual evidência

---

## Três estados

| Estado | Significado | Ação |
|--|--|--|
| **✓ Quitada** | Condição de fechamento satisfeita; não há trabalho pendente | Nenhuma; registrada para rastreabilidade histórica |
| **✘ Aberta** | Condição não satisfeita; trabalho aguardando | Atribuir owner e priorizar |
| **◐ Parcial** | Parte satisfeita, parte encaminhada | Declares na célula: o que foi quitado; a quem coube o resto |

---

## Regra: não editar artefatos promovidos

Quando um artefato é promovido (passa de draft para normativo), ele não é editado retroativamente. Então:

- A pendência original **fica onde nasceu** (ex.: FND-03 §10.4).
- **Este ledger carrega o estado vigente** (ex.: `REC-001` diz que foi quitada por FND-09).
- Quem quiser saber o estado, consulta **este documento**, não volta ao original.

Disso decorre: este ledger é a fonte de verdade do estado. O artefato que a pendência nasceu é a fonte de contexto.

---

## Próximos passos (trabalho aberto)

| Pendência | Owner | Ação |
|--|--|--|
| `REC-003` | Owner da RFC | Incorporar 20 termos de FND-04 em `rfc-dmpf-foundation-v0.1.md` §14.1 |
| `REC-007` | Owner da RFC | Consolidar 4 linhas de sucessão da tabela de RFC §14.4 numa passagem única |
| `REC-009` | Owner da RFC | Ratificar ampliação de faixa `010`–`028` (19 IDs para 19 acionamentos) |
| `REC-011` | Área de Segurança | Re-revisar §7 e §8 com titular independente do author |

**Consulte o ledger completo** em [`docs/dmpf/reconciliacao.md`](../reconciliacao.md) para histórico datado e argumentação integral de cada estado.
