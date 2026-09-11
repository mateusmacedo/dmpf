---
id: SPEC-4W1BQK93
slug: dmpf-consolidacao-horizontal
title: DMPF — Consolidação horizontal do acervo (sincronização, ledger de reconciliação e verificador)
stage: done
priority: P2
depends_on: []
ticket_url: null
subtask_urls: []
created: 2026-08-22
---
# SPEC-4W1BQK93: DMPF — Consolidação horizontal do acervo

## Resumo

A revisão de congruência horizontal de 2026-08-22 indexou programaticamente as
definições de regra dos artefatos promovidos em `docs/dmpf/` — 766 definições
primárias sob o contrato de extração de RF5, conferindo com os totais que os
próprios artefatos declaram (54, 64, 64, 131, 112, 121, 142 e 78) — e confirmou
que o acervo é internamente consistente. Encontrou, porém, defeitos de
concordância entre artefatos e dois riscos estruturais, concentrados no ponto de
entrada do acervo: o `docs/dmpf/README.md` afirma números e estados que os
artefatos promovidos já não sustentam, e pendências registradas em artefatos
imutáveis foram quitadas por irmãos posteriores sem que nenhum registro vivo
aponte a quitação.

Esta spec sincroniza o README com os artefatos promovidos, cria o ledger de
reconciliação que passa a ser o estado vivo das pendências cruzadas, aplica uma
errata fechada de forma em quatro definições e instala verificação mecânica de
congruência horizontal em CI — para que a classe inteira de defeito seja
detectada por máquina daqui em diante, e não por revisão manual. O draft desta
spec foi revisado por LLM externo (Codex) em 2026-08-22; os sete pontos
bloqueantes da revisão estão incorporados.

## Contexto

O acervo DMPF opera sob um princípio deliberado: artefato promovido não é
editado retroativamente — adições acontecem por âncora (RFC §12.3), sob as
regras de monotonicidade M1–M4, e "duplicar fontes criaria duas verdades que
envelheceriam em ritmos diferentes" (FND-03 §10.2). O princípio está correto e
esta spec não o reabre. Duas ressalvas o completam:

- **RFC §14.2 admite expressamente** correção de redação, exemplo ou evidência
  via PR, sem incremento de versão — é o fundamento da errata de RF3, que muda
  apenas a forma do marcador, nunca a norma.
- A consequência temporal não tratada: quando um irmão posterior quita uma
  pendência registrada num artefato anterior, o registro original caduca e
  **nenhum lugar do acervo diz isso**. Hoje o custo aparece em três formas:

1. **O README envelheceu**: a seção sobre o FND-09 descreve a versão
   pré-reconciliação do artefato ("seis fontes", "551 identificadores",
   "25 obrigações" — o promovido declara sete fontes, 672 identificadores e 31
   obrigações), e a seção sobre o FND-08 declara "118 regras" quando o artefato
   contém e declara 121 (`resiliencia-observabilidade.md` §13.1).
2. **Pendências quitadas seguem anunciadas como abertas**: FND-03 §10.4 afirma
   que o contrato de vetores "ainda não existe" no FND-09 — o FND-09 o quitou
   (matriz de §3.2, obrigações `H3-5`/`H3-6`; instanciação das 54 regras em
   §13.4) — e o README repete a afirmação caduca.
3. **Não há verificação mecânica**: as contagens declaradas, as citações de
   seção e as referências por linha física entre artefatos só são conferidas
   quando alguém as confere. A revisão que originou esta spec provou que a
   conferência é automatizável — e a revisão externa provou que ela exige um
   contrato de extração explícito (RF5) para ser decidível.

<constraints>
- [P0] Nenhuma mudança de semântica normativa nos artefatos promovidos: a
  errata de RF3 é lista fechada com antes/depois exato por linha, fundamentada
  em RFC §14.2, com diff auditável.
- [P0] A RFC (`rfc-dmpf-foundation-v0.1.md`) não é editada — qualquer mudança
  nela exige rito de versão (RFC §14.3), fora do escopo.
- [P0] Nenhuma inserção ou remoção de linha em artefato promovido: toda edição
  da errata é in-place (mesma linha, mesma contagem de linhas do arquivo). O
  PR de implementação reconfirma, antes do merge, que nenhuma linha citada por
  referência física se desloca (reverse-lookup de `arquivo:linha`).
- [P0] `plans/` permanece fora do versionamento (ADR-007); referências a
  `plans/` são escopo da SPEC-4CKAD0BC, não desta.
- [P1] O ledger de reconciliação aponta, nunca copia: entrada de ledger cita
  pendência e quitação por artefato e seção, sem replicar texto normativo.
- [P1] O verificador usa apenas a biblioteca padrão do Node — sem dependência
  nova no workspace.
</constraints>

## Requisitos

### Funcionais

