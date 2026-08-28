# Ledger de reconciliação — DMPF

Estado vivo das pendências cruzadas do acervo `docs/dmpf/`.

Este documento **não é artefato promovido**: fica fora do rito de monotonicidade
M1–M4 e da faixa de âncoras da RFC. Ele existe porque o acervo tem uma
consequência temporal que nenhum artefato pode resolver sozinho — quando um
irmão posterior quita uma pendência registrada num artefato anterior, o registro
original caduca, e o princípio de não editar artefato promovido impede que a
quitação seja anotada onde a pendência nasceu. Sem um registro vivo, a pendência
segue anunciada como aberta para sempre.

O ledger **aponta, nunca copia**: cada entrada cita artefato e seção, e não
replica texto normativo. A norma continua morando no artefato; aqui mora apenas o
estado dela.

Escopo: pendências que mudaram de estado **depois** da promoção do artefato que
as registrou. Pendências com tabela viva em artefato promovido — como a de
FND-10 §8, com `P1`–`P13` — são referenciadas como fonte, não duplicadas.

Fonte: SPEC-4W1BQK93 (ARQ-492).

## Estado atual

Tabela mutável e **autoritativa**: uma linha por pendência, sempre o estado
vigente. Estados admitidos: `aberta`, `quitada` e `parcial` — este último com a
partição declarada na própria célula.

| ID | Pendência | Registrada em | Estado | Tratada por | Evidência |
|----|-----------|---------------|--------|-------------|-----------|
| `REC-001` | Vetores de equivalência do desfecho e diagnóstico estável por regra | FND-03 §10.4 | `quitada` | FND-09 §3.2 (obrigações `H3-5` e `H3-6`) e FND-09 §13.4 | as 54 regras de FND-03 estão instanciadas no mapa por regra |
| `REC-002` | Failure modes traduzidos em cenários executáveis | FND-04 §11.5 | `quitada` | FND-08 §9 (rastreabilidade 1:1) e FND-09 §4 | 12 de 12 failure modes com sinal observável e cenário |
| `REC-003` | Glossário: 20 termos do FND-04 a incorporar em RFC §14.1 | FND-04 §11.5 | `aberta` | owner da RFC | — |
| `REC-004` | Realinhamento do critério de round-trip e do ownership da fixture | FND-05 §10.4 | `quitada` | reconciliação feita antes da redação do FND-09 — §1, §8.3 e §8.4 | o critério são os três oráculos, não «mesmo byte» em geral; o ownership está em FND-09 §8.4 |
| `REC-005` | Byte-preservação do payload pelos transportes | FND-05 §10.4 | `quitada` | FND-06 §5 (matriz de hops) | quatro caminhos `não conforme` nomeados na matriz |
| `REC-006` | Localização do validador do perfil de envelope | FND-09 §14 (`RAS-41`) | `parcial` | instrumento **quitado** em FND-09 §11.6; localização **encaminhada**, por depender de co-decisão com FND-06 | FND-06 §18.2, obrigação 15 |
| `REC-007` | Atualização da tabela de sucessão de RFC §14.4 | FND-04 §11.5, FND-06 §18.3, FND-07 §11.4 e FND-08 §13.4 | `aberta` | owner da RFC — consolidar as quatro numa passagem única | proposta de consolidação em FND-07 §11.4 |
| `REC-008` | Reconciliação FND-10 × FND-09 (pendência `P9`) | FND-10 §8 | `quitada` | verificação registrada no README do acervo | sem colisão de prefixo e sem obrigação nova |

## Histórico

Seção **append-only**: uma entrada datada por mudança de estado. Nenhuma entrada
é reescrita ou removida — quando o estado muda de novo, entra uma linha nova.

- 2026-08-27 — `REC-001`..`REC-008`: inventário inicial, a partir da revisão de
  congruência horizontal de 2026-08-22 (SPEC-4W1BQK93, ARQ-492). Cinco entram
  como `quitada`, duas como `aberta` e uma como `parcial`. As duas pontas de cada
  registro — a pendência de origem e a quitação que a fechou — foram conferidas
  contra o acervo antes desta entrada.

## Regras do ledger

1. A tabela de **estado atual** é a autoridade e pode ser reescrita: uma
   pendência tem exatamente uma linha vigente.
2. O **histórico** nunca é reescrito. Ele é a trilha, não o estado.
3. Toda mudança na tabela exige uma entrada nova no histórico, com data, estado
   anterior, estado novo, motivo e evidência.
4. Entradas citam artefato e seção (`FND-09 §3.2`), nunca replicam texto
   normativo — pela convenção de citação do README do acervo.
5. Uma pendência entra como `parcial` apenas com a partição declarada na célula:
   o que foi quitado e o que segue encaminhado, com o destinatário nomeado.
6. Pendência que já tem tabela viva em artefato promovido é referenciada como
   fonte. Duplicá-la aqui criaria duas verdades envelhecendo em ritmos
   diferentes — exatamente o defeito que este ledger existe para evitar.
