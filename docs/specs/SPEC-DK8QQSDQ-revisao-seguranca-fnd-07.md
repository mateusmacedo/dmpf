---
id: SPEC-DK8QQSDQ
slug: revisao-seguranca-fnd-07
title: DMPF FND-07 — Revisão de Segurança do threat model e da governança do dado
stage: done
priority: P0
depends_on: [SPEC-XQWGGAXF]
ticket_url: null
subtask_urls: []
created: 2026-08-30
---
# SPEC-DK8QQSDQ: DMPF FND-07 — Revisão de Segurança do threat model e da governança do dado

## Resumo

O artefato `docs/dmpf/contexto-erros-seguranca.md` foi mergeado em `develop` pelo
PR #10, e a regra `THR-03` dele declara, com todas as letras, que o threat model
**não está satisfeito** enquanto não for revisado por Segurança. A story do FND-07
(ARQ-444) foi fechada; este gate
não. Esta spec executa a revisão de §7 e §8, registra o parecer como evidência
versionada e nomeia o titular do papel — fechando o último gate aberto do épico
ARQ-436.

Como responsável pela plataforma, quero o threat model do FND-07 revisado e o
parecer registrado, para que o épico da fundação possa ser concluído e os épicos
subsequentes deixem de herdar um gate aberto.

## Contexto

- **Problema**: `THR-03` é o único gate que impede o fechamento do FND-07 e, por
  cadeia, mantém o ARQ-436 em *Em Desenvolvimento*. A `SPEC-XQWGGAXF` está com
  `stage: done` e **dois dos três critérios de aceite desmarcados** — o CA1, já
  cumprido de fato pelo PR #10, e o CA2, que é este gate. O primeiro é
  inconsistência de registro; o segundo é pendência real.
- **Impacto**: com o gate fechado, a `SPEC-YRJRADY9` (Kernel e SDK Go,
  ARQ-519) deixa de estar
  bloqueada, e a cadeia de rastreabilidade do acervo passa a declarar o estado
  verdadeiro em vez de uma pendência que sobreviveu à entrega que a criou.
- **Causa da pendência**: o cabeçalho do artefato declara o **papel** do revisor —
  «um representante de Segurança para §7 e §8, obrigatório» — e não a pessoa. Sem
  nome atribuído, o gate nunca foi acionável. A descrição da ARQ-488 é explícita:
  «o assignee atual responde pelo encaminhamento, não pela revisão».
- **Ausência de área constituída**: a organização não tem área de Segurança
  formalizada no repositório. O `.github/CODEOWNERS` permanece com o placeholder
  `@owner` do template, e nenhum documento do acervo nomeia titular. O G2 dos
  cinco gates externos do FND-10 cita «Arquitetura, com Segurança como revisora»,
  também sem pessoa.
- **Umbrella**: [SPEC-QG2N8STY](./SPEC-QG2N8STY-dmpf-foundation.md)
- **Issue**: ARQ-488
- **Artefato revisado**: `docs/dmpf/contexto-erros-seguranca.md` (1878 linhas,
  `draft normativo`), sob a âncora ANC-05 da RFC v0.1
- **Spec de origem**: [SPEC-XQWGGAXF](./SPEC-XQWGGAXF-dmpf-contexto-erros-seguranca.md)
- **Precedente de forma**: [SPEC-4CKAD0BC](./SPEC-4CKAD0BC-rastreabilidade-fontes-dmpf.md)
  — spec documental tardia do mesmo épico

### Por que não existe atalho

Duas regras do próprio acervo fecham as saídas fáceis, e a spec é desenhada
dentro delas:

| Regra | Fonte | Efeito sobre esta entrega |
|-------|-------|---------------------------|
| `GOV-05` | FND-10 §1.5 | «Marcar critério de aceite como atendido por antecipação é defeito de rastreabilidade.» O CA2 só fecha **depois** do parecer registrado, nunca junto com ele por antecipação |
| `GOV-31` + `GOV-32` N1 | FND-10 §5.1 | O escape hatch tem universo positivo fechado em E1, E2 e E3, e N1 veda exceção sobre constraint P0. `THR-03` não é nenhum dos três objetos excepcionáveis: **não há via de dispensa da revisão** |