- **RF1 [P0] — Sincronizar o README**: aplicar em `docs/dmpf/README.md` as
  **oito** correções da lista fechada da tabela de §Design — as sete afirmações
  desatualizadas mais a linha de índice do ledger de RF2, que a tabela
  `## Artefatos` ainda não tem. Nenhuma outra seção do README é alterada.
- **RF2 [P0] — Ledger de reconciliação**: criar `docs/dmpf/reconciliacao.md`
  com identificador estável por pendência (`REC-NNN`), tabela de **estado
  atual** (mutável, uma linha por pendência) e **histórico append-only** em
  seção separada (uma entrada datada por mudança de estado). O inventário
  inicial é fechado e está na tabela de §Design. Estados admitidos: `aberta`,
  `quitada`, `parcial` (com a partição declarada — ex.: instrumento quitado,
  localização encaminhada).
- **RF3 [P1] — Errata de forma (lista fechada)**: aplicar as oito edições
  in-place da tabela de §Design, sobre quatro definições — três `ESC` que passam
  à forma canônica de definição (uma linha cada), quatro linhas do caso `RAS-30`
  (o marcador órfão é esvaziado, o ID migra para a forma destacada de §11.1,
  aberta na 2162 e fechada na 2163, e sai do cabeçalho da tabela) e a distinção
  de contagem no total do FND-09 §13.1 (142 = 141 `normativo` + 1 `encaminhado`, `RAS-41`), alinhando o
  critério ao que o FND-10 já pratica. `ENV-25`, `INT-05` e `UPR-I04` **não**
  integram a errata: têm definição canônica em tabela e os parágrafos
  `` `normativo` — `ID` `` são qualificadores legítimos (forma reconhecida pelo
  contrato de RF5).
- **RF4 [P1] — Convenção de citação**: publicar em `docs/dmpf/README.md` a
  convenção para citações novas entre artefatos: citar por seção e por ID de
  regra (`FND-04 §5.3`, `OBX-12`), reservando `arquivo:linha` para quando a
  âncora semântica não existir; as referências por linha existentes permanecem
  e passam a ser verificadas mecanicamente (RF5-C4).
- **RF5 [P0] — Verificador de congruência horizontal**: criar
  `tools/dmpf-verify.mjs` com **contrato de extração explícito** (ver §Design) e
  sete checagens: (C1) unicidade de definição primária e ownership de prefixo;
  (C2a) o total declarado por cada índice confere com as definições extraídas;
  (C2b) as projeções do README conferem com as métricas-fonte; (C3) citações de
  seção com documento adjacente resolvem no alvo; (C4) referências físicas
  resolvem e o conteúdo apontado confere com o trecho registrado no manifesto;
  (C5) formas de definição e qualificação conformes à gramática, com o escopo
  fechado da forma destacada; (C6) faixas reservadas seguem não atribuídas.
  Interface: `--root <dir>` injetável (default: raiz do repositório), exit code
  ≠ 0 em qualquer violação, relatório por checagem.
- **RF6 [P1] — Gate em CI**: criar `.github/workflows/dmpf-verify.yml`
  executando **o harness e o verificador**, nessa ordem, em PRs que toquem
  qualquer um de quatro paths: `docs/dmpf/**`, `tools/dmpf-verify.mjs`,
  `tools/tests/dmpf-verify/**` e o próprio `.github/workflows/dmpf-verify.yml`
  — o gate não pode ser cego a mudanças em si mesmo nem no harness que o
  sustenta. Roda no `gitea-runner`, e o harness roda por `node --test`, porque o
  CI do Nx não o alcança: a raiz declara `test: {executor: nx:noop}` e o
  `ci.yml` a exclui por `NX_EXCLUDE`. Node 24 vem da action local que o `ci.yml`
  já consome, sem passo de instalação novo — o contrato documentado do runner
  (ADR-005) garante apenas `jq` e `curl`. Hardening: `permissions` read-only, `timeout-minutes`
  e cancelamento por concorrência, no estilo do `ci.yml`. Workflow dedicado
  porque o `ci.yml` ignora mudanças exclusivamente em `**/*.md`.

### Não-funcionais

- **RNF1 [P0]**: zero mudança de semântica normativa — auditável por diff; a
  revisão do PR confere que toda edição em artefato promovido está na lista
  fechada da errata.
- **RNF2 [P1]**: verificador roda em menos de 5 segundos em execução fria no
  runner de CI (`gitea-runner`), sobre o corpus atual (~20 mil linhas); o
  limite é revisitado se o corpus dobrar.
- **RNF3 [P1]**: todo texto novo em PT-BR com acentuação correta;
  identificadores e código permanecem em inglês.

## Camadas afetadas

Nenhuma camada de runtime. A mudança é inteiramente documental e de tooling:

| Camada | Impacto |
|--------|---------|
| Documentação (`docs/dmpf/`) | README sincronizado, ledger novo, 8 edições in-place em 2 artefatos |
| Tooling (`tools/`) | Script de verificação novo + fixtures de teste, sem dependências |
| CI (`.github/workflows/`) | Workflow novo, disparo por path |

## Localização de código

