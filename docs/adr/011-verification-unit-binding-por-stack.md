# ADR-011: Adotar a verification_unit como unidade arquitetural com binding idiomático por stack

## Status

Aceito — 2026-08-28. Implementa SPEC-DBTRMM3X.

## Contexto

A regra de dependência do DMPF só é verificável se houver resposta inequívoca a
duas perguntas: *que coisa* é classificada em um bloco e *como* se descobre a
qual coisa um arquivo pertence (RFC §3). Sem essas respostas, o grafo de
dependências fica sem vértice estável e nenhuma aresta pode ser afirmada com
confiança.

O obstáculo é que a palavra "módulo" é usada informalmente para artefatos
distintos, e tratá-los como um só é a origem de boa parte da ambiguidade (RFC
§3.1). A RFC separa quatro conceitos: o `ownership_module` (o que tem ciclo de
vida próprio, é versionado e publicado), a `verification_unit` (o menor conjunto
de código a que uma classificação de bloco se aplica integralmente), o
`metadata_container` (onde a classificação é declarada) e a `canonical_key` (o
identificador estável que serve de chave do grafo). Em particular,
`ownership_module` e `verification_unit` **não coincidem**: em Go, uma lib é um
módulo com uma camada, mas um app é um módulo com todas as camadas — e a
classificação nunca se aplica ao `ownership_module`.

O inventário confirma que fronteira de camada não é fronteira de projeto:
`legado-titulos-services/apps/api` é um único projeto Nx que contém
`src/{domain,application,infra}`, e `legado-rendas-bff` sequer usa Nx (RFC §3.4).
Um binding que dependesse do projeto agruparia três blocos em uma unidade só; um
binding derivado da posição do arquivo transformaria um `git mv` em
reclassificação normativa silenciosa.

As forças em jogo estão codificadas em seis invariantes — cobertura total (I1),
não sobreposição (I2), independência de tooling (I3), opacidade a alias (I4),
move/rename explícito (I5) e determinismo (I6) —, que são o critério de
aceitação de qualquer binding, não preferências (RFC §3.2). A tensão central: a
unidade precisa satisfazer as seis invariantes e, ao mesmo tempo, ser adotável
sobre os layouts heterogêneos que já existem nos repositórios, sem exigir
reestruturação prévia.

## Decisão

- **Adotar um contrato abstrato único de `verification_unit`** — o menor
  conjunto de código a que uma classificação de bloco se aplica integralmente
  (RFC §3.1) — governado pelas seis invariantes I1–I6 como critério de aceitação
  de qualquer binding físico (RFC §3.2).
- **O binding é idiomático por stack.** Em Go, a `verification_unit` é o
  **package** (diretório), com `canonical_key` igual ao import path completo do
  package (RFC §3.3). Em TypeScript, é o **root declarado em manifesto**
  (`dmpf-units.json`, um por `ownership_module`, com `include` disjuntos), com
  `canonical_key` igual a `<nome do pacote>#<id da unidade>` (RFC §3.4).
- **O manifesto é declaração, não descoberta.** Caminho não coberto por nenhum
  `include` não é classificado por inferência, e sobreposição entre dois
  `include` é erro de configuração, não ambiguidade a resolver por precedência
  (RFC §3.4).
- **A resolução de origem e destino de uma aresta opera sobre o arquivo real
  resolvido**, nunca sobre o texto do import — do que decorre I4 (RFC §3.5).
  Imports que não resolvem para o universo são dependências externas e seguem
  §6, não a matriz de §7.
- **Todos os casos degenerados falham fechados** (RFC §3.6): a lacuna de
  configuração reprova, em vez de ser lida como conformidade.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Binding por pasta de camada em TypeScript (classificação derivada da posição do arquivo, ex.: `src/domain/**`) | Viola I5: um `git mv` reclassifica em silêncio, transformando movimentação em mudança normativa sem intenção declarada nem revisão. O `rationale` de RFC §3.2 enquadra esse binding como o mesmo vetor de reclassificação silenciosa que §10 combate no metadado; o `rationale` de RFC §3.4 fecha o argumento — só o manifesto faz a reclassificação exigir uma edição visível, revisável em PR. |
