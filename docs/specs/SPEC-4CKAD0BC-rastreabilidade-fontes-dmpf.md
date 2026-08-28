---
id: SPEC-4CKAD0BC
slug: rastreabilidade-fontes-dmpf
title: DMPF — Substituir caminhos efêmeros por âncora versionada nas fontes
stage: done
priority: P2
depends_on: [SPEC-4W1BQK93]
ticket_url: https://lider-cap.atlassian.net/browse/ARQ-490
subtask_urls: []
created: 2026-08-21
---
# SPEC-4CKAD0BC: DMPF — Substituir caminhos efêmeros por âncora versionada nas fontes

## Resumo

Trinta e nove referências espalhadas por dezenove arquivos de `docs/` apontam
para caminhos sob `plans/`, que está inteiro no `.gitignore` (linha 81). Onde a
referência é decorativa isso é inofensivo; onde ela declara a **fonte** de um
conteúdo normativo, produz rastreabilidade aparente — o leitor não consegue abrir
aquilo de que o texto diz derivar, e quem clona o repositório nunca terá o
arquivo.

Esta spec substitui as referências da classe grave por âncora versionada, e
mantém, com redação explícita, as que apenas descrevem o mecanismo de trabalho.

## Contexto

O DMPF nasceu de uma base conceitual (`Parte-1-conceitual.md`) que nunca foi
versionada: ela vive em `plans/references/`, área local declarada fora do
versionamento por decisão registrada em
[ADR-007](../adr/007-fronteira-plugin-template.md) §«`plans/` permanece fora do
versionamento». A decisão continua correta — planos e drafts são artefatos de
trabalho, e versioná-los poluiria o histórico.

O defeito não está na decisão, e sim na **forma de citar**. A RFC resolveu o
problema desde o início e ninguém replicou a solução: ela cita a base conceitual
**pelo nome**, nunca pelo caminho, recepciona os capítulos relevantes em §1.3,
fixa a convenção de citação `Parte-1 §N` e enumera em §1.4 o que já substituiu,
com a regra de que «não há revogação implícita»
(`rfc-dmpf-foundation-v0.1.md:59-68`, `:152-156`).

O FND-08 aplicou essa correção nos seus três pontos ao ser entregue (PR #12,
commit `6399219`), e é o precedente que esta spec generaliza.

A regra do próprio repositório já enuncia o defeito, palavra por palavra:
«apontar para eles a partir de conteúdo versionado cria uma dependência que o
próximo clone do repositório não resolve», e determina que «o conhecimento
durável que nasce neles deve migrar para um artefato versionado (spec ou ADR)
antes de ser citado em código ou em documentação permanente»
(`.claude/rules/ephemeral-refs.md`).

Duas coisas explicam por que a prática sobreviveu mesmo assim. A primeira é que
a mesma regra isenta `docs/specs/**` — isenção correta, porque uma spec precisa
poder citar o plano que a originou; ela cobre o gate, não a consequência de
declarar como **fonte** um arquivo que não existe no clone. A segunda é que a
regra é explícita ao dizer que «orienta a revisão manual; não é aplicada por
hook»: sem gate, a correção depende de alguém olhar.

<constraints>
- [P0] A decisão do ADR-007 não é reaberta: `plans/` permanece fora do versionamento
- [P0] Nenhuma regra normativa muda de conteúdo — a alteração é de forma de citação
- [P0] `Parte-1 §N` permanece válido: é a convenção que a RFC fixa em §1.3
- [P1] Artefatos já promovidos em `develop` só mudam onde a citação declara fonte ou sucessão
- [P1] Referência que descreve o mecanismo de trabalho é preservada, não removida
</constraints>

## Requisitos

### Funcionais

- [ ] **[P0] Fontes normativas ancoradas na RFC**: toda linha que declare `Fontes`,
  `Fonte conceitual` ou `Draft de origem` apontando para `plans/` passa a citar
  `Parte-1 §N` na convenção da RFC, com `docs/dmpf/rfc-dmpf-foundation-v0.1.md`
  como referência versionada
- [ ] **[P0] Sucessão sem referente efêmero**: `politicas-transporte.md:178`
  declara sucessão sobre a base conceitual pelo caminho; passa à mesma âncora que
  o FND-08 adotou
- [ ] **[P1] Metadado de processo explícito**: as linhas `Draft de trabalho` deixam
  de dar caminho e passam a declarar a condição — fora do versionamento, local à
  máquina do autor, não citável como fonte
- [ ] **[P1] Marcadores `ephemeral-ref-ok` revistos**: onde a citação deixar de
  existir, o marcador que a justificava é removido, porque exceção sem objeto vira
  ruído

### Não-funcionais

- [ ] **[P0] Nenhuma alteração de conteúdo normativo**: as regras com ID estável
  permanecem idênticas em texto e em numeração