| Arquivo | Ação |
|---------|------|
| `docs/dmpf/README.md` | Editar (RF1, RF4) |
| `docs/dmpf/reconciliacao.md` | Criar (RF2) |
| `docs/dmpf/upr-decision-mensagens.md` | Editar in-place — definições `ESC-04`, `ESC-05`, `ESC-06` (3 linhas) |
| `docs/dmpf/testes-interop.md` | Editar in-place — caso `RAS-30` (4 linhas, §11.1) e total de §13.1 (1 linha) |
| `tools/dmpf-verify.mjs` | Criar (RF5) |
| `tools/tests/dmpf-verify/` | Criar — fixtures herméticas e vetores por checagem (RF5) |
| `.github/workflows/dmpf-verify.yml` | Criar (RF6) |

## Design

### Arquitetura

Três blocos independentes, integráveis em qualquer ordem, com o verificador por
último (ele valida os outros dois):

```text
[1] Sincronização (README)      [2] Ledger (reconciliacao.md)
        \                              /
         \                            /
      [3] Verificador (tools/dmpf-verify.mjs + workflow)
          — passa somente com [1] e [2] aplicados
```

### Tabela de correções do README (RF1 — lista fechada)

| # | Onde (seção do README) | Está escrito | Passa a refletir | Fonte da verdade |
|---|------------------------|--------------|------------------|------------------|
| 1 | Sobre `testes-interop.md` | "herda de **seis** fontes — a RFC e os **cinco** irmãos" | sete fontes: a RFC e os seis irmãos (FND-03 a FND-08) | `testes-interop.md` §1.4 |
| 2 | Sobre `testes-interop.md` | "a matriz de §3 trata **25** obrigações atômicas" | 31 obrigações (`H2-1`..`H8-6`) | `testes-interop.md` §3.2 |
| 3 | Sobre `testes-interop.md` | "**551** identificadores estáveis e **533** recebem linha", "os 18 restantes" | 672 identificadores; 664 estáveis; 654 com linha; 650 com mecanismo; 4 exceções nomeadas | `testes-interop.md` §1.4 e §13.1 |
| 4 | Sobre `testes-interop.md` | "**535** cláusulas `normativo` e 551 identificadores... divergência inteira na RFC" | 682 cláusulas e 672 identificadores; divergência na RFC e no FND-08 | `testes-interop.md` §1.4 |
| 5 | Sobre `resiliencia-observabilidade.md` | "As **118** regras `normativo`" | 121 regras (inclui `MET-05a`, `RUN-11a`, `RUN-18a`) | `resiliencia-observabilidade.md` §13.1 |
| 6 | Sobre `upr-decision-mensagens.md` | "encaminhados a FND-09 e **ainda inexistentes** (§10.4)" | pendência quitada pelo FND-09 (§3.2, `H3-5`/`H3-6`; instanciação em §13.4) — remissão ao ledger (`REC-001`) | `testes-interop.md` §3.2 e §13.4 |
| 7 | Sobre `cloudevents-protobuf-buf.md` | "o realinhamento com FND-09, cuja spec hoje exige «mesmo byte»" | a spec do FND-09 foi reconciliada antes da redação (critério = três oráculos) — remissão ao ledger (`REC-004`) | `testes-interop.md` §1, §8.3-8.4 |
| 8 | Tabela `## Artefatos` | (o ledger não é indexado) | linha nova para `reconciliacao.md`, com status **registro vivo** (não promovido, fora de M1–M4) e a célula Spec / Issue preenchida com `SPEC-4W1BQK93` / ARQ-492 | esta spec (RF2) |

### Formato do ledger (RF2)

`docs/dmpf/reconciliacao.md` com duas seções:

**Estado atual** (tabela mutável — uma linha por pendência, sempre o estado
vigente):

```markdown
| ID | Pendência | Registrada em | Estado | Tratada por | Evidência |
|----|-----------|---------------|--------|-------------|-----------|
| REC-001 | Vetores de equivalência do desfecho | FND-03 §10.4 | quitada | FND-09 §3.2 (H3-5, H3-6) e §13.4 | 54 regras de FND-03 instanciadas no mapa por regra |
| REC-002 | Failure modes → cenários executáveis | FND-04 §11.5 p3 | quitada | FND-08 §9 (1:1) + FND-09 §4 | rastreabilidade 12/12 com sinal |
| REC-003 | Glossário: 20 termos do FND-04 em RFC §14.1 | FND-04 §11.5 p2 | aberta | owner da RFC | — |
| REC-004 | Realinhamento round-trip/ownership de fixture | FND-05 §10.4 p9 | quitada | reconciliação da spec do FND-09 (§1, §8.3-8.4) | critério = três oráculos; ownership em §8.4 |
| REC-005 | Byte-preservação pelos transportes | FND-05 §10.4 p10 | quitada | FND-06 §5 (matriz de hops) | quatro caminhos `não conforme` nomeados |
| REC-006 | Localização do validador do perfil de envelope | FND-09 §14 / RAS-41 | parcial | instrumento quitado (FND-09 §11.6); localização encaminhada — co-decisão com FND-06 | FND-06 §18.2 obr. 15 |
| REC-007 | Atualização da tabela de RFC §14.4 | FND-04 §11.5, FND-06 §18.3, FND-07 §11.4, FND-08 §13.4 | aberta | owner da RFC — consolidar em passagem única | proposta de consolidação em FND-07 §11.4 |
| REC-008 | Reconciliação FND-10 × FND-09 (P9) | FND-10 §8 | quitada | verificação registrada no README (sem colisão de prefixo; sem obrigação nova) | FND-10 §8, P9 |
```

