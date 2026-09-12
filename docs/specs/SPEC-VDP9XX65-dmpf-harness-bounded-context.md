---
id: SPEC-VDP9XX65
slug: dmpf-harness-bounded-context
title: DMPF KRN-12.2h — Harness de agentes para criar bounded contexts (AI SDD)
stage: done
priority: P2
depends_on: [SPEC-H1A190Y8, SPEC-XMNBMY50]
ticket_url: null
subtask_urls: []
created: 2026-09-09
---

# SPEC-VDP9XX65: DMPF KRN-12.2h — Harness de agentes para criar bounded contexts (AI SDD)

## Resumo

Sub-spec da reescrita do KRN-12.2 que substitui o generator orientado ao
domínio (sub-specs a/b/c, deferidas) por um **harness de agentes**: um agente,
uma skill com o golden path, rules com as normas aplicáveis e um command que,
a partir de uma **spec de bounded context** escrita num template próprio,
produzem o contexto completo nos cinco blocos DMPF. O esqueleto determinístico
(módulos, tags, targets, manifesto, `go.work`) continua vindo do generator
`bounded-context` já entregue; o código de negócio — agregados, UPRs, eventos,
casos de uso, repositórios, rotas, contrato `.proto` — é escrito pelo agente a
partir da spec, e a garantia não é determinismo de bytes, mas os **gates** que
o repositório já tem: verificador de conformidade, cadeia Go, prova em worktree,
`biome`/`gofmt`, testes. É o processo de desenvolvimento do DMPF aplicado a si
mesmo: spec-driven, com agentes executando e gates mecânicos julgando.

Como squad, quero escrever a spec do meu bounded context, rodar um command e
receber um contexto que compila, testa e é aprovado pelo verificador — com a
regra de negócio escrita, não como `TODO`.

## Contexto

- **Problema**: um generator determinístico que *entende* o domínio precisa de
  uma DSL rica (pré-condições, efeitos, bindings, drift de contrato,
  inventário de regeneração) — duas revisões externas das sub-specs a/b/c
  somaram 24 achados, 16 bloqueantes, e a complexidade só cresce. Enquanto
  isso, na mesma sessão, um agente gerou o bloco `domain` completo a partir da
  spec, com testes rodando e `0 issues` no lint com depguard/forbidigo,
  reprovando apenas no gate normativo (D002) — onde um gate deve reprovar.
- **Impacto**: a spec do contexto vira a única fonte do domínio (sem DSL
  paralela); o agente escreve regra de negócio; o gate acusa erro; o time
  revisa um PR, não preenche stubs.
- **Inspiração**: o próprio fluxo deste repositório (`skill-feature-pipeline`,
  agentes em `.claude/agents/`, skills em `.claude/skills/` e
  `.agents/skills/dmpf-testkit`), o golden path de `docs/guides/dmpf-manifesto.md`,
  e os módulos `example/orders`/`example/reservations` como forma canônica.
- **Links relevantes**:
  - `SPEC-H1A190Y8` — guarda-chuva: plugin, esqueleto, prova, CI, docs
  - `SPEC-XMNBMY50` — shared kernel; sem ela o contexto gerado reprova em
    `DMPF-D002`
  - `SPEC-8FSD8505`, `SPEC-VZ16X0MS`, `SPEC-F7S5B6KV` — caminho determinístico
    deferido; seus requisitos descrevem, em detalhe, o que o agente precisa
    produzir (pré-condições por comando, efeitos, eventos, consultas,
    integração, contrato pelo rito Buf) e servem de checklist à skill
  - FND-03, FND-04 §3.2/§5/§6, FND-05, FND-06 §9/§11; ADR-012, ADR-017,
    ADR-032, ADR-033, ADR-034, ADR-035, ADR-036, ADR-041 (addendum)
  - `.claude/README.md` — índice gerado do conteúdo de `.claude/`