<constraints>
- [P0] NUNCA marcar o CA2 da SPEC-XQWGGAXF antes do parecer estar registrado — `GOV-05` trata antecipação como defeito de rastreabilidade
- [P0] NUNCA invocar o escape hatch para dispensar `THR-03` — `GOV-31` fecha o universo em E1/E2/E3 e `GOV-32` N1 veda exceção sobre P0
- [P0] Nenhuma regra normativa de `contexto-erros-seguranca.md` muda de texto ou de numeração — a revisão CONFERE, não reescreve
- [P0] A divergência sobre quem exerce o papel de revisor é decisão REGISTRADA em ADR, com alternativa descartada e condição de re-revisão — nunca silêncio
- [P0] O desfecho da revisão é resultado, não premissa: o parecer pode concluir aprovação, aprovação com ressalvas ou reprovação com achados
- [P1] O parecer é evidência de varredura, não norma nova — nenhum identificador `THR-*` ou `DAT-*` é criado, alterado ou renumerado
- [P0] NENHUM artefato promovido é editado retroativamente — o estado vigente vai para o ledger `reconciliacao.md`, conforme `navegacao.md` e o precedente `REC-009`
</constraints>

## Requisitos

### Funcionais

- [ ] **[P0] Titular nomeado e divergência registrada**: `docs/adr/029-*.md`
  registra que o papel de «representante de Segurança para §7 e §8» é exercido por
  Mateus Macedo Dos Anjos, na ausência de área de Segurança constituída na
  organização. O ADR traz os quatro itens que `GOV-30` exige de uma decisão
  registrada — decisão com alternativa descartada, owner, justificativa técnica e
  condição de retorno —, ainda que não seja um pedido de escape hatch.
  - A divergência a declarar é nominal: o cabeçalho do artefato e a ARQ-488 separam
    quem encaminha de quem revisa, e aqui a separação não existe
  - A condição de re-revisão é explícita: constituída a área de Segurança, §7 e §8
    voltam à revisão por titular independente

- [ ] **[P0] Varredura STRIDE conferida nos sete vetores**: para cada vetor de §7.2
  a §7.8, o parecer confere que as seis categorias — *Spoofing*, *Tampering*,
  *Repudiation*, *Information disclosure*, *Denial of service* e *Elevation of
  privilege* — foram avaliadas, seja em linha de ameaça material, seja na linha de
  exclusões com motivo.
  - Universo conferido: **33 linhas de ameaça material** e **7 linhas de
    exclusões**, uma por vetor, distribuídas como V1=5, V2=4, V3=3, V4=6, V5=4,
    V6=5 e V7=6
  - Categoria que não apareça em nenhuma das duas posições de um vetor é achado
    reportado com vetor, categoria e natureza

- [ ] **[P0] Exclusões conferidas uma a uma**: cada uma das 7 linhas de exclusões é
  conferida quanto a três coisas — a categoria está nomeada, o motivo está
  declarado, e o motivo sustenta a ausência.
  - `§7.1` exige a linha de exclusões porque «categoria ausente sem justificativa é
    indistinguível de categoria não avaliada»; conferir a existência da linha não
    basta, o motivo tem de sustentar
  - Exclusão que apenas remeta a outro vetor é conferida contra o vetor de destino:
    a categoria tem de aparecer lá

- [ ] **[P0] Owner conferido linha a linha**: `§7.1` normatiza «nenhuma linha sem
  owner». O parecer confere as 40 linhas — 33 materiais e 7 de exclusões — quanto à
  presença e à qualidade do owner.
  - Owner declarado como regra deste artefato pela ID, artefato irmão pela âncora,
    ou plataforma, conforme `§7.1` admite
  - Owner que aponte para artefato irmão é conferido quanto à existência da âncora
    citada e ao estado do artefato de destino