**Histórico** (append-only — uma entrada datada por mudança de estado):

```markdown
- 2026-08-22 — REC-001..REC-008: inventário inicial (SPEC-4W1BQK93).
- <data> — REC-NNN: <estado anterior> → <estado novo>; motivo e evidência.
```

Regras do ledger: a tabela de estado atual é a autoridade e pode ser reescrita;
o histórico nunca é reescrito; toda mudança na tabela exige entrada no
histórico; entradas citam artefato e seção, nunca replicam texto normativo;
pendências com tabela viva em artefato promovido (ex.: FND-10 §8, P1–P13) são
referenciadas como fonte, não duplicadas — o ledger cobre o que mudou de estado
**depois** da promoção do artefato que registrou a pendência.

### Errata de forma (RF3 — lista fechada, antes/depois exato)

| # | Arquivo:linha | Alvo | Antes | Depois |
|---|---------------|------|-------|--------|
| 1 | `upr-decision-mensagens.md:1015` | `ESC-04` | `` `normativo` — `ESC-04`: a rejeição **não** se expressa como sequência vazia de `` | `` `normativo` `ESC-04` — a rejeição **não** se expressa como sequência vazia de `` |
| 2 | `upr-decision-mensagens.md:1047` | `ESC-05` | `` `normativo` — `ESC-05`: a durabilidade do evento não o promove a contrato `` | `` `normativo` `ESC-05` — a durabilidade do evento não o promove a contrato `` |
| 3 | `upr-decision-mensagens.md:1054` | `ESC-06` | `` `normativo` — `ESC-06`: manter eventos antigos legíveis não converte o evento de `` | `` `normativo` `ESC-06` — manter eventos antigos legíveis não converte o evento de `` |
| 4 | `testes-interop.md:2160` | marcador órfão de §11.1 | `` `normativo` `` | (linha esvaziada — o marcador migra para a 5) |
| 5 | `testes-interop.md:2162` | abertura de `RAS-30` | `A calibração de que a cadeia se instancia **por regra, calibrada pelo modo** é a` | `` `normativo` — **`RAS-30`. A calibração de que a cadeia se instancia por regra, calibrada pelo modo é a `` |
| 6 | `testes-interop.md:2163` | fechamento de `RAS-30` | `chave de leitura desta seção e da §13. Tratar as 682 regras do acervo como iguais` | `chave de leitura desta seção e da §13.** Tratar as 682 regras do acervo como iguais` |
| 7 | `testes-interop.md:2170` | cabeçalho da tabela de modos | `` | `RAS-30` — Modo | O que a linha da cadeia instancia | `` | `` | Modo | O que a linha da cadeia instancia | `` |
| 8 | `testes-interop.md:2666` | linha do total de §13.1 | `**Total: 142 regras próprias.**` | ``**Total: 142 regras próprias: 141 `normativo` e uma `encaminhado`, `RAS-41`.**`` |

Propriedades da errata: oito edições em quatro definições, todas in-place (a
contagem de linhas dos dois arquivos não muda); o texto normativo é preservado —
muda o marcador e nada mais; cada edição resulta em exatamente **uma** definição
primária por ID (invariante C1).

A forma canônica **não exige negrito**: 54 das 429 definições em prosa do acervo
não o têm, 36 em FND-04 e 18 em FND-09, e preservam a ênfase interna do autor
(`uow-inbox-outbox.md:457`, `` `normativo` `UOW-01` — Uma UoW tem **exatamente
uma** transação local. ``). As edições 1 a 3 seguem esse precedente: movem o ID
para antes do travessão e nada mais. Impor negrito à frase-tese obrigaria a
editar também a linha seguinte de cada uma — porque a frase atravessa duas
linhas — e a apagar a ênfase que o autor pôs no meio dela, o que seria mudança
de redação além do marcador. As edições 5 e 6 são a exceção necessária: a forma
destacada de §11.1 **é** definida pelo negrito, e por isso `RAS-30` consome duas
linhas, perdendo a ênfase interna da 2162 — sem isso não haveria como fechar o
negrito, que markdown não aninha.
`ESC-04`, `ESC-05` e `ESC-06` não têm linha de tabela (as tabelas de §7 cobrem
`ESC-01..03` e `ESC-07..09`), então a prosa é a definição primária deles — a
errata a coloca na forma canônica de definição. Fundamento: RFC §14.2
(correção de redação via PR, sem incremento de versão).