<constraints>
- [P0] O agente NUNCA escreve `dmpf-units.json`, `project.json`, `go.mod`, `go.work` à mão: o esqueleto é do generator `bounded-context`; o agente só acrescenta packages ao `include` pelo merge por campo do manifesto.
- [P0] O agente NUNCA regrava baselines (`units-baseline.json`, baseline Buf) nem `gen/go`: classificação e rito Buf são passos humanos/CI nomeados pela skill.
- [P0] Todo contexto produzido pelo harness passa pelos mesmos gates que qualquer módulo: `fmt-check`, `vet`, `build`, `lint` (depguard/forbidigo por bloco), `test-race`, `dmpf-conformance --base`, `biome ci`; nenhum gate é afrouxado para código de agente.
- [P0] A spec de bounded context segue o template dedicado; o command recusa spec fora do template (seções obrigatórias ausentes).
- [P1] O golden `bookings` é regenerado pelo harness a cada mudança do harness e comparado com o gate; divergência que reprova é regressão do harness.
- [P1] Prosa em PT-BR; código e godoc em inglês; comentários só pelos três critérios do repositório.
</constraints>

## Requisitos

### Funcionais

- [ ] **[P0] Template de spec de bounded context** em
  `.agents/skills/dmpf-bounded-context/references/template-bounded-context.md`
  — vive na skill, que é quem o consome e o valida —, com o mesmo frontmatter
  de 9 campos do catálogo (compatível com `spec-query.sh`) e corpo próprio,
  distinto do template de feature do `skill-feature-pipeline`. Seções
  obrigatórias: **Identidade** (`bounded_context` declarado, `name` do módulo,
  linguagem ubíqua — glossário de termos com definição de uma linha);
  **Agregados** (por agregado: identidade `generated` ou natural, campos
  tipados, estados e transições permitidas, invariantes de estado);
  **Comandos (UPRs)** (por comando: entrada, pré-condições com o código de
  rejeição, efeitos sobre o estado, eventos emitidos, resposta; qual comando
  cria ou inicializa o agregado); **Eventos de domínio** (campos e origem de
  cada campo); **Consultas** (nome, filtros, cardinalidade); **Relações entre
  agregados** (por identidade, nunca por objeto); **Integração** (eventos
  publicados como integration events, contratos consumidos com `messageFQN`
  e mapeamento para comando local, disposições de consumo); **Políticas
  transversais** (idempotência, autorização, auditoria — por referência às
  normas, não reescritas); **Critérios de aceite** (os gates + um cenário
  por comando: aceite e cada rejeição); **Escopo fora**. Um exemplo
  preenchido acompanha o template: `SPEC-<id>-bookings.md` (o golden).
  `docs/specs/README.md` ganha a seção "Specs de bounded context" apontando
  para o template na skill e dizendo como o catálogo as distingue (título com
  prefixo "Bounded context —" e a seção Identidade obrigatória); a spec
  preenchida continua em `docs/specs/`, como qualquer spec.
- [ ] **[P0] Agente `dmpf-context-author`** em `.claude/agents/`: frontmatter no
  padrão do repositório (ferramentas de leitura/escrita/`Bash`, modelo, skills
  carregadas: `dmpf-bounded-context`, `skill-go`, `skill-go-testing`,
  `skill-ddd`); instruções: ler a spec do contexto, rodar o generator para o
  esqueleto, escrever os cinco blocos copiando a forma de `example/orders` e
  `example/reservations`, escrever o `.proto` dos eventos publicados no
  padrão `contracts/proto/company/<goIdent>/event/v1/`, rodar a cadeia e o
  verificador até passar, parar em qualquer gate normativo e reportar.
  Regras duras carregadas (esqueleto pelo generator, baselines e `gen/go`
  fora, comentários pelos três critérios, sem `git commit`).