- [ ] **[P0] §8 revisada nos quatro eixos e na matriz**: classificação e critério de
  sensibilidade (§8.1, `DAT-01`..`DAT-04`), cifra em repouso (§8.3,
  `DAT-08`..`DAT-10`), acesso ao dado armazenado (§8.4, `DAT-11`..`DAT-13`) e teto
  de retenção (§8.5, `DAT-14`..`DAT-18`), mais a matriz do baseline de Parte-1 §14
  (§8.8) quanto ao estado declarado por cláusula e ao dono do que não é consolidado.
  - As **treze** cláusulas da matriz são conferidas por texto, não por contagem —
    `DAT-26` e o `rationale` de §8.8 fixam esse critério
  - Os quatro pontos que a ARQ-488 nomeia — `CTX-25`, `IDN-14`, `DAT-10` e
    `THR-01` — recebem tratamento nominal no parecer

- [ ] **[P0] Parecer registrado como artefato versionado**:
  `docs/dmpf/revisao-seguranca-fnd-07.md`, com cabeçalho no padrão dos demais
  artefatos de `docs/dmpf/`, a matriz de conferência por vetor, a conferência de §8,
  os achados e o desfecho declarado.
  - O desfecho é um de três: **aprovado**, **aprovado com ressalvas** (achados que
    não impedem o fechamento, com destino nomeado) ou **reprovado** (achado que
    invalida a varredura)
  - Achado, quando houver, tem vetor ou subseção, natureza, severidade e destino

- [ ] **[P0] Critérios de aceite da SPEC-XQWGGAXF reconciliados**: o CA1 é marcado
  por já estar cumprido — o artefato foi promovido e aprovado no PR #10 —, e o CA2
  é marcado somente se o desfecho do parecer for aprovação ou aprovação com
  ressalvas.
  - Reprovação mantém o CA2 desmarcado e converte os achados em pendência com
    destino, sem fechar a ARQ-488

- [ ] **[P0] Estado registrado no ledger de reconciliação, não no artefato**:
  `docs/dmpf/reconciliacao.md` recebe a entrada `REC-011` na tabela de estado
  atual e a entrada datada correspondente no histórico append-only.
  - `navegacao.md` fixa a convenção: «pendência registrada num artefato promovido
    não pode ser riscada nele — o artefato não é editado retroativamente. Quem
    carrega o estado vigente é o ledger de reconciliação»
  - O estado é `parcial`, com a partição declarada na célula: owner nomeado e
    revisão executada ficam quitados; a re-revisão por titular independente segue
    encaminhada
  - `REC-009` é o precedente direto — três irmãos com transcrição envelhecida
    foram preservados, e o ledger carregou o estado

- [ ] **[P0] Nenhum artefato promovido é editado**: `contexto-erros-seguranca.md`,
  `testes-interop.md` e `resiliencia-observabilidade.md` permanecem exatamente como
  foram promovidos. O FND-07 continua registrando a pendência onde ela nasceu, e o
  FND-09 §14 continua listando a pendência 10 que a referencia.

- [ ] **[P1] Registros vivos atualizados**: o `README.md` do acervo — que é índice,
  não artefato promovido — aponta `REC-011` como estado vigente da pendência e
  lista o parecer na tabela de artefatos.

### Não-funcionais

- [ ] **[P0] Nenhuma alteração de conteúdo normativo**: as 112 regras com ID estável
  do FND-07 — `CTX` 28, `IDN` 20, `ERR` 28, `MAP` 7, `THR` 3 e `DAT` 26 — permanecem
  idênticas em texto e em numeração. A conferência é verificável pela contagem, que
  o próprio `testes-interop.md:3523` declara.
- [ ] **[P0] Rastreabilidade auditável**: cada linha conferida no parecer aponta o
  vetor ou a subseção de origem, e o parecer é conferível contra o artefato sem
  consultar esta spec.