### Contrato de extração (RF5 — o que torna C1–C6 decidíveis)

O verificador materializa este contrato; a spec o fixa para que as checagens
sejam reproduzíveis. Toda forma declarada abaixo foi medida no acervo por
extração determinística, e as contagens citadas são o estado de `f46ced2`.
Forma presente no acervo e ausente deste contrato é defeito do contrato, não
do acervo — foi assim que a forma destacada de `RAS-31`..`RAS-41` e o limite
de C3 entraram aqui.

- **Gramática de ID**: `PREFIXO-NN[s]` onde `PREFIXO ∈ [A-Z]{2,6}`,
  `NN ∈ \d{2,}` e `s ∈ [a-z]` opcional (`TRP-24b`, `KFK-01c`); mais as famílias
  compostas do FND-03 (`UPR-I\d{2}`, `UPR-L\d{2}`, `MSG-N\d{2}`) e o caso
  `ADO-00`. Fragmentos dentro de identificadores maiores não são IDs de regra
  (ex.: `COE-1` dentro de `V-COE-1`, que o FND-09 declara não ser ID).
- **Mapa prefixo → documento dono** (fechado): `DEC/CTR/FRT/ESC/UPR-I/UPR-L/MSG-N`
  → FND-03; `UOW/OBX/INB/GAR/BLK` → FND-04; `ENV/PTB/BUF/REP/INT` → FND-05;
  `TRP/RST/GRP/KFK/SQS/ASY/COE` → FND-06; `CTX/IDN/ERR/MAP/THR/DAT` → FND-07;
  `RES/MET/TRC/LOG/RUN` → FND-08; `PIR/CEN/FIX/ORA/KIT/FIT/RAS` → FND-09;
  `GOV/AUT/BOM/ADO/PIL/RDY` → FND-10. Prefixo fora do mapa em posição de
  definição é erro.
- **Mapa documento → arquivo** (fechado): `FND-01` → `inventario-as-is.md`;
  `FND-03` → `upr-decision-mensagens.md`; `FND-04` → `uow-inbox-outbox.md`;
  `FND-05` → `cloudevents-protobuf-buf.md`; `FND-06` →
  `politicas-transporte.md`; `FND-07` → `contexto-erros-seguranca.md`;
  `FND-08` → `resiliencia-observabilidade.md`; `FND-09` → `testes-interop.md`;
  `FND-10` → `governanca-bom-pilotos.md`; `RFC` →
  `rfc-dmpf-foundation-v0.1.md`. `FND-02` não existe no acervo e `FND-11` é
  futuro: ambos são ignorados, não são erro. Token de documento fora deste mapa
  — como `Parte-1`, que designa um rascunho de `plans/references/` (três
  ocorrências, todas na forma `Parte-1 §§5 e 6`) — não é citação verificável e
  também é ignorado.
- **Definição primária** (conta para C1 e C2a): linha de tabela no documento
  dono, com a assinatura que o perfil daquele documento declara; ou prosa
  `` `força` `ID` — `` no documento dono (429 ocorrências no acervo); ou a
  **forma destacada**, restrita ao escopo fechado abaixo. Um ID tem exatamente
  uma definição primária.
- **Forma destacada, fechada por escopo**: a sintaxe estrita
  `` `força` — **`ID`. `` é definição primária **somente** no documento FND-09,
  na seção §11.1, para os IDs `RAS-30`..`RAS-41`, com força `normativo` em
  `RAS-30`..`RAS-40` e `encaminhado` em `RAS-41`. Fora dessas quatro condições
  — outro documento, outra seção, ID fora da faixa, ou força diversa — a mesma
  sintaxe é **erro**, acusado por C5. O escopo é fechado por decisão, não por
  legado: `RAS-30` nasce nesta forma pela errata de RF3, e a forma não autoriza
  definições futuras em nenhum outro ponto do acervo.
- **Qualificador** (não conta como definição): prosa `` `força` — `ID` ... ``
  (68 ocorrências), inclusive a variante com dois-pontos
  `` `força` — `ID`: ... `` (3 ocorrências, todas em FND-03). O qualificador
  incide sobre regra já definida em outro lugar — é o caso de `ENV-25`,
  `INT-05` e `UPR-I04`. As três ocorrências com dois-pontos são `ESC-04`,
  `ESC-05` e `ESC-06`, que **não** têm definição em tabela: por isso, e apenas
  por isso, a errata de RF3 as converte para a forma canônica de definição.
- **Classes de força**: `normativo` (1139 ocorrências) e `encaminhado` (157)
  com ID são contadas separadamente — o FND-10 declara "77 + 1"; o FND-09,
  após a errata, "141 + 1". `recepcionado` (120) nunca conta: a força
  permanece da fonte. Os lexemas `registro` e `rationale` marcam contexto de
  parágrafo, não força de regra, e não participam de C1, C2a nem C5.