- [ ] **[P0] Skill `dmpf-bounded-context`** em `.agents/skills/` (ao lado de
  `dmpf-testkit`): o golden path passo a passo, cada passo com a norma que o
  rege e o arquivo que o exemplifica — (1) validar a spec contra o template;
  (2) `pnpm nx g @mateusmacedo/dmpf-plugin:bounded-context <name>
  --bounded-context <ctx>`; (3) `domain` por agregado (forma de
  `example/orders`; desfecho `(Accepted[R], *Rejection)`; sem `time`, sem
  porta; `Instant` como inteiro); (4) `port` (`Resources`, `Reader`, `Bind`);
  (5) `application` (nove passos de FND-04 §3.2 por comando; consultas fora
  da UoW; `Consume` pelas sete disposições); (6) `provider-postgres`
  (`schema.sql`, repositórios com optimistic locking, mapper para o payload
  do contrato, `Reader`); (7) `app` (rotas com `ContractRef`, OpenAPI em
  `contracts/openapi/<name>/v1/`, consumer com `envelope.Unpack`, e2e); (8)
  contrato: `.proto` + `(cd contracts && bash ../tools/buf.sh generate)` + os
  quatro gates Buf + unidade `contract` do contexto no manifesto do
  `dmpf-contracts` por merge; (9) `include` dos packages novos nos
  manifestos; (10) classificação (`--write-baseline` em commit próprio — passo
  humano); (11) cadeia, verificador com `--base`, testes; (12) checklist final
  (tabela de "o que o generator nunca toca", o que é do agente, o que é
  humano). Inclui a lista de armadilhas verificadas nesta implementação
  (D002 sem shared kernel; `time` no domain; `require` no `go.mod`;
  `exceptions` do manifesto; `pnpm install` após gerar).
- [ ] **[P0] Rules DMPF** em `.claude/rules/dmpf-bounded-context.md`: as normas
  que o agente precisa obedecer ao escrever um contexto, em forma de regras
  curtas com referência — matriz de blocos e C2 (ADR-010/017 + shared
  kernel), classificação declarada (ADR-012), desfecho da UPR (ADR-032),
  fronteira de UoW e outbox (ADR-034/035), inbox e disposições (ADR-036),
  contrato pelo rito Buf (ADR-033, PTB-01/REP-01), REST (RST-02/04),
  observabilidade mínima. Complementa, não repete, `.claude/rules/*` existentes.
- [ ] **[P0] Command `/dmpf-new-context <SPEC-ID>`** em `.claude/commands/`
  (padrão dos commands existentes): resolve a spec pelo catálogo, valida o
  template, invoca o agente com a skill, e ao final imprime o rito humano
  restante (rito Buf, classificação, PR). Recusa spec fora do template ou com
  `stage` diferente de `planning`/`building`.
- [ ] **[P0] Golden `bookings` commitado**: a partir de uma spec de exemplo
  (`SPEC-<id>-bookings.md`, "Bounded context — bookings", contexto
  `resource-scheduling`; agregados `Booking` e `Resource` como nas sub-specs
  deferidas), o harness gera os cinco módulos `libs/backend/go/bookings/{domain,ports,application,provider,app}`, o
  contrato `contracts/proto/company/bookings/event/v1/booking_reserved.proto`
  com `gen/go` pelo rito, e a unidade `resource-scheduling/contract`. Entram no
  workspace como módulos reais: tags, `layer:*`, `go.work`, baseline,
  `test-race` com Postgres, aprovados pelo verificador (exige
  `SPEC-XMNBMY50`). São a referência viva do que o harness produz.
- [ ] **[P1] Prova de regressão do harness** `tools/dmpf-harness-check.sh`:
  gera de novo o `bookings` num worktree descartável a partir da mesma spec
  (passo humano/local — exige LLM), roda os gates e reporta divergências
  contra o golden commitado; **não roda no CI** (custo, credenciais, não
  determinismo). O CI prova o golden como módulos normais e a prova do
  esqueleto (`dmpf-generator-check.sh --phase structural|self-test`).