- [ ] **[P1] Diff legível**: a propagação altera as linhas que declaram estado, sem
  reformatar parágrafo adjacente que a esconda.

## Camadas afetadas

Esta spec toca documentação e governança; não há camada de código envolvida.

| Camada | Afeta | Observação |
|--------|-------|------------|
| Domain / Application / Infra | [ ] | Nenhuma |
| Documentação normativa (`docs/dmpf/`) | [ ] | **Nenhuma.** Artefato promovido não é editado retroativamente |
| Registros vivos (`docs/dmpf/`) | [x] | `reconciliacao.md` (`REC-011`), `README.md` (índice) e o parecer, novo — nenhum é artefato promovido |
| Decisões (`docs/adr/`) | [x] | ADR-029, novo — a decisão sobre o titular do papel |
| Specs (`docs/specs/`) | [x] | `SPEC-XQWGGAXF` nos critérios de aceite; esta spec |
| Tracker (Jira) | [x] | ARQ-488 e, por cadeia, ARQ-436 |
| CI / workflows | [ ] | Nenhuma — `dmpf-verify.yml` afere congruência do acervo e não conhece o gate |

## Localização de código

| Fase | Caminho |
|------|---------|
| Objeto revisado | `docs/dmpf/contexto-erros-seguranca.md` §7 (linhas 1250–1405) e §8 (linhas 1406–1608) |
| Parecer | `docs/dmpf/revisao-seguranca-fnd-07.md` — a criar |
| Decisão | `docs/adr/029-titular-da-revisao-de-seguranca-fnd-07.md` — a criar |
| Reconciliação | `docs/specs/SPEC-XQWGGAXF-dmpf-contexto-erros-seguranca.md` §Critérios de aceite |
| Estado da pendência | `docs/dmpf/reconciliacao.md` — `REC-011` na tabela de estado atual e no histórico |
| Registros vivos | `docs/dmpf/README.md` — resumo das pendências e tabela de artefatos |

**Arquivos a modificar**:

| Arquivo | O que muda |
|---------|------------|
| `docs/dmpf/reconciliacao.md` | `REC-011` na tabela de estado atual (`parcial`) e entrada datada no histórico append-only |
| `docs/dmpf/README.md` | Resumo das pendências do FND-07 aponta `REC-011`; parecer entra na tabela de artefatos |
| `docs/adr/README.md` | ADR-029 no índice, após ADR-028; a tabela `ADR-DMPF-A..S` fica intocada |
| `docs/specs/SPEC-XQWGGAXF-*.md` | CA1 e CA2 marcados conforme o desfecho |

**Arquivos que NÃO mudam — todos os artefatos promovidos.** `navegacao.md` fixa a
convenção: «pendência registrada num artefato promovido não pode ser riscada nele
— o artefato não é editado retroativamente. Quem carrega o estado vigente é o
ledger de reconciliação». O verificador `tools/dmpf-verify.mjs` a reforça
mecanicamente: editar o FND-07 desloca linhas fisicamente referenciadas por
irmãos e quebra os manifestos versionados de `tools/tests/dmpf-verify/`. Ficam
intactos `contexto-erros-seguranca.md`, `testes-interop.md` e
`resiliencia-observabilidade.md`.

## Design

### Arquitetura da entrega

```
ADR-029  ──── decide QUEM exerce o papel, e sob qual divergência declarada
   │
   └──▶ revisao-seguranca-fnd-07.md  ──── registra O QUE foi conferido, e o desfecho
              │
              ├──▶ contexto-erros-seguranca.md  §7.10, §11.4, cabeçalho
              ├──▶ testes-interop.md            §14 e as três ocorrências da matriz
              ├──▶ README.md                    resumo das pendências
              └──▶ SPEC-XQWGGAXF                CA1 e CA2
```

A separação entre ADR e parecer não é estilística. A decisão sobre o titular tem
vida longa e é revisitada quando a organização constituir a área; a evidência da
varredura é datada e vale para a versão conferida do artefato. Fundi-las obrigaria
a reabrir a decisão a cada re-revisão.