- [ ] **[P1] Diff auditável**: a mudança é legível linha a linha, sem reformatação
  de parágrafo que a esconda

## Camadas afetadas

Esta spec toca documentação; não há camada de código envolvida.

| Camada | Afeta | Observação |
|--------|-------|------------|
| Domain / Application / Infra | [ ] | Nenhuma |
| Documentação normativa (`docs/dmpf/`) | [x] | Três arquivos: `politicas-transporte.md` e `inventario-as-is.md` na classe A, `README.md` na B |
| Specs (`docs/specs/`) | [x] | Treze specs, entre `done` e `backlog`, mais dois auxiliares em `SPEC-K9H204F1/` |
| ADR (`docs/adr/`) | [ ] | ADR-007 é isento: descrever `plans/` é a função dele |

## Localização de código

| Fase | Caminho |
|------|---------|
| Fontes | `Parte-1 §N`, na convenção que a RFC fixa em §1.3; a referência versionada é `docs/dmpf/rfc-dmpf-foundation-v0.1.md` |
| Alvo | `docs/dmpf/*.md` e `docs/specs/SPEC-*.md`, conforme a matriz de §Design |
| Precedente aplicado | `docs/dmpf/resiliencia-observabilidade.md` §1.5 e `docs/specs/SPEC-E15TBHCD-*.md` |

## Design

### As quatro classes

A correção não é uniforme, e tratá-la como tal produziria dois erros: apagar
metadado honesto de um lado, e deixar fonte inacessível de outro.

| Classe | Critério | Ocorrências | Ação |
|--------|----------|-------------|------|
| **A — fonte normativa** | A linha declara de onde o conteúdo deriva (`Fontes`, `Fonte conceitual`, `Draft de origem`) ou declara sucessão sobre a base | 10 | Trocar pelo par «`Parte-1 §N` + RFC como referência versionada» |
| **B — metadado de processo** | A linha descreve onde o trabalho acontece, e já declara que é local | 21 | Reescrever sem dar caminho; preservar a informação |
| **C — documentação do mecanismo** | O texto existe para explicar a fronteira `plans/` × `docs/` | 4 | Preservar. Citar `plans/` é a função do documento |
| **D — outros drafts locais** | Aponta documento local que não é a Parte-1 (relatórios, docx, prompts) | 4 | Avaliar caso a caso; onde o conteúdo importa, promover a `docs/` |

### Classe A — a lista

| Arquivo | Linha | Como está |
|---------|-------|-----------|
| `docs/dmpf/politicas-transporte.md` | 178 | Sucessão sobre a base pelo caminho |
| `docs/dmpf/inventario-as-is.md` | 6 | `Draft de origem:` |
| `docs/specs/SPEC-6RQBN98G-dmpf-testes-interop.md` | 67 | `Fontes` |
| `docs/specs/SPEC-7H08RZDG-dmpf-cloudevents-protobuf-buf.md` | 68 | `Fontes`, mais um docx |
| `docs/specs/SPEC-7PJ5WVCS-dmpf-uow-inbox-outbox.md` | 118 | `Fonte conceitual` |
| `docs/specs/SPEC-8MNDEWDP-dmpf-upr-decision-mensagens.md` | 83 | `Fontes` |
| `docs/specs/SPEC-8YVF0RR5-dmpf-rfc-limites-deps.md` | 73 | `Fontes` |
| `docs/specs/SPEC-QG2N8STY-dmpf-foundation.md` | 30 | `**Fontes**` |
| `docs/specs/SPEC-XQWGGAXF-dmpf-contexto-erros-seguranca.md` | 92 | `Fontes` |
| `docs/specs/SPEC-YWFGNPG5-dmpf-politicas-transporte.md` | 68 | `Fontes`, mais um docx |

### O padrão de substituição

Aplicado no FND-08 e replicável linha a linha. Em spec:

```text
| Fontes | `plans/references/Parte-1-conceitual.md` §15 |
```

passa a:

```text
| Fontes | `Parte-1 §15`, na convenção de citação que a RFC fixa em §1.3; a
referência versionada é `docs/dmpf/rfc-dmpf-foundation-v0.1.md` |
```

Em artefato normativo, a formulação é a de `resiliencia-observabilidade.md` §1.5:
nomear a base, apontar onde a RFC a recepciona, e declarar que a referência
estável é a RFC.

### Por que não versionar a Parte-1

Seria a solução aparente e está descartada por ADR-007: a base conceitual é
insumo de trabalho, tem 41 KB de texto que a RFC já recepcionou no que importa, e
versioná-la criaria uma segunda fonte normativa concorrente — exatamente o que a
convenção `Parte-1 §N` existe para evitar. A RFC declara qual das duas prevalece
(§1.3); duplicar o arquivo reabriria a questão.