- [ ] **[P1] Documentação**: `docs/guides/dmpf-composicao.md` reescrito para o
  fluxo AI SDD (escrever spec → command → rito Buf → classificação → PR);
  `AGENTS.md` (Comandos, Diretórios, o harness); `docs/nx-reference/tasks.md`;
  `.claude/README.md` (índice regenerado com agente, rule e command novos);
  `tools/dmpf-plugin/README.md` (o generator como esqueleto do harness);
  addendum no ADR-041 registrando a virada para o híbrido.

### Não-funcionais

- [ ] Todo `.go` do golden passa `gofmt -l`, `go vet`, `golangci-lint` do
  workspace (depguard/forbidigo por bloco) sem exceção nova; testes rodam de
  fato (`ok` por package).
- [ ] `dmpf-conformance --root . --base <antes>` aprova o workspace com o golden.
- [ ] Sem dependência npm ou Go nova além das já decididas nas specs
  anteriores; o harness é Markdown (agent, skill, rule, command).
- [ ] Prosa PT-BR revisada; godoc em inglês revisado.

## Camadas afetadas

| Camada (bloco DMPF) | Afetada? | O que muda |
| --- | --- | --- |
| Kernel | [ ] | Consumido como shared kernel (`SPEC-XMNBMY50`) |
| Golden `bookings` (cinco blocos + contrato) | [x] | Módulos novos permanentes gerados pelo harness |
| Harness (`.claude/`, `.agents/`) | [x] | Agente, skill, rule, command; índice regenerado |
| Catálogo de specs | [x] | Seção no README apontando para o template (na skill); spec `bookings` |
| Prova / CI | [x] | `dmpf-harness-check.sh` (local); CI prova golden + esqueleto |
| Documentação | [x] | Guia, `AGENTS.md`, `tasks.md`, README do plugin, ADR-041 |

## Localização de código

```text
docs/specs/README.md                              — MODIFICAR: seção "Specs de bounded context" (aponta para o template na skill)
docs/specs/SPEC-<id>-bookings.md                  — NOVO: spec de exemplo (entrada do golden)
.claude/agents/dmpf-context-author.md             — NOVO
.agents/skills/dmpf-bounded-context/SKILL.md      — NOVO
.agents/skills/dmpf-bounded-context/references/template-bounded-context.md — NOVO: template dedicado (frontmatter de 9 campos + corpo próprio)
.agents/skills/dmpf-bounded-context/references/    — checklist por bloco, armadilhas verificadas
.claude/rules/dmpf-bounded-context.md             — NOVO
.claude/commands/dmpf-new-context.md              — NOVO
.claude/README.md                                 — MODIFICAR: índice
libs/backend/go/bookings/{domain,ports,application,provider,app}/ — NOVO (golden, pelo harness; uma pasta por contexto — ADR-030)
tools/dmpf-plugin/src/generators/bounded-context/{blocks,generator}.ts — MODIFICAR: módulos em <directory>/<name>/<bloco>
.golangci.yml, tools/dmpf-gate-check.sh                      — MODIFICAR: depguard/forbidigo alcançam **/domain/**, **/ports/**, **/application/**
docs/adr/030-granularidade-modulo-go-e-bom.md      — MODIFICAR: addendum (contexto de negócio em pasta própria)
contracts/proto/company/bookings/event/v1/*.proto — NOVO (golden); gen/go pelo rito Buf
libs/backend/go/dmpf-contracts/dmpf-units.json    — MODIFICAR: unidade resource-scheduling/contract
tools/dmpf-baseline/units-baseline.json           — MODIFICAR: classificação do golden (commit próprio)
go.work                                           — MODIFICAR: cinco use (pelo generator)
tools/dmpf-harness-check.sh                       — NOVO (local; não roda no CI)
docs/guides/dmpf-composicao.md, AGENTS.md, docs/nx-reference/tasks.md, tools/dmpf-plugin/README.md, docs/adr/041-*.md — MODIFICAR
```