### Fluxo principal

1. **Nomear** — o ADR-029 registra o titular, a divergência em relação ao cabeçalho
   do artefato e à ARQ-488, a alternativa descartada e a condição de re-revisão
2. **Conferir §7** — os 7 vetores, um a um: as 6 categorias por vetor, a linha de
   exclusões com motivo que sustente, e o owner de cada uma das 40 linhas
3. **Conferir §7.9 e §7.10** — a fronteira do eixo *Denial of service* encaminhada
   ao FND-08 sob ANC-06 está no lugar certo, e o gate está corretamente formulado
4. **Conferir §8** — os quatro eixos e a matriz de treze cláusulas do baseline
5. **Registrar** — o parecer com a matriz de conferência, os achados e o desfecho
6. **Propagar** — o estado do gate nos pontos que o declaram aberto
7. **Reconciliar** — CA1 e CA2 da `SPEC-XQWGGAXF`, conforme o desfecho
8. **Fechar** — ARQ-488, e o efeito em cadeia sobre ARQ-436 e `SPEC-YRJRADY9`

### Critério de conferência por vetor

Para cada vetor, a conferência produz uma linha da matriz:

```
para cada vetor V em {adapters, desserializacao, contratos, mensageria,
                      secrets, pii, permissoes}:
    materiais   = categorias com linha de ameaca propria em V
    excluidas   = categorias nomeadas na linha de exclusoes de V
    cobertura   = materiais UNIAO excluidas
    se cobertura != as 6 categorias STRIDE:
        achado(V, categorias ausentes, "categoria nao avaliada")
    para cada linha L em V:
        se owner(L) vazio:
            achado(V, L, "linha sem owner — viola §7.1")
        se owner(L) aponta artefato irmao e a ancora citada nao existe:
            achado(V, L, "owner com ancora inexistente")
    para cada categoria C em excluidas:
        se motivo(C) ausente ou nao sustenta a ausencia:
            achado(V, C, "exclusao sem justificativa suficiente")
```

## Decisões técnicas

- **Titular do papel: o próprio assignee, com divergência declarada em ADR**. A
  organização não tem área de Segurança constituída, e as duas alternativas foram
  descartadas: deixar o gate aberto indefinidamente perpetua a pendência que já
  sobreviveu à entrega que a criou, e invocar o escape hatch é vedado por
  `GOV-32` N1. A divergência é real — o cabeçalho do artefato e a ARQ-488 separam
  quem encaminha de quem revisa — e por isso é registrada, não contornada.
- **Parecer separado do artefato revisado**, e não uma seção nova dentro dele. O
  FND-07 já tem 1878 linhas, e misturar norma com evidência de revisão confunde
  dois sujeitos com ciclos de vida distintos: a norma vale até ser sucedida, o
  parecer vale para a versão conferida.
- **Desfecho como resultado, não como premissa**. A spec especifica o processo e o
  registro, e admite os três desfechos. Uma spec que garantisse a aprovação de
  antemão tornaria a revisão uma encenação, e o `rationale` de §7.9 nomeia
  exatamente esse risco ao explicar por que a avaliação de *Denial of service* foi
  registrada mesmo sem gerar norma.
- **Conferência mecânica antes da conferência de mérito**. A presença de categoria,
  de motivo e de owner é verificável por contagem e reprodutível; o mérito da
  mitigação não é. Separar as duas deixa explícito o que qualquer pessoa pode
  reconferir e o que depende do julgamento do revisor.
  Alternativa descartada: parecer em prosa corrida, que não permite reconferência.
- **Contagem como instrumento de verificação**. As 112 regras e as 40 linhas do §7
  são reproduzíveis por varredura do arquivo, e o total de 112 já é declarado por
  `testes-interop.md:3523` — o que dá um segundo ponto de aferição independente.

## Regras relacionadas

