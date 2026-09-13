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
| `REC-009` | Ampliação da faixa reservada de ADRs de `010`–`024` para `010`–`028` | FND-10 §8.2 (obrigação `O12`) | `parcial` | faixa **ampliada** na RFC §13.1 e §13.2 pelo FND-11, sob a competência que a §13.2 lhe delega; **ratificação encaminhada** ao owner da RFC | RFC §13.1 e §13.2 passam a declarar `docs/adr/010`–`028`, faixa de 19 IDs — um por acionamento `ADR-DMPF-A`..`ADR-DMPF-S`. Três irmãos — `uow-inbox-outbox.md`, `resiliencia-observabilidade.md` e `cloudevents-protobuf-buf.md` — transcrevem a norma com a faixa antiga e seguem preservados |
| `REC-010` | Reconciliação de cardinalidade da série de ADRs, com a redação e a numeração dos acionamentos que faltavam | FND-10 §8.2 (obrigação `O12`), desdobrada em `P7` e `P8` na tabela de §8.4 | `quitada` | FND-11 (ARQ-448), a dona que §8.4 nomeia para as duas pendências | os 19 acionamentos `ADR-DMPF-A`..`ADR-DMPF-S` promovidos para `docs/adr/010`–`028`, um por acionamento; mapa das 15 linhas da tabela §8 do épico publicado em `docs/adr/README.md`, com as cinco sem ADR próprio e os seis acionamentos adicionais nomeados; FND-10 na tabela de cadência da `SPEC-DBTRMM3X`. As condições de fechamento seguem na tabela viva de FND-10 §8.4 |
| `REC-011` | Owner nomeado da revisão de Segurança, sem o qual o gate de `THR-03` não era acionável e o critério de aceite do threat model não podia ser marcado | FND-07 §7.10 e §11.4 (pendência 4); referenciada em FND-09 §14 (pendência 10) | `parcial` | ADR-029 (ARQ-488) — **quitado**: papel atribuído ao owner do artefato na ausência de área de Segurança constituída, e revisão de §7 e §8 executada. **Encaminhada**: a re-revisão por titular independente, condicionada à constituição da área | parecer em `revisao-seguranca-fnd-07.md`, desfecho `aprovado com ressalvas` — seis categorias conferidas nos sete vetores sem lacuna, sete exclusões com motivo e remissão verificada, 40 de 40 linhas com owner; as duas ressalvas são de insumo organizacional que o próprio FND-07 encaminha para fora das suas âncoras |

## Histórico

Seção **append-only**: uma entrada datada por mudança de estado. Nenhuma entrada
é reescrita ou removida — quando o estado muda de novo, entra uma linha nova.

- 2026-08-27 — `REC-001`..`REC-008`: inventário inicial, a partir da revisão de
  congruência horizontal de 2026-08-22 (SPEC-4W1BQK93, ARQ-492). Cinco entram
  como `quitada`, duas como `aberta` e uma como `parcial`. As duas pontas de cada
  registro — a pendência de origem e a quitação que a fechou — foram conferidas
  contra o acervo antes desta entrada.

- 2026-08-28 — `REC-009`: registro novo, sem estado anterior, entra como
  `parcial`. Motivo: os 19 acionamentos `ADR-DMPF-A`..`ADR-DMPF-S` do acervo não
  cabem nos 15 slots que a faixa `010`–`024` reservava, e a obrigação `O12` de
  FND-10 §8.2 encaminhou a reconciliação de cardinalidade ao FND-11. A ampliação
  para `010`–`028` foi executada nas duas linhas normativas da RFC (§13.1 e
  §13.2), sob a competência que a própria §13.2 delega ao FND-11, e classificada
  pela §14.2 como correção de redação — portanto sem incremento de versão, já que
  a §14.3, que o tornaria bloqueante, não é acionada. Fica `parcial` porque a
  ratificação é do owner da RFC.

  Efeito colateral aceito: três artefatos irmãos — `uow-inbox-outbox.md`,
  `resiliencia-observabilidade.md` e `cloudevents-protobuf-buf.md` — transcrevem
  a norma da §13.2 com a faixa antiga, cada um no bloco `normativo` que declara
  provisório o seu identificador `ADR-DMPF-*`. O de `resiliencia-observabilidade`
  transcreve entre aspas e endereça a RFC §13.2 por número de linha, então a
  transcrição passa a divergir da fonte. Nenhum foi editado: artefato promovido
  não se corrige retroativamente, e é este ledger que carrega o estado vigente.
  O verificador de referências não acusa a divergência porque sua âncora é a
  primeira linha do bloco citado, que a edição não tocou — o que envelheceu é o
  conteúdo transcrito, não o endereço.