## Design

### Arquitetura

```text
  docs/specs/SPEC-<id>-<ctx>.md (pelo template em .agents/skills/dmpf-bounded-context/references/)
   │  /dmpf-new-context SPEC-<id>
   ▼
 command ── valida template ── invoca agent dmpf-context-author (skill dmpf-bounded-context + rules)
   │  (1) generator: esqueleto dos cinco módulos + go.work        (determinístico)
   │  (2) agente escreve domain/port/application/provider/app + .proto + OpenAPI  (julgamento)
   │  (3) agente roda gates e corrige até passar; para em gate normativo
   ▼
 rito humano: buf.sh generate + 4 gates Buf → --write-baseline (commit próprio) → commit do código → PR
 gates (CI): fmt/vet/build/lint · test-race · dmpf-conformance --base · biome ci · prova do esqueleto · golden bookings como módulos
```

### Fluxo principal

1. A squad escreve `SPEC-<id>-<ctx>.md` pelo template e leva ao `planning`.
2. `/dmpf-new-context SPEC-<id>`: o command valida as seções obrigatórias e
   invoca o agente.
3. O agente roda o generator (esqueleto), escreve o código por bloco seguindo
   a skill, escreve o `.proto` e o OpenAPI, acrescenta packages ao `include`.
4. O agente roda `fmt-check`, `vet`, `build`, `lint`, testes e o verificador
   com `--base`; corrige até passar; se um gate normativo reprovar (ex.: D002
   sem shared kernel), para e reporta.
5. A squad executa o rito Buf, a classificação em commit próprio e abre o PR;
   o CI prova o contexto como qualquer módulo.

## Decisões técnicas

- **Híbrido: esqueleto pelo generator, código pelo agente** porque o esqueleto
  é a parte que não admite erro (tags, targets, manifesto, `go.work`) e é
  trivialmente determinística; o código de negócio é a parte que exige
  julgamento e onde a DSL determinística explodiu em complexidade (24
  achados). Alternativas descartadas: harness puro (perderia o esqueleto sem
  erro); generator orientado ao domínio (custo — sub-specs a/b/c deferidas).
- **Garantia pelos gates, não por bytes idênticos** porque é o princípio do
  DMPF: classificação declarada e gates fail-closed. Dois runs do agente podem
  divergir em forma; ambos precisam passar nos mesmos gates.
- **Template próprio de spec de bounded context, guardado na skill** porque a
  spec de feature descreve *uma mudança*; a spec de contexto descreve *um
  modelo* — agregados, comandos, eventos, integração — e é a única fonte do
  domínio (sem DSL paralela). Fica na skill porque é ela quem o consome e
  valida; `docs/specs/` guarda só specs preenchidas. Mesmo frontmatter para o
  catálogo continuar funcionando.
- **Golden commitado, prova de regressão local** porque geração por LLM não
  cabe no CI (custo, credenciais, não determinismo); o CI prova o golden como
  módulos reais e o esqueleto; a regressão do harness é detectada
  regenerando o golden localmente. Alternativa descartada: golden em worktree
  no CI.
- **Skill em `.agents/skills/`** porque é o lugar das skills de workspace
  DMPF (`dmpf-testkit`); agente, rule e command ficam em `.claude/` como os
  demais.

## Regras relacionadas

- ADR-010, 012, 017, 032, 033, 034, 035, 036, 041; FND-03, FND-04, FND-05, FND-06.
- `SPEC-H1A190Y8` (guarda-chuva), `SPEC-XMNBMY50` (shared kernel), sub-specs a/b/c deferidas (checklist do que produzir).

## Verificação e testes

### Critérios de aceite

- [x] `.agents/skills/dmpf-bounded-context/references/template-bounded-context.md`
  existe com as dez seções obrigatórias; `SPEC-<id>-bookings.md` o preenche; `spec-query.sh` lista
  ambas as specs (a de `bookings` com o título prefixado).