- `GOV-05` — gate externo não é satisfeito por declaração nem por ausência de menção
- `GOV-30`, `GOV-31`, `GOV-32` — escape hatch: requisitos, universo positivo e negações
- `THR-01`, `THR-02`, `THR-03` — o eixo *Denial of service* e o gate da revisão
- `DAT-01` a `DAT-26` — governança do dado de negócio
- `SEC-1` a `SEC-7` — as sete obrigações herdadas que §8 quita
- [SPEC-XQWGGAXF](./SPEC-XQWGGAXF-dmpf-contexto-erros-seguranca.md) — a entrega que criou o gate
- [SPEC-YRJRADY9](./SPEC-YRJRADY9-dmpf-kernel-sdk-go.md) — bloqueada enquanto o épico não fecha
- `.claude/rules/ephemeral-refs.md` — citação de artefato durável

## Verificação e testes

### Critérios de aceite

- [x] **[P0] ADR-029 criado**, nomeando o titular, declarando a divergência em
  relação ao cabeçalho do FND-07 e à ARQ-488, com alternativa descartada e condição
  de re-revisão
- [x] **[P0] Parecer em `docs/dmpf/revisao-seguranca-fnd-07.md`** com matriz de
  conferência dos 7 vetores, conferência de §8 nos 4 eixos e na matriz de 13
  cláusulas, achados e desfecho declarado
- [x] **[P0] As 40 linhas de §7 conferidas** — 33 materiais e 7 de exclusões — com
  a distribuição por vetor registrada e reproduzível
- [x] **[P0] As 6 categorias STRIDE conferidas em cada um dos 7 vetores**, com a
  posição de cada uma (linha própria ou exclusão) registrada
- [x] **[P0] Os quatro pontos nominados pela ARQ-488** — `CTX-25`, `IDN-14`,
  `DAT-10` e `THR-01` — tratados nominalmente no parecer
- [x] **[P0] As 112 regras do FND-07 inalteradas** em texto e numeração, conferido
  por contagem por prefixo: `CTX` 28, `IDN` 20, `ERR` 28, `MAP` 7, `THR` 3, `DAT` 26
- [x] **[P0] CA1 da SPEC-XQWGGAXF marcado** e **CA2 marcado somente se o desfecho
  for aprovação ou aprovação com ressalvas**
- [x] **[P0] `REC-011` registrado** no ledger — linha `parcial` na tabela de estado
  atual, com a partição declarada, e entrada datada no histórico append-only
- [ ] **[P0] Os três artefatos promovidos byte-idênticos** ao estado anterior:
  `contexto-erros-seguranca.md`, `testes-interop.md`, `resiliencia-observabilidade.md`
- [ ] **[P0] `node tools/dmpf-verify.mjs` sem violações** — o gate mecânico que
  reforça a convenção de não editar artefato promovido
- [ ] **[P1] ARQ-488 transicionada**, com o parecer citado
- [x] **[P0] Revisão PT-BR** do parecer e do ADR sem erro de acentuação

### Cenários de teste (mínimo 3)