- **Perfil decidível por documento**: a união das três formas de definição,
  **filtrada pelo prefixo dono e deduplicada por ID**, fecha os oito totais
  exatamente — 54, 64, 64, 131, 112, 121, 142 e 78, somando 766. O filtro por
  prefixo é o que dispensa o intervalo de seções: as tabelas de rastreabilidade
  que inflam a contagem crua (`testes-interop.md` tem 628 linhas de tabela
  iniciadas por ID, contra 142 regras próprias; `cloudevents-protobuf-buf.md`,
  138 contra 64) citam IDs **herdados**, de outros documentos, e caem fora do
  filtro sozinhas.
  Fechar os oito totais, porém, **não** prova que o extrator está certo: um
  classificador que perdesse uma definição e contasse uma citação no lugar
  chegaria ao mesmo número. Por isso o perfil de cada documento declara o
  **conjunto exato de IDs com a sua localização** — é o conjunto, não a
  cardinalidade, que faz C2a provar alguma coisa. Declara também o heading e o
  intervalo de seções com definição, a assinatura exata da tabela, as formas de
  força admitidas, a força esperada por exceção nomeada e o seletor da
  declaração de total, para que uma mudança futura de forma seja detectada como
  divergência do perfil, e não absorvida em silêncio por um extrator permissivo.
- **Contextos excluídos**: tabelas de índice e de rastreabilidade, matrizes de
  obrigações herdadas e blocos de recepção citam IDs sem defini-los. A exclusão
  é feita pelo intervalo de seções do perfil, não por heurística de conteúdo.
- **Citações de seção** (C3) — **escopo declarado**: C3 valida **apenas** a
  citação cujo documento é adjacente ao símbolo de seção, nas formas
  `FND-XX §N`, `FND-XX §N.M`, `RFC §N` e `RFC §N.M` (1383 ocorrências no
  acervo, todas resolvendo hoje). Listas (`§N, §M`, `§N e §M` — 457 itens) e
  faixas (`§§N–M` — 15) herdam o documento da mesma expressão e são decompostas
  nos elementos; `§§N e M` é lista, não faixa. Os headings do FND-01 não usam o
  símbolo `§`, e `FND-01 §4.1` resolve para o heading `4.1`.
  **Fora do escopo de C3**: o símbolo de seção sem documento adjacente (3576
  ocorrências, 72% do total). Essa forma não é decidível — tratá-la como
  autorreferência ao documento corrente produz 413 falsos positivos, e herdar o
  último documento mencionado no parágrafo produz 446, com atribuição errada. O
  limite é declarado aqui para que o verificador não seja escrito contra uma
  leitura otimista do contrato e reprove o acervo corrigido.
- **Referências físicas** (C4), sintaxe fechada em quatro formas: (1)
  `arquivo.md:N` e `arquivo.md:N-M`; (2) a mesma com **caminho relativo à raiz
  do repositório**, como `docs/adr/README.md:7-15` (2 ocorrências); (3) a forma
  **abreviada** `SPEC-XXXXXXXX:N-M`, sem `.md` e com o nome truncado ao prefixo
  (2 ocorrências, ambas para `SPEC-DBTRMM3X`); (4) o **shorthand** `:N`, que
  herda o arquivo da **última referência completa anterior no fluxo do texto** —
  não da mesma linha física: 10 dos 33 shorthands do acervo herdam de linha
  anterior, e a leitura restrita a uma linha deixaria os dez órfãos. São **123**
  referências no total: 88 completas, 33 shorthands e 2 abreviadas.
  A resolução do alvo é **explícita por entrada do manifesto**: cada entrada
  registra o caminho resolvido a partir da raiz. Resolução por basename é
  proibida — `SPEC-DBTRMM3X-dmpf-adrs-minimos.md:93-101` só encontra alvo dessa
  maneira, e admitir a busca por nome tornaria o resultado dependente do
  conteúdo de diretórios que a referência não nomeia. Cada entrada registra
  também um trecho esperado da linha alvo; C4 falha se a linha não existir
  **ou se o conteúdo não contiver o trecho** — deslocamento semântico é
  detectado, não apenas estouro de EOF.
- **Faixas reservadas** (C6): `ORA-14..29`, `RAS-17..29`, `TRP-45`,
  `GOV-27..29` — definição nova em qualquer delas é erro.
- **C2a e C2b**: C2a confere o total que cada índice declara contra as
  definições extraídas sob este contrato, artefato por artefato. C2b confere as
  projeções do README contra a métrica-fonte que cada correção de RF1 cita. São
  checagens distintas porque falham por motivos distintos: C2a acusa extrator ou
  perfil errados; C2b acusa índice desatualizado. O total geral de 766 é soma
  derivada dos oito totais e não é declarado por artefato algum — o relatório o
  registra como soma, nunca como valor conferido contra uma fonte.