- [x] `/dmpf-new-context` recusa spec sem uma seção obrigatória, nomeando-a.
- [x] O golden `bookings` está no workspace: cinco módulos com tags e
  `layer:*`, `go.work`, unidade `contract`, baseline; `pnpm nx run-many -t
  fmt-check,vet,build,lint -p bookings-*` verde; `DMPF_PG_DSN=… test-race`
  dos cinco verde, inclusive e2e; `dmpf-conformance --base` aprova o
  workspace.
- [x] `tools/dmpf-harness-check.sh` regenera `bookings` num worktree e reporta
  os gates; uma execução documentada no CHECKPOINT com o resultado. Executada em
  2026-09-12: o agente reescreveu o contexto em 80 arquivos a partir da mesma
  spec, e passaram `biome ci`/`gofmt -l`, o rito Buf, os dois commits, a cadeia
  `fmt-check,vet,build,lint` nos cinco módulos, o `test-race` do provider e do
  app e o verificador (`conforme`). **Ressalva:** o script abortou no `test-race`
  por ambiente, não por defeito do contexto regenerado — ver a armadilha do
  schema residual no CHECKPOINT; os dois passos finais foram concluídos à mão
  sobre o mesmo commit (`refs/regen/bookings`).
- [x] `.claude/README.md` regenerado lista agente, rule e command novos;
  `pt-reviewer`/`en-reviewer` ✓ em tudo que tem prosa.
- [x] Cadeia completa do workspace verde; `biome ci .` verde.

### Cenários de teste

```text
DADO SPEC-<id>-bookings.md pelo template, em planning, e o shared kernel designado
QUANDO /dmpf-new-context SPEC-<id> roda
ENTÃO o esqueleto vem do generator, os cinco módulos recebem código por agregado, o .proto e o OpenAPI existem, e a cadeia + verificador passam no worktree do agente, que termina imprimindo o rito humano

DADO uma spec de bounded context sem a seção "Comandos (UPRs)"
QUANDO /dmpf-new-context roda
ENTÃO recusa nomeando a seção ausente e nada é escrito

DADO o golden bookings commitado e uma mudança na skill que troca a forma do desfecho da UPR
QUANDO tools/dmpf-harness-check.sh roda localmente
ENTÃO o bookings regenerado reprova no gate (lint ou verificador) e a prova reporta a divergência como regressão do harness

DADO o workspace sem SPEC-XMNBMY50 aplicada
QUANDO o agente roda o verificador sobre bookings
ENTÃO D002 aponta bookings-domain → dmpf-kernel/domain e o agente para reportando o gate normativo, sem contornar
```

<critical_constraints>
- [P0] O agente NUNCA escreve `dmpf-units.json`, `project.json`, `go.mod`, `go.work` à mão: esqueleto pelo generator; `include` por merge.
- [P0] O agente NUNCA regrava baselines nem `gen/go`: classificação e rito Buf são passos humanos/CI.
- [P0] Todo contexto do harness passa pelos mesmos gates que qualquer módulo; nenhum gate é afrouxado.
- [P0] Spec fora do template é recusada pelo command.
- [P1] O golden `bookings` é regenerado a cada mudança do harness e comparado com o gate.
- [P1] Prosa em PT-BR; código e godoc em inglês; comentários pelos três critérios.
</critical_constraints>

## Escopo fora

- **Geração por LLM no CI**: custo, credenciais e não determinismo; o CI prova
  o golden e o esqueleto.
- **Generator orientado ao domínio** (DSL, `--update`, inventário): sub-specs
  a/b/c, deferidas.
- **Composition root (`cmd/` com `--role`)**: continua sendo copiar
  `dmpf-reference`; a skill aponta o passo.
- **Revisão automática do código do agente**: o PR segue o fluxo humano do
  repositório (`skill-code-review`, `pr-guardian`); o harness não aprova a si
  mesmo.