```
DADO o artefato contexto-erros-seguranca.md na versão conferida
QUANDO a varredura percorre os sete vetores de §7.2 a §7.8
ENTÃO cada uma das seis categorias STRIDE aparece, em cada vetor, ou como linha
     de ameaça material ou como categoria nomeada na linha de exclusões — e a
     matriz do parecer registra em qual das duas posições

DADO um vetor cuja linha de exclusões nomeie uma categoria remetendo a outro vetor
QUANDO a conferência segue a remissão até o vetor de destino
ENTÃO a categoria aparece lá como ameaça material; se não aparecer, o parecer
     registra achado "exclusão sem destino verificável" com vetor de origem,
     vetor de destino e categoria

DADO que a conferência encontre uma linha de ameaça sem owner
QUANDO o parecer é redigido
ENTÃO o desfecho NÃO é aprovação: a linha viola §7.1 «nenhuma linha sem owner»,
     o achado é registrado com severidade e destino, e o CA2 da SPEC-XQWGGAXF
     permanece desmarcado até o achado ser tratado

DADO o parecer concluído com desfecho de aprovação
QUANDO o CA2 da SPEC-XQWGGAXF é marcado
ENTÃO a marcação ocorre no mesmo commit que registra o parecer ou depois dele,
     nunca antes — sob GOV-05, marcar por antecipação é defeito de rastreabilidade

DADO o gate satisfeito e o parecer registrado
QUANDO o estado da pendência precisa ser anotado no acervo
ENTÃO ele entra como REC-011 no ledger de reconciliacao.md, e NENHUM artefato
     promovido é editado — nem o FND-07 onde a pendência nasceu, nem o FND-09
     que a referencia; a convenção está em navegacao.md e o precedente em REC-009

DADO a tentação de riscar a pendência no proprio FND-07
QUANDO a edição desloca linhas fisicamente referenciadas por artefatos irmãos
ENTÃO node tools/dmpf-verify.mjs acusa violação nos manifestos versionados de
     tools/tests/dmpf-verify/, e o emissor de line-refs recusa regenerar, porque
     emitir sobre alvo divergente abençoaria o deslocamento em vez de acusá-lo
```

<critical_constraints>
- [P0] NUNCA marcar o CA2 da SPEC-XQWGGAXF antes do parecer estar registrado — `GOV-05` trata antecipação como defeito de rastreabilidade
- [P0] NUNCA invocar o escape hatch para dispensar `THR-03` — `GOV-31` fecha o universo em E1/E2/E3 e `GOV-32` N1 veda exceção sobre P0
- [P0] Nenhuma regra normativa de `contexto-erros-seguranca.md` muda de texto ou de numeração — a revisão CONFERE, não reescreve
- [P0] A divergência sobre quem exerce o papel de revisor é decisão REGISTRADA em ADR, com alternativa descartada e condição de re-revisão — nunca silêncio
- [P0] O desfecho da revisão é resultado, não premissa: o parecer pode concluir aprovação, aprovação com ressalvas ou reprovação com achados
- [P1] O parecer é evidência de varredura, não norma nova — nenhum identificador `THR-*` ou `DAT-*` é criado, alterado ou renumerado
- [P0] NENHUM artefato promovido é editado retroativamente — o estado vigente vai para o ledger `reconciliacao.md`, conforme `navegacao.md` e o precedente `REC-009`
</critical_constraints>

## Escopo fora

- **Mérito das regras `CTX`, `IDN`, `ERR` e `MAP`**: a revisão de `THR-03` incide
  sobre §7 e §8, conforme o texto da regra e o cabeçalho do artefato. As demais
  seções foram revisadas por Plataforma e Arquitetura no PR #10 e não são reabertas.
- **Pendência 2 do FND-08** — validação do runbook por SRE, também sem owner
  nomeado. Usa o mesmo tratamento de `THR-03` como precedente
  (`resiliencia-observabilidade.md:1967`), mas é gate próprio, de outra story
  (ARQ-445).
- **Pendências 1, 2, 3 e 5 de §11.4 do FND-07**: RFC §14.4, a consolidação das
  quatro pendências acumuladas, a forma de persistir os três atributos e a
  divergência sobre ADR-012/ADR-013. Nenhuma é gate de `THR-03`, e cada uma tem
  destino próprio já nomeado.
- **Constituição da área de Segurança na organização**: decisão organizacional,
  fora do alcance de uma spec técnica. O ADR-029 registra a condição de re-revisão
  para quando ela ocorrer.
- **Adoção do `.github/CODEOWNERS`**: o placeholder `@owner` é dívida do template e
  não é resolvido aqui. `REP-03` do FND-05 já governa o assunto para diretórios de
  contrato.
- **Promoção do artefato de `draft normativo` para outro estado**: todos os nove
  artefatos FND estão no mesmo estado, e mudá-lo em um só produziria incoerência no
  acervo. O gate de `THR-03` é independente do estado do documento.