### Por que não usar `ephemeral-ref-ok` em tudo

O marcador isenta o gate e não corrige o defeito: a referência continua sem
resolver para quem lê. Ele é adequado onde citar o efêmero **é** a informação —
classe C —, e inadequado onde a citação pretende ser fonte.

## Decisões técnicas

- **A RFC é a âncora, não um arquivo novo**: nada é criado; usa-se o que já está
  versionado e já foi projetado para esse papel.
- **Correção por classe, não em lote cego**: um `sed` global apagaria os
  metadados honestos da classe B e mutilaria o ADR-007.
- **Specs `done` são editáveis nesta correção**: a alteração não muda o que a
  spec especificou, apenas como ela cita a fonte. O `stage` não regride.
- **Um PR só**: a mudança é homogênea e de baixo risco; fatiá-la por arquivo
  multiplicaria revisão sem reduzir risco.

## Verificação e testes

### Critérios de aceite

- [ ] Nenhuma linha de `docs/` declara fonte normativa apontando para `plans/`
- [ ] `git grep -n 'plans/' -- 'docs/**/*.md'` retorna apenas ocorrências das
  classes B e C, todas com redação que declara o artefato como não versionado
- [ ] Nenhum ID de regra mudou: o conjunto de IDs `normativo` por artefato é
  idêntico antes e depois
- [ ] Nenhum marcador `ephemeral-ref-ok` sobra sem objeto
- [ ] As citações novas a `rfc-dmpf-foundation-v0.1.md:N` resolvem para o
  conteúdo afirmado
- [ ] **Integridade das referências físicas de terceiros** (obrigação herdada de
  SPEC-4W1BQK93, RF5-C4): esta entrega altera `inventario-as-is.md:6` e
  `politicas-transporte.md:178`, e os dois arquivos são alvo de referência física
  a partir de linhas **posteriores** a essas. Sete grupos estão em risco de
  deslocamento — no inventário, as linhas 77, 146 e 175-183, todas citadas pela
  RFC; em políticas, as linhas 189, 269, 1186-1188 e **1757**, todas citadas por
  `resiliencia-observabilidade.md`, esta última pelo shorthand `:1757` da linha
  295. O PR satisfaz **uma** das duas condições abaixo, e declara qual:
  - **(a) preservação**: a contagem de linhas dos dois arquivos não muda, e cada
    um dos sete grupos é comparado com o preimage do commit base, linha a linha,
    com zero divergências; ou
  - **(b) atualização explícita**: todo lexema de origem afetado é atualizado
    **antes** de o manifesto ser regenerado, com diff allowlisted e prova
    antes/depois por referência. Regenerar `line-refs.json` sobre um alvo
    deslocado devolveria C4 ao verde com a referência já tendo perdido o
    significado — o manifesto abençoaria o defeito em vez de acusá-lo.

  "Revisar os deltas" não satisfaz nenhuma das duas: não é condição decidível e
  não produz evidência conferível no PR.

### Cenários de teste

```text
DADO um leitor que clonou o repositório sem acesso à máquina do autor
QUANDO ele abre uma spec do DMPF e procura a fonte do conteúdo
ENTÃO encontra `Parte-1 §N` com a RFC versionada como referência, e consegue
     ler na RFC o que a Parte-1 é e o que dela já foi consolidado

DADO o artefato `politicas-transporte.md` após a correção
QUANDO §1.5 declara a sucessão sobre a base conceitual
ENTÃO a declaração aponta a RFC, e nenhum caminho sob `plans/` aparece

DADO o ADR-007, que documenta a fronteira entre `plans/` e `docs/`
QUANDO a correção é aplicada
ENTÃO o ADR permanece intacto, porque citar `plans/` é a função dele
```

<critical_constraints>
- A decisão do ADR-007 permanece: `plans/` fica fora do versionamento
- Nenhuma regra normativa muda de conteúdo, texto ou ID
- `Parte-1 §N` continua sendo a convenção válida de citação, fixada por RFC §1.3
- Referência que descreve o mecanismo de trabalho é preservada
</critical_constraints>

## Escopo fora

- **Versionar a base conceitual**: descartado, com o motivo em §Design.
- **Revisar o conteúdo normativo dos artefatos**: esta spec é de forma de
  citação; qualquer defeito de conteúdo encontrado no caminho vira registro
  próprio.
- **Alterar `.claude/rules/ephemeral-refs.md`**: a isenção de `docs/specs/**`
  continua correta para o caso que ela cobre, e transformar a regra em hook é
  decisão à parte — ela própria declara que orienta revisão manual.
- **Os drafts da classe D que mereçam promoção**: se um relatório local for
  julgado durável, promovê-lo a `docs/` é trabalho separado.