| TS project references como `verification_unit` (alternativa D) | Satisfaz as seis invariantes, mas exigiria reestruturar os repositórios em subprojetos. O `rationale` de RFC §3.4 escolhe o manifesto justamente por satisfazê-las **sem** reestruturação prévia sobre o layout existente; a alternativa D fica registrada como caminho de evolução opcional (um repositório que já a use pode derivar o manifesto dela), não como exigência de conformidade. |
| Convenção ou chave sintética própria para a `verification_unit` em Go, em vez de reusar o package e seu import path | Desnecessário: o `rationale` de RFC §3.3 registra que o package é a unidade que o próprio compilador reconhece e sobre a qual a análise de imports opera nativamente, e que o import path já é identificador estável e único, sem convenção adicional. As seis invariantes valem por construção da linguagem, com I5 coberta pela cláusula `package` no topo do arquivo, que o compilador exige coerente. |

## Consequências

**Positivas:**

- O grafo ganha um vértice inequívoco e estável: toda aresta liga duas
  `verification_unit`, com origem e destino resolvidos sobre o arquivo real (I4),
  o que torna a regra de dependência de §7 verificável.
- O binding é adotável sobre os layouts que já existem no inventário — Go
  hexagonal com `internal/`, Go em camadas sob `src/`, TS Nx multi-camada e TS
  projeto único — sem exigir reestruturação prévia dos repositórios.
- Move/rename deixa de reclassificar em silêncio (I5): em Go a cláusula
  `package` torna a mudança visível ao compilador; em TS a reclassificação exige
  edição do manifesto, revisável em PR.
- O verde do verificador passa a significar cobertura provada (I1), não
  ausência de erro detectado: um arquivo de produção órfão ou uma
  `canonical_key` duplicada reprovam o baseline em vez de passarem
  despercebidos (RFC §3.6).
- Separar `ownership_module` de `verification_unit` desfaz a ambiguidade de
  "módulo" e permite classificar um app (um módulo, várias camadas) camada a
  camada.

**Negativas:**

- **Custo aceito:** em TypeScript, cada `ownership_module` passa a exigir e
  manter à mão um `dmpf-units.json` sincronizado com a estrutura de pastas — um
  artefato de configuração novo cuja desatualização (arquivo fora de todos os
  `include`) reprova por §3.6 em vez de degradar suavemente. Esse overhead de
  manutenção foi aceito em troca de I5 por construção e da reclassificação
  revisável em PR.
- Assimetria entre stacks: Go usa unidade nativa (package) e TS usa unidade
  declarada (manifesto), então o mesmo conceito abstrato tem duas formas físicas
  e dois formatos de `canonical_key`, ampliando a superfície do verificador e do
  onboarding.
- A alternativa D (project references), potencialmente mais robusta, fica
  adiada como evolução opcional; repositórios que já a usam não colhem benefício
  automático sem derivar o manifesto.
- Erros de configuração do manifesto (sobreposição de `include`, caminho não
  coberto) são tratados como falha dura, não resolvidos por precedência — exigem
  disciplina de configuração em vez de tolerância a ambiguidade.

## Referências

- ADR-010 — regra de dependência e os seis blocos. Decide a função de decisão e
  a matriz de blocos de §7, sobre seis blocos de pertencimento único; importa
  aqui porque a `verification_unit` é o vértice sobre o qual essa regra opera e a
  que a classificação de bloco se aplica integralmente.
- ADR-012 — classificação por metadado declarado. Ancora o valor de `block` em
  metadado declarado por unidade, recusando inferir da convenção de diretório;
  esta decisão define o vértice, mas delega ao ADR-012 como a classificação é
  atribuída a cada unidade.
- ADR-015 — capabilities externas por bloco. Governa o regime das dependências
  externas; importa aqui porque um import que não resolve para o universo é
  dependência externa e segue esse regime (RFC §6), não a matriz de §7.
- Spec: `SPEC-DBTRMM3X` (em `docs/specs/`).
- RFC DMPF: `docs/dmpf/rfc-dmpf-foundation-v0.1.md` — §3 (origem da decisão:
  binding por stack, invariantes I1–I6 e resolução do grafo), §13.2 (acionamento
  `ADR-DMPF-B`).