- **Artefatos de dados normativos**: três manifestos, todos sob
  `tools/tests/dmpf-verify/` e revisáveis no PR — `profiles.json` (o perfil por
  documento), `counts.json` (artefato → local da declaração de total → valor
  esperado, exigido porque os totais convivem com totais de regras herdadas que
  um seletor genérico contaria) e `line-refs.json` (as 123 referências físicas,
  com caminho resolvido e trecho esperado).

### Fluxo principal

1. Aplicar RF3 (errata) e RF1/RF4 (README).
2. Criar RF2 (ledger) com o inventário inicial `REC-001..REC-008`.
3. Criar RF5 (verificador + fixtures + manifesto de referências físicas) e
   validar: exit 0 no acervo corrigido; reverse-lookup confirma que nenhuma
   linha citada se deslocou.
4. Criar RF6 (workflow) e validar o disparo por path.

## Decisões técnicas

- **Ledger com estado atual mutável + histórico append-only** em vez de tabela
  única append-only: uma pendência que muda de estado precisa de uma única
  linha vigente (autoridade) sem perder a trilha. Alternativa descartada:
  tabela única append-only — sem regra de resolução, duas linhas para a mesma
  pendência deixam o leitor e o verificador sem saber qual vale.
- **Ledger em arquivo próprio** (`docs/dmpf/reconciliacao.md`) em vez de seção
  do README: o README é índice e carta de leitura; o ledger é estado
  operacional com ritmo de mudança diferente. Alternativa descartada: seção no
  README — misturaria gêneros e aceleraria o envelhecimento que esta spec
  corrige.
- **Errata fechada com antes/depois por linha** em vez de "normalizar formatos
  desviantes" em aberto: a revisão externa demonstrou que dois candidatos
  (`ENV-25`, `INT-05`) eram qualificadores legítimos, não desvios — critério
  aberto convidaria a reclassificação indevida. O fundamento normativo é RFC
  §14.2; M1–M4 governam adições por âncora e não classificam edição
  retroativa, por isso a errata não se apoia neles.
- **C4 com trecho esperado (manifesto)** em vez de só "linha dentro do
  arquivo": inserir uma linha antes do alvo mantém o número válido apontando
  para conteúdo errado — o modo de falha real é deslocamento semântico, e só a
  comparação de conteúdo o detecta.
- **Workflow de CI dedicado** em vez de estender o `ci.yml`: o `ci.yml` ignora
  `**/*.md` por design; remover a exclusão custaria pipeline inteiro para todo
  PR documental. O workflow novo dispara por `docs/dmpf/**`, pelo script e por
  si mesmo, e também pelo harness — e executa os dois, porque um gate que roda
  só o verificador deixa o harness sem cobertura alguma. Node 24 já vem da
  action local (ADR-005 documenta apenas `jq` e `curl` no runner).
- **Referências por linha existentes permanecem**: migrá-las para âncoras
  tocaria massivamente artefatos promovidos, com risco desproporcional; C4 as
  vigia (com detecção de deslocamento) e a convenção de RF4 estanca o
  crescimento. Alternativa descartada: migração em massa — reabriria o risco
  P0 de deslocamento que esta spec se compromete a evitar.

## Regras relacionadas

- `docs/specs/README.md` — schema de frontmatter e convenções deste catálogo.
- [SPEC-4CKAD0BC](./SPEC-4CKAD0BC-rastreabilidade-fontes-dmpf.md) — fronteira:
  aquela spec corrige referências de docs versionados a `plans/` (não
  versionado); esta corrige concordância **entre** documentos versionados de
  `docs/dmpf/`. Sem dependência normativa, mas com **risco de sequenciamento**:
  ambas editam o README do DMPF e artefatos do acervo — coordenar a ordem de
  entrega para evitar conflito de merge e para que o manifesto de C4 seja
  gerado depois da que entrar por último.
- [SPEC-DBTRMM3X](./SPEC-DBTRMM3X-dmpf-adrs-minimos.md) — a contradição 15
  slots × 19 acionamentos de ADR permanece com o FND-11 (ARQ-448); o ledger a
  referencia sem tratá-la.
- [SPEC-QG2N8STY](./SPEC-QG2N8STY-dmpf-foundation.md) — spec umbrella do épico
  ARQ-436; esta spec é manutenção da fundação que o épico produziu.
- `.claude/rules/ephemeral-refs.md` — princípio de citação durável que
  fundamenta a convenção de RF4.

## Verificação e testes

### Critérios de aceite

- [ ] `docs/dmpf/README.md` com as 8 correções da tabela aplicadas: nenhuma das
      7 afirmações desatualizadas permanece, os números conferem com os
      artefatos promovidos (fonte da verdade citada por correção), e a tabela
      `## Artefatos` passa de dez para onze linhas, com o ledger indexado.
- [ ] `docs/dmpf/reconciliacao.md` criado com o inventário fechado
      `REC-001..REC-008` (estado atual + histórico), cada linha com os 6
      campos preenchidos.