- 2026-08-30 — `REC-010`: registro novo, sem estado anterior, entra como
  `quitada`. Motivo: a obrigação `O12` de FND-10 §8.2 encaminhou ao FND-11 a
  reconciliação de cardinalidade da série de ADRs, e a tabela de §8.4 a desdobra
  em duas pendências com a mesma dona — `P7`, cuja condição é mapa de
  consolidação decidido e faixa reservada compatível com o número de
  acionamentos, mais a inclusão de FND-10 na tabela de cadência; e `P8`, cuja
  condição é a promoção de `ADR-DMPF-R` e `ADR-DMPF-S` para `docs/adr/` com as
  alternativas descartadas de §8.1. As duas condições foram conferidas contra a
  entrega antes desta linha: a faixa `010`–`028` reserva 19 IDs para os 19
  acionamentos; o mapa está publicado no índice de `docs/adr/`, endereçando as 15
  linhas do épico e os 6 acionamentos sem linha correspondente; FND-10 consta da
  tabela de cadência; `R` e `S` saíram como `027` e `028`, e as tabelas de
  alternativas dos dois conferem uma a uma com o registro de acionamento de §8.1.

  Uma linha, não quatro. `O12`, `C7`, `P7` e `P8` são a mesma obrigação vista de
  quatro tabelas do FND-10: §8.2 a enuncia, a tabela de condições a lista como
  `C7`, e §8.4 a desdobra no par `P7`/`P8`. Pela regra 6 deste ledger, pendência
  que já tem tabela viva em artefato promovido é referenciada como fonte, não
  duplicada — quatro linhas aqui criariam quatro verdades para um estado só,
  envelhecendo em ritmos diferentes.

  O que esta entrada **não** quita: a ratificação da ampliação da faixa, que
  segue com o owner da RFC em `REC-009`; e as âncoras `ANC-08` e `ANC-09`, que
  §8.4 mantém abertas por gates alheios a esta entrega. `C7` deixa de ser
  condição pendente de `ANC-09`, mas a âncora permanece aberta por `C1`, `C3`,
  `C4` e `C6`.

- 2026-08-30 — `REC-011`: registro novo, sem estado anterior, entra como
  `parcial`. Motivo: a pendência 4 de FND-07 §11.4 registrava que o cabeçalho do
  artefato declara o **papel** do revisor de Segurança — «um representante de
  Segurança para §7 e §8, obrigatório» — e não a pessoa, e que sem ela o gate de
  `THR-03` não era acionável nem o critério de aceite do threat model podia ser
  marcado. A ARQ-488 migrou o gate para fora do FND-07 e o tornou entregável.

  O que foi **quitado**. O ADR-029 atribuiu o papel ao owner do artefato, na
  ausência de área de Segurança constituída na organização — condição conferida
  contra o acervo: nenhum documento nomeia titular, e o `CODEOWNERS` deste
  repositório segue com o placeholder do template, mesma ausência que o ADR-028
  já registrara em 10 dos 10 repositórios do inventário AS-IS. As vias
  alternativas foram examinadas e estão fechadas por norma: `GOV-31` limita o
  escape hatch a `E1`, `E2` e `E3`, nenhum dos quais alcança `THR-03`, e
  `GOV-32` `N1` veda exceção sobre constraint P0. A revisão foi então executada
  sobre §7 e §8, e o parecer está em `revisao-seguranca-fnd-07.md`.

  O que segue **encaminhado**: a re-revisão por titular independente da autoria,
  condicionada à constituição da área de Segurança. É por isso que a entrada é
  `parcial` e não `quitada` — o ADR-029 registra a condição, e o parecer nasce
  com validade limitada por ela.

  Divergência declarada, não contornada. A descrição da ARQ-488 separa os dois
  papéis — «o assignee atual responde pelo encaminhamento, não pela revisão» — e
  a atribuição escolhida contraria esse texto. A perda de independência entre
  autor e revisor está registrada no ADR-029, no cabeçalho do parecer e na §1.3
  dele, com as garantias compensatórias que a limitam sem a eliminar: a camada
  mecânica da conferência é reproduzível por terceiros, e a camada de julgamento
  fica isolada em seção própria.

  Nenhum artefato promovido foi editado. O FND-07 continua registrando a
  pendência onde ela nasceu, e o FND-09 §14 continua listando a pendência 10 que
  a referencia — é esta linha que carrega o estado vigente das duas, pela mesma
  regra que `REC-009` aplicou aos três irmãos que transcrevem a faixa antiga.

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