- [ ] As 8 edições da errata aplicadas exatamente como especificadas
      (antes/depois por linha); a contagem de linhas de
      `upr-decision-mensagens.md` e `testes-interop.md` não muda; nenhum outro
      trecho dos dois arquivos é alterado.
- [ ] Reverse-lookup executado no PR: nenhuma referência física do acervo tem
      alvo deslocado após a errata (C4 verde com o manifesto gerado).
- [ ] Convenção de citação publicada no README de `docs/dmpf/`.
- [ ] `node tools/dmpf-verify.mjs` sai com código 0 no acervo corrigido, e
      `node --test tools/tests/dmpf-verify/` passa.
- [ ] Suíte de fixtures cobre cada checagem com ao menos 1 vetor positivo e 1
      negativo: C1 (definição duplicada; definição em doc não-dono), C2
      (contagem declarada ≠ real, incluindo sufixo de letra e `encaminhado`),
      C3 (seção inexistente; forma `§N` e `§N.M`), C4 (linha além do EOF;
      linha existente com conteúdo deslocado), C5 (definição fora da
      gramática; qualificador tratado como definição), C6 (definição em faixa
      reservada).
- [ ] Teste histórico executável: o script atual, com
      `--root <worktree do commit base do PR>`, acusa as violações conhecidas
      (números do README, formas da errata) — demonstrando que o verificador
      detectaria o estado anterior.
- [ ] Workflow `dmpf-verify.yml` executa em PR que altera apenas
      `docs/dmpf/*.md` e também em PR que altera apenas `tools/dmpf-verify.mjs`
      ou o próprio workflow.
- [ ] Diff do PR não contém alteração fora da lista fechada (errata + README +
      arquivos novos) — revisão humana + RNF1.

### Cenários de teste (mínimo 3)

```text
DADO o acervo docs/dmpf/ com RF1-RF3 aplicados e o manifesto de C4 gerado
QUANDO node tools/dmpf-verify.mjs executa
ENTÃO exit code 0 e relatório com as 6 checagens em "ok"

DADO uma fixture com a citação `FND-04 §99.9` (seção inexistente)
QUANDO o verificador executa com --root apontando para a fixture
ENTÃO exit code ≠ 0 e C3 aponta arquivo, linha e alvo não resolvido

DADO uma fixture em que a linha alvo de `arquivo.md:N` existe mas o conteúdo
     não contém o trecho registrado no manifesto (deslocamento semântico)
QUANDO o verificador executa
ENTÃO exit code ≠ 0 e C4 distingue "linha inexistente" de "conteúdo deslocado"

DADO uma fixture com `` `normativo` `XYZ-01` — `` num documento que não é dono
     do prefixo XYZ
QUANDO o verificador executa
ENTÃO exit code ≠ 0 e C1 aponta definição fora do documento dono

DADO o worktree do commit base do PR de implementação
QUANDO o script atual executa com --root nesse worktree
ENTÃO exit code ≠ 0 acusando ao menos as divergências de contagem do README
     (C2b) e as formas pré-errata (C5)

DADO um PR alterando somente tools/dmpf-verify.mjs
QUANDO o CI do repositório roda
ENTÃO dmpf-verify.yml executa (gate não é cego a mudanças no próprio verificador)
```

<critical_constraints>
- [P0] Nenhuma mudança de semântica normativa nos artefatos promovidos — errata
  fechada (8 edições in-place, antes/depois por linha), fundamentada em RFC
  §14.2, com diff auditável.
- [P0] A RFC não é editada; mudanças nela exigem rito de versão (fora de
  escopo).
- [P0] Nenhuma inserção/remoção de linha em artefato promovido; o PR reconfirma
  por reverse-lookup que nenhuma linha citada se desloca.
- [P0] `plans/` permanece fora do versionamento; referências a `plans/` são da
  SPEC-4CKAD0BC.
- [P1] Ledger: estado atual mutável + histórico append-only; aponta, nunca
  copia texto normativo.
- [P1] Verificador sobre a biblioteca padrão do Node, com `--root` injetável e
  fixtures herméticas.
</critical_constraints>

## Escopo fora

- **Migração das referências `arquivo:linha` para âncoras semânticas**:
  vigiadas por C4 (com detecção de deslocamento); migrar exigiria editar
  massivamente artefatos promovidos — só se justifica se C4 passar a quebrar
  com frequência.
- **Referências a `plans/` em docs versionados**: SPEC-4CKAD0BC (ARQ-490).
- **Reconciliação dos 19 acionamentos de ADR nos 15 slots**: FND-11
  (ARQ-448, SPEC-DBTRMM3X), conforme FND-10 §8.2.
- **Ampliação do escopo da ANC-09 (P13) e rito de versão 0.2 da RFC (G5)**:
  owner da RFC; o ledger registra o estado, não resolve.
- **Qualquer correção de conteúdo normativo** dos artefatos promovidos: se a
  revisão do PR identificar necessidade de mudança semântica, ela sai desta
  spec e entra pelo rito de âncora/versão da RFC.
