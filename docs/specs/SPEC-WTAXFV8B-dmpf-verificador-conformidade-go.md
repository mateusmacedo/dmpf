---
id: SPEC-WTAXFV8B
slug: dmpf-verificador-conformidade-go
title: DMPF KRN-02 — Verificador de conformidade DMPF em Go
stage: done
priority: P0
depends_on: [SPEC-MQA5HAXF]
ticket_url: null
subtask_urls: []
created: 2026-09-01
---

# SPEC-WTAXFV8B: DMPF KRN-02 — Verificador de conformidade DMPF em Go

## Resumo

Entregar `conformance`, uma CLI Go que decide a regra de dependência do DMPF
sobre o grafo real de imports do workspace, lendo a classificação declarada nos
manifestos `dmpf/units@1` e reprovando o PR no CI. Como time de plataforma,
queremos que a conformidade arquitetural seja decidida por máquina — sobre o
arquivo resolvido, não sobre o texto do import, e transitivamente entre módulos —
para que os dez módulos que `KRN-03`..`KRN-12` vão criar nasçam sob gate em vez
de sob revisão manual.

Esta é a segunda metade do Incremento 1 da spec guarda-chuva
[SPEC-YRJRADY9](./SPEC-YRJRADY9-dmpf-kernel-sdk-go.md): "Módulo Go compila, valida
e é verificado pela regra de dependência no CI". A primeira metade,
[SPEC-MQA5HAXF](./SPEC-MQA5HAXF-dmpf-fundacao-nx-go.md) (`KRN-01`), entregou o
terreno e um gate deliberadamente parcial; esta entrega o substitui pelo gate
completo.

## Contexto

A regra de dependência do DMPF é hoje **quase inteiramente documental**. O
ADR-010 a define como a função
`decide(source_block, target_block, source_bc, target_bc, target_surface)`,
conjunção de dois predicados: **C1**, o par `(bloco de origem, bloco de destino)`
permitido na matriz 6×6 da RFC §7.3, e **C2**, o predicado de contexto — mesmo
`bounded_context`, ou destino que seja superfície pública de integração. RFC
§7.2 e §5.5, ambas `normativo`, definem essa superfície por dois caminhos: a
unidade **é `contract package`**, ou declara `public_integration_surface: true`.
Uma unidade não declarada é interna ao seu context — o default é privado. Reprovar em
qualquer uma reprova a aresta.

O `KRN-01` entregou um gate parcial e **declarou a própria limitação em código**.
O bloco `domain` do `.golangci.yml` usa `depguard` com `list-mode: strict` e uma
allowlist de stdlib pura, e o comentário que o acompanha nomeia esta história
como sucessora (`.golangci.yml`, regra `domain`):

> ALCANCE (limitação declarada, não resolvida aqui): a regra seleciona por nome
> de diretório, não pelo `dmpf-units.json`, e o golangci-lint analisa um módulo
> por vez. Ela NÃO vê a aresta entre módulos: um `domain` que importe outro
> módulo do workspace que por sua vez use I/O passa verde. O enforcement
> transitivo e a leitura da classificação autoritativa são do verificador do
> `KRN-02` (ARQ-521).

São três lacunas concretas, e cada uma tem um caminho de burla hoje aberto:

| Lacuna do gate atual | O que passa verde hoje |
| --- | --- |
| Seleciona por nome de diretório (`**/*-domain/**`), não pelo manifesto | Uma unidade `domain` declarada em `dmpf-units.json` cujo diretório não case o glob fica sem gate |
| Um módulo por vez | `domain` → módulo intermediário → `net/http` passa; a transitividade da pureza (RFC §6.3) não é avaliada |
| Nenhuma noção de `bounded_context` | C2 não é avaliada em lugar nenhum; a proibição de import entre contexts (RFC §5.5) é hoje só prosa |

Sem execução, a conformidade depende de revisão manual, e o inventário mostra
onde isso falha: o `goservice/domain` do `legado-golibs` declara `EventPublisher` — o
caso que o ADR-014 cita como unidade mal dimensionada — e
`goweb/domain/http_request.go:6` importa `net/http`, violando P0-1. Por isso o
verificador está no Incremento 1: sem ele, cada módulo de `KRN-03` em diante
nasceria sem gate, e a dívida do `legado-golibs` se reproduziria em terreno novo.

O terreno herdado do `KRN-01` é mínimo e conhecido: um `go.work` com `go 1.26.4`
declarando um único módulo (`libs/backend/go/domain`), com manifesto
`dmpf-units.json` válido de uma unidade, `project.json` com as três tags 3D e
cinco targets Go, e `tools/dmpf-gate-check.sh` provando o gate atual com seis
vetores negativos e um positivo por módulo.

<constraints>
- [P0] Fail-closed em toda ramificação: lacuna de configuração, arquivo não
  coberto, import não resolvido, manifesto ausente e caso degenerado REPROVAM.
  Condição que o verificador não consegue avaliar é reportada como "não
  verificado", NUNCA como "conforme" (RFC §3.6, §10.2, §11.3).
- [P0] A resolução do destino ocorre ANTES da classificação e sobre o destino
  REAL RESOLVIDO pelo toolchain — em Go, o package (RFC §3.3, §3.5) —, nunca
  sobre o texto do import. Alias e caminhos distintos para o mesmo package
  produzem a MESMA aresta.
- [P0] A aresta `domain → port` é proibida, sem condicional e sem exceção
  (RFC §7.3; ADR-014). Nenhuma flag da CLI pode relaxá-la.
- [P0] A classificação vem do manifesto declarado, NUNCA de convenção de
  diretório, de nome de pacote ou de heurística (ADR-012; RFC §4.4). Herança não
  existe: cada unidade declara os seus campos por completo (RFC §10.1).
- [P0] O conjunto de seis blocos é FECHADO. Valor fora dele não é ignorado nem
  tratado como não classificado: reprova com `DMPF-M002` (RFC §10.1, §4.1).
- [P0] O baseline é cópia INDEPENDENTE, mantida FORA do `ownership_module` que
  descreve (RFC §10.2, T1). Divergência reprova (T3); mudança normativa sem
  evidência de autorização reprova (T5).
- [P0] Nenhum artefato desta entrega declara nem sugere exactly-once fim a fim; a
  garantia é at-least-once com efeitos idempotentes (RFC §2.3, P0-3).
- [P0] Esta spec NÃO reabre decisão da fundação nem da guarda-chuva: divergência
  encontrada vira ADR novo ou escape hatch declarado, nunca alteração silenciosa
  da norma.
- [P1] O binário NÃO se chama `dmpf-verify`: o nome está ocupado por
  `tools/dmpf-verify.mjs` e pelo workflow `dmpf-verify.yml`, que verificam a
  congruência do acervo `docs/dmpf/` — outro gate, outro objeto.
- [P1] NUNCA redeclarar em `project.json` um target que o `@nx-go/nx-go` ou os
  `targetDefaults` já forneçam com o mesmo executor (ADR-002).
</constraints>

### Fontes normativas

| Artefato | Seções que vinculam esta entrega |
| --- | --- |
| `docs/dmpf/rfc-dmpf-foundation-v0.1.md` | §2.3 (P0), §3.3 (binding Go), §3.5 (resolução), §3.6 (degenerados), §4.1 (seis blocos), §4.5 (anti-bypass), §5.4-§5.5 (bounded context), §6 (capabilities), §7.1-§7.4 (regra e matriz), §10 (contrato do verificador), §11.1-§11.2 (vetores) |
| `docs/adr/010-regra-de-dependencia-e-seis-blocos.md` | A decisão é função, não célula |
| `docs/adr/011-verification-unit-binding-por-stack.md` | `verification_unit` = package; `canonical_key` = import path completo |
| `docs/adr/012-classificacao-por-metadado-declarado.md` | Metadado declarado, sem herança |
| `docs/adr/013-autoridade-sobre-classificacao.md` | Baseline T1-T3; autorização T4-T6; fail-closed |
| `docs/adr/014-proibir-aresta-domain-port.md` | `domain → port` proibida sem exceção |
| `docs/adr/015-capabilities-externas-por-bloco.md` | Default deny, allowlist por entrypoint, pureza transitiva |
| `docs/adr/017-bounded-context-declarado-superficie-publica.md` | `public_integration_surface: true` em `domain` é inválido |
| `docs/adr/028-processo-de-autorizacao-da-classificacao.md` | Delta efetivo como ato regulado; evidência persistida e endereçável; vigência amarrada a G2/G5 — até lá, mecanismo mínimo de RFC §10.2 |
| `docs/adr/030-granularidade-modulo-go-e-bom.md` | Granularidade de módulo Go decidida no `KRN-01` |

## Requisitos

### Funcionais

- [x] **[P0] Leitura e validação dos manifestos.** Descobrir todo
  `dmpf-units.json` versionado, validar contra o schema `dmpf/units@1` e reprovar
  antes de decidir qualquer aresta.
  - `block`, `bounded_context` e `include` são obrigatórios por unidade; ausência
    emite `DMPF-M001`.
  - `public_integration_surface` tem default `false` quando ausente.
  - `block` fora dos seis valores fechados emite `DMPF-M002`.
  - `public_integration_surface: true` em unidade de bloco `domain` emite
    `DMPF-M002` — a RFC §7.2 fecha essa porta explicitamente, e ADR-017 registra
    que sem a cláusula o campo seria o caminho trivial para burlar §5.5.
  - `id` duplicado no mesmo manifesto emite `DMPF-M003`.
  - Herança não é implementada: nenhum campo é derivado de diretório pai, de
    módulo ou de outra unidade.
- [x] **[P0] Construção do universo.** A `verification_unit` é o **package** Go e
  a `canonical_key` é o **import path completo** (ADR-011; RFC §3.3).
  - A descoberta de módulos usa um **inventário independente**, reconciliando
    três fontes: arquivos `go.mod` rastreados no repositório, projetos Nx com
    tag `stack:go` e membros do `go.work`. Módulo presente em qualquer fonte
    entra no universo de verificação — a omissão do `go.work` não o tira do
    alcance de `DMPF-U004`.
  - `include` casa por import path **exato**, não por glob de arquivo nem por
    prefixo de subárvore. Cada package é declarado individualmente, e a unidade
    só possui packages do próprio `ownership_module`. Casar por prefixo faria
    `x/domain/infra` herdar a classificação de `x/domain` pelo lugar do
    diretório — inferência por convenção, que ADR-012 e RFC §4.4 proíbem, e
    caminho para classificar package novo sem edição revisável do manifesto. O
    binding com glob é o de TypeScript (RFC §10.1); o de Go é import path. A
    única tolerância de forma é a barra final: `x/domain/` e `x/domain`
    designam o mesmo package.
  - Cada elemento de `include` é validado individualmente: valor que não seja
    import path (vazio, com barra inicial ou dupla, com segmento relativo, ou
    com glob) emite `DMPF-M002`. Contar `≥ 1` não substitui validar cada
    entrada.
  - Exclusões **fechadas** de RFC §10.3, e nenhuma outra: arquivos `_test.go`,
    diretórios `testdata/`, `vendor/` e arquivos sob build tag não usada em
    produção.
  - "Build tag não usada em produção" tem definição **operacional**: o conjunto
    de perfis de produção (`GOOS` × `GOARCH` × `CGO_ENABLED` × tags) é dado
    versionado do verificador. A extração roda por perfil e o grafo é a união
    das arestas; arquivo sob tag fora de todos os perfis está excluído. O
    conjunto inicial tem um perfil — `linux/amd64`, `CGO_ENABLED=0`, sem tags:
    o que o CI executa hoje, agora declarado. Adicionar alvo de produção é
    editar o dado, não o código.
  - Código **gerado NÃO é excluído** — participa do grafo pelo que importa em
    runtime (RFC §4.5, regra 2).
  - Arquivo de produção não coberto por nenhuma unidade emite `DMPF-U001`;
    coberto por mais de uma, `DMPF-U002`; `canonical_key` duplicada no universo,
    `DMPF-U003`; módulo de produção com código e sem manifesto, `DMPF-U004`.
- [x] **[P0] Resolução de arestas sobre o package resolvido.** Extrair o grafo
  com `go list -e -deps -json` sobre os módulos do inventário. A aresta
  canônica em Go é **package → package** (RFC §3.3): as arestas diretas vêm de
  `Imports`/`ImportMap` — a resolução do toolchain, nunca o texto do import
  (RFC §3.5); `Deps` é fechamento transitivo e serve apenas à avaliação de
  pureza (`DMPF-E002`). O `go/parser` entra só para nomear o arquivo de origem
  na mensagem do diagnóstico, nunca para decidir.
  - Import estático que resolve para o universo é aresta entre unidades.
  - Import estático que resolve fora do universo não é aresta: é capability (§6).
  - O `-e` é transporte estruturado de erro, não aceitação de universo parcial:
    package com `Incomplete: true` ou `Error` preenchido emite `DMPF-E003` e
    reprova — a política fail-closed é do consumidor, e o import nunca é
    tratado como ausente.
- [x] **[P0] A função `decide`.** Implementar
  `decide(source_block, target_block, source_bc, target_bc, target_surface)`
  como conjunção de C1 e C2, com a matriz 6×6 da RFC §7.3 como **dado
  versionado**, revisável em PR, e não como `switch` espalhado no código.
  - C1 falsa emite `DMPF-D001`; C2 falsa emite `DMPF-D002`.
  - As duas são avaliadas de forma independente: uma aresta pode emitir ambas.
  - `P` na matriz não é autorização final — a aresta permitida continua sujeita a
    C2 e à política de capabilities.
- [x] **[P0] Capabilities externas.** Todo import fora do universo é avaliado
  contra a política do bloco de origem (RFC §6.2) e a allowlist do manifesto
  (§6.3).
  - Blocos `domain` e `port` aceitam apenas `pure`; `application` é default deny
    com exceção nominal; `contract` aceita `pure` e `wire.codec`; `provider` e
    `app` são permissivos.
  - Builtins recebem capability como qualquer outra dependência: `net/http` é
    `io.network`; `crypto` é `pure`.
  - `observability` é vedada em `domain` e em `port` ainda que a biblioteca seja
    tecnicamente pura (RFC §6.2, princípio 11).
  - Capability não permitida para o bloco emite `DMPF-E001`; entrada declarada
    `pure` cujo fechamento transitivo introduz capability diferente emite
    `DMPF-E002`.
  - A pureza é computada a partir dos **entrypoints declarados**, na faixa de
    versões declarada — não do pacote inteiro.
- [x] **[P0] Baseline e autorização.** Manter o baseline como cópia independente
  fora do `ownership_module` que descreve (ADR-013, T1).
  - Cobre `block` e `bounded_context` de cada `canonical_key`, mais o
    **membership resolvido** (os packages que o `include` de cada unidade
    captura) e um digest do conjunto (T2). O membership existe porque o ato
    regulado é o **delta efetivo** `arquivo → (canonical_key, block,
    bounded_context)` (ADR-028): remapear `include`, root ou caminho muda a
    classificação efetiva sem tocar em campo algum, e um baseline só de chaves
    não veria o remapeamento.
  - Divergência entre manifesto e baseline emite `DMPF-T001` (T3).
  - Alteração de `block` ou `bounded_context` de unidade existente, criação de
    unidade nova, remoção de unidade e **remapeamento que altere o membership**
    são mudança normativa (T4, T6; ADR-028): sem evidência de autorização
    distinta da autoria, emite `DMPF-T002` (T5).
  - O processo de autorização **já está definido**: o ADR-028 (aceito,
    2026-08-29) institui a Autoridade de Classificação Arquitetural, o rito e a
    evidência persistida e endereçável no repositório. A vigência, porém, está
    amarrada ao aceite de um titular (G2) e ao fechamento da ANC-08 (G5); até
    lá vige o mecanismo mínimo de RFC §10.2 — mudança normativa em commit
    próprio, separado de mudanças de código — e as duas normas nunca se
    aplicam ao mesmo tempo. O verificador que não consiga avaliar a condição
    reporta "não verificado", nunca "conforme" — e "não verificado" reprova.
- [x] **[P0] Diagnósticos estáveis.** Emitir os **quinze** códigos de RFC §10.3,
  cada um rastreável até a seção que o institui, com saída diferente de zero em
  toda reprovação.
  - `DMPF-E004` (import dinâmico com alvo não determinável) é **declarado sem
    ocorrência possível no binding Go**: a gramática da linguagem exige
    `ImportPath = string_lit`, e import dinâmico não existe em Go. O código
    permanece reservado e estável (os quinze não mudam); carregamento dinâmico
    via API (`plugin.Open` e afins) já é capturado pela política de
    capabilities (`DMPF-E001`/`DMPF-E002`). O vetor da classe vira teste de
    não-emissão documentado; `E004` volta a ter vetor executável no binding TS
    (`KRN-11`).
- [x] **[P0] Suite de vetores.** Cobertura por classe, não por contagem
  (RFC §11.1): cada uma das 36 células com vetor positivo e negativo, e um vetor
  por classe de diagnóstico. A suite falha se um negativo passar.
- [x] **[P0] Gate de CI.** Ligar o verificador ao `ci.yml` sem
  `continue-on-error`, no mesmo ponto em que hoje roda `tools/dmpf-gate-check.sh`.
- [x] **[P0] O verificador passa no próprio gate.** O módulo declara o próprio
  `dmpf-units.json` e é verificado por ele mesmo.
- [x] **[P1] Guia de escrita do manifesto**, para que `KRN-03`..`KRN-12` declarem
  as suas unidades sem reabrir a RFC.
- [x] **[P1] ADR** registrando a decisão, as alternativas descartadas e as
  limitações declaradas.

### Não-funcionais

- [x] **[P0] Determinismo.** Mesma árvore, mesmo veredicto e mesma ordem de
  diagnósticos. A saída é ordenada por `canonical_key` e por código.
- [x] **[P0] Zero dependência externa impura no núcleo de decisão.** A unidade
  que implementa `decide`, a matriz e a validação de schema é `pure` — ela mesma
  reprovaria se importasse I/O.
- [x] **[P1] Mensagem acionável.** Cada diagnóstico nomeia a aresta
  (`origem → destino`), o arquivo de origem que a introduz (nomeado via
  `go/parser`, apenas para a mensagem — nunca para decidir), o código e a
  seção normativa.
- [x] **[P1] Tempo de execução compatível com CI** no workspace atual, sem
  paralelismo especulativo; o custo dominante é o `go list`.

## Camadas afetadas

| Camada | Impacto |
| --- | --- |
| **Workspace Go** | `go.work` ganha o segundo módulo |
| **Nx** | Novo projeto `conformance` com tags 3D e targets Go; inputs do gate cobrem `tools/dmpf-baseline/**`, para que mudança só no baseline dispare a verificação |
| **CI** | `ci.yml` ganha o passo do verificador; `tools/dmpf-gate-check.sh` passa a conviver com ele |
| **Lint** | `.golangci.yml` mantém a regra `depguard` como defesa local rápida; a limitação declarada no comentário deixa de valer para a aresta entre módulos |
| **Governança** | Baseline versionado passa a ser artefato de revisão obrigatória em PR |
| **Documentação** | ADR novo e guia de escrita do manifesto |

Nenhuma camada de produto é afetada: não há app, endpoint, banco ou fila nesta
entrega.

## Localização de código

```text
dmpf/
├── libs/backend/go/conformance/          # CRIAR — módulo Go, projeto conformance
│   ├── go.mod                                 # module .../libs/backend/go/conformance
│   ├── package.json                           # private: true (Nx Release exige manifesto npm)
│   ├── project.json                           # tags 3D + targets Go espelhando domain
│   ├── dmpf-units.json                        # o verificador declarado para si mesmo
│   ├── build-profiles.json                    # perfis de produção — dado versionado (GOOS × GOARCH × CGO_ENABLED × tags)
│   ├── cmd/conformance/
│   │   └── main.go                            # bloco `app` — composition root
│   └── internal/
│       ├── rule/                              # bloco `domain` — decide(), C1, C2, diagnósticos
│       │   ├── matrix.go                      # matriz 6×6 como dado versionado
│       │   ├── decide.go
│       │   ├── diagnostic.go                  # os 15 códigos
│       │   └── *_test.go                      # vetores por célula e por classe
│       ├── manifest/                          # bloco `domain` — schema dmpf/units@1 sobre modelo puro
│       │   ├── schema.go
│       │   └── validate.go
│       ├── baseline/                          # bloco `domain` — digest e comparação (T1-T3)
│       ├── port/                              # bloco `port` — GraphSource, ManifestSource, BaselineStore
│       ├── golist/                            # bloco `provider` — go list -e -deps -json, por perfil
│       └── fsstore/                           # bloco `provider` — leitura e decodificação JSON (wire.codec) de manifesto e baseline
├── tools/dmpf-baseline/
│   └── units-baseline.json                    # CRIAR — cópia independente, FORA do módulo (T1)
├── tools/dmpf-verify.mjs                      # INTOCADO — outro gate (acervo docs/dmpf/)
├── tools/dmpf-gate-check.sh                   # INTOCADO nesta entrega
├── .github/workflows/ci.yml                   # MODIFICAR — passo do verificador
├── nx.json                                    # MODIFICAR — inputs: mudança no baseline dispara o gate
├── .github/workflows/dmpf-verify.yml          # INTOCADO — outro gate
├── go.work                                    # MODIFICAR — use ./libs/backend/go/conformance
├── docs/adr/031-verificador-de-conformidade-dmpf-em-go.md   # CRIAR
└── docs/dmpf/                                 # INTOCADO — acervo normativo
```

Import path canônico do módulo:
`github.com/mateusmacedo/dmpf/libs/backend/go/conformance`

## Design

### Arquitetura

O verificador é implementado **na própria arquitetura que verifica**. Isso não é
elegância: é o que torna o DoD "o módulo passa no seu próprio gate" um teste real
em vez de um carimbo. Um verificador monolítico passaria trivialmente no próprio
gate por ter uma unidade só; decomposto em `domain`, `port`, `provider` e `app`
dentro de um mesmo `bounded_context`, ele exercita C1 de verdade — e a aresta
`domain → port`, que o ADR-014 proíbe, torna-se um erro que o próprio build pega.

```text
cmd/conformance  [app]         composition root: monta providers, roda, imprime
        │
        ├──────────────┬────────────────┐
        ▼              ▼                ▼
 internal/golist  internal/fsstore   internal/rule   [domain]
   [provider]       [provider]       internal/manifest
        │              │             internal/baseline
        └──────┬───────┘                    ▲
               ▼                            │
        internal/port  [port] ──────────────┘
```

Arestas exercitadas, todas `P` na matriz: `app → *`, `provider → port`,
`provider → domain`, `port → domain`, `domain → domain`. A aresta que o desenho
torna impossível é `domain → port`: nenhuma unidade de `rule`, `manifest` ou
`baseline` importa `port`, e o gate reprova se alguém tentar.

Todas as unidades declaram `bounded_context: conformance`, de modo que C2
passa internamente e o módulo não depende de `public_integration_surface` para
compilar.

### Fluxo principal — de `go.work` a veredicto

1. **Descobrir** os módulos pelo inventário independente — `go.mod` rastreados,
   projetos Nx `stack:go` e `go.work`, reconciliados — e todo `dmpf-units.json`
   versionado.
2. **Validar** cada manifesto contra `dmpf/units@1`. Qualquer `DMPF-M*` ou
   `DMPF-U004` aqui **encerra**: sem classificação válida não há decisão possível,
   e prosseguir seria adivinhar.
3. **Construir o universo**: mapear cada package de produção à sua unidade,
   aplicando as exclusões fechadas. Package descoberto e não coberto emite
   `DMPF-U001`; coberto duas vezes, `DMPF-U002`.
4. **Extrair o grafo** com `go list -e -deps -json`, por perfil de produção,
   tomando as arestas diretas de `Imports`/`ImportMap` — a resolução do
   toolchain, nunca o texto do import — e unindo os perfis.
5. **Particionar** as arestas: destino no universo vai para `decide`; destino fora
   vai para a política de capabilities.
6. **Decidir** cada aresta interna com C1 e C2; avaliar cada dependência externa
   contra a política do bloco e o fechamento transitivo da allowlist.
7. **Conferir o baseline**: comparar `block`, `bounded_context`, o membership
   resolvido e o digest; onde houver mudança normativa, exigir evidência de
   autorização.
8. **Reportar** os diagnósticos ordenados e sair com código diferente de zero se
   houver qualquer um.

O passo 2 encerrar antes do passo 3 é deliberado: um manifesto inválido produz um
universo inválido, e diagnósticos de aresta calculados sobre universo inválido
seriam ruído que esconde a causa real.

### Fluxo de decisão — a conjunção

```text
para cada aresta (package_origem -> package_destino) do grafo:   // Imports/ImportMap, por perfil
    unidade_origem = unidade_que_contem(package_origem)           // senão: DMPF-U001, reprova
    se package_destino não resolveu:                              // Incomplete/Error do go list -e
        emite DMPF-E003                                           // reprova, nunca "ausente"
        continua
    se package_destino não pertence ao universo:
        avalia_capability_externa(unidade_origem, package_destino)  // política do bloco (§6)
        continua
    unidade_destino = unidade_que_contem(package_destino)         // senão: DMPF-U001, reprova

    C1 = matriz[unidade_origem.block][unidade_destino.block] == PERMITIDA
    C2 = unidade_origem.bounded_context == unidade_destino.bounded_context
         ou unidade_destino.block == contract          // contract é superfície por construção
         ou unidade_destino.public_integration_surface

    se não C1: emite DMPF-D001
    se não C2: emite DMPF-D002
```

O default de toda ramificação ausente é **reprovar**. Um verificador que não
consiga avaliar uma condição reporta "não verificado", nunca "conforme".

### A matriz como dado versionado

A matriz 6×6 da RFC §7.3, transcrita literalmente e mantida em arquivo revisável:

| De ↓ / Para → | domain | application | app | port | provider | contract |
| --------------- | :------: | :-----------: | :---: | :----: | :--------: | :--------: |
| **domain** | P | ✗ | ✗ | ✗ | ✗ | ✗ |
| **application** | P | P | ✗ | P | ✗ | ✗ |
| **app** | P | P | P | P | P | P |
| **port** | P | ✗ | ✗ | P | ✗ | ✗ |
| **provider** | P | ✗ | ✗ | P | P | P |
| **contract** | ✗ | ✗ | ✗ | ✗ | ✗ | P |

Vinte das 36 células não têm evidência no universo inventariado (ADR-010) — elas
se apoiam apenas na fonte normativa. Os vetores negativos dessas células são
derivados de RFC §7.4, célula a célula, e não de código observado.

## Decisões técnicas

| Decisão | Alternativas descartadas |
| --- | --- |
| **Nome `conformance`** | `dmpf-verify` está ocupado por `tools/dmpf-verify.mjs` e pelo workflow `dmpf-verify.yml`, que verificam a congruência do acervo `docs/dmpf/`; reusar confundiria dois gates com objetos distintos. `dmpf-depcheck` foi descartado por sugerir escopo só de dependência, quando a entrega inclui manifesto, baseline e capabilities. |
| **Local `libs/backend/go/conformance`** | `tools/go/` agruparia os dois gates, mas contraria o caminho por scope/stack que o `AGENTS.md` fixou e que o `domain` inaugurou. |
| **Tag `type:lib`** | `type:app` seria semanticamente mais próximo de um CLI, mas o módulo é majoritariamente biblioteca (regra, manifesto, baseline) com `cmd/` fino, e `type:lib` preserva o comportamento já provado do `domain` no `nx-release.yml`: versionado por Conventional Commits, excluído da publicação por `private: true` e pelo filtro `!tag:stack:go`. |
| **Grafo via `go list -e -deps -json`** | Parsear os arquivos com `go/parser` daria o texto do import, não a resolução — exatamente o que RFC §3.5 proíbe como base da decisão. `go list` entrega a resolução do toolchain: `Imports`/`ImportMap` para arestas diretas, `Deps` só para o fechamento transitivo de pureza. Sem `-e`, package errôneo vai para stderr fora do JSON e `DMPF-E003` não é emissível de forma estável; o `-e` é transporte estruturado de erro — o fail-closed é do consumidor. |
| **Inventário independente de módulos** | Descoberta só via `go.work` deixaria módulo omitido (com `go.mod`, código de produção e projeto Nx `stack:go`) fora do universo — ele nunca geraria o `DMPF-U004` que existe para detectá-lo. |
| **Perfis de produção como dado versionado** | Perfil implícito do runner mantém "produção" indefinida — trocar o runner mudaria o veredicto em silêncio. União de todos os `GOOS`/`GOARCH` do toolchain decidiria arestas de plataformas nunca compiladas (falsos vermelhos) e multiplicaria o custo do `go list`. |
| **Decodificação JSON no provider** | `internal/manifest` como `domain` validando bytes exigiria `encoding/json`, que é `wire.codec` — vedado em `domain` (RFC §6.2; `.golangci.yml`): ao ativar `DMPF-E001`, o verificador reprovaria a si mesmo. O provider decodifica; o domínio valida modelo puro. |
| **Baseline com membership resolvido** | Baseline só de chave → (`block`, `bounded_context`) + digest não detecta remapeamento por `include` — o delta efetivo que o ADR-028 regula; o rito seria evitável por construção. |
| **`DMPF-E004` não aplicável ao binding Go** | Definir APIs de carregamento dinâmico (`plugin.Open`, `go:linkname`) como "import dinâmico" ampliaria a semântica de RFC §10.3 por decisão local; a gramática exige `ImportPath = string_lit`, e essas APIs já caem na política de capabilities. |
| **Verificador decomposto em `domain`/`port`/`provider`/`app`** | Um pacote único passaria no próprio gate trivialmente. A decomposição torna o DoD um teste real e dá ao repositório o primeiro exemplo executável da arquitetura. |
| **Baseline em `tools/dmpf-baseline/`** | Dentro do módulo violaria T1 (cópia independente, fora do `ownership_module`). Em `docs/dmpf/` misturaria dado operacional com acervo normativo em prosa, que tem gate próprio. |
| **Manifesto validado antes de qualquer aresta** | Decidir arestas com manifesto inválido produziria diagnósticos derivados de classificação que não vale, escondendo a causa real. |
| **`.golangci.yml` mantido** | Remover a regra `depguard` deixaria o desenvolvedor sem sinal local rápido. Ela vira defesa em profundidade; a limitação que o comentário declara passa a ser coberta por este verificador. |
| **ADR-031** | O ticket diz "a partir de `029`", mas `029` (titular da revisão FND-07) e `030` (granularidade de módulo Go) já existem. `031` é o próximo livre. |

## Regras relacionadas

- `AGENTS.md` — tags 3D obrigatórias; caminho `libs/<scope>/<stack>/<módulo>`;
  nome de projeto Nx com sufixo de stack; Conventional Commits em PT-BR.
- `docs/adr/002-nx-task-configuration.md` — não redeclarar target já fornecido.
- `.claude/rules/process-enforcement.md` — cadeia de validação Go: `gofmt`,
  `go vet`, `golangci-lint`, `go build`, `go test`, `go test -race`,
  `govulncheck`.
- `.claude/rules/git-safety.md` — `master`, `develop` e `release/**` protegidas.

## Verificação e testes

### Critérios de aceite

- [x] Um package `domain` que importa um package `port` reprova com `DMPF-D001`,
      sem exceção e sem flag que relaxe (ADR-014).
- [x] Dois packages `domain` em bounded contexts diferentes, um importando o
      outro, reprovam com `DMPF-D002`.
- [x] Um package de produção fora de todo `include` reprova com `DMPF-U001`; um
      módulo de produção sem manifesto reprova com `DMPF-U004`; um `block` fora
      dos seis valores reprova com `DMPF-M002`.
- [x] `public_integration_surface: true` em unidade de bloco `domain` reprova com
      `DMPF-M002` (RFC §7.2; ADR-017).
- [x] Uma unidade `domain` cujo fechamento de imports alcança `net/http` reprova —
      por `DMPF-D001` se via unidade do universo, por `DMPF-E001` se via
      dependência externa —, **inclusive quando o alcance é transitivo por outro
      módulo do `go.work`**, que é precisamente o que o gate do `KRN-01` não vê.
- [x] Uma entrada da allowlist declarada `pure` cujo fechamento transitivo, a
      partir dos entrypoints declarados, introduz capability diferente reprova com
      `DMPF-E002`.
- [x] Cada uma das 36 células tem vetor positivo e negativo, e a suite falha se um
      negativo passar.
- [x] Alterar o `block` sem atualizar o baseline reprova com `DMPF-T001`; alterar
      os dois no mesmo commit, sem autorização distinta da autoria, reprova com
      `DMPF-T002`.
- [x] Import não resolvido reprova com `DMPF-E003`; `DMPF-E004` é declarado sem
      ocorrência possível no binding Go (gramática: `ImportPath = string_lit`),
      com código reservado e teste de não-emissão; nenhuma condição não avaliada
      é reportada como conforme.
- [x] Remapear `include` movendo packages entre unidades, sem tocar nenhum campo,
      é mudança normativa detectada pelo membership do baseline: sem evidência de
      autorização, reprova com `DMPF-T002` (ADR-028).
- [x] Um módulo `stack:go` com código de produção, omitido do `go.work` e sem
      manifesto, reprova com `DMPF-U004` — o inventário independente o alcança.
- [x] Um PR que altere apenas `tools/dmpf-baseline/units-baseline.json` dispara o
      gate do verificador (inputs Nx cobrem o baseline).
- [x] O mesmo arquivo importado por caminhos diferentes produz **uma** aresta, e o
      veredicto não muda com a forma do import.
- [x] Um arquivo `_test.go` que importa `net/http` dentro de uma unidade `domain`
      **não** reprova — a exclusão é fechada e teste não altera classificação
      (RFC §4.5, regra 3). O vetor existe, mas **não é falseável por mutação do
      verificador**: o `go list` separa os arquivos de teste em `TestGoFiles` e
      devolve `Imports` vazio, então o import nunca chega ao código. Ele guarda
      contra uma mudança futura que passasse a ler `TestGoFiles` no grafo.
- [x] Código gerado consumido em runtime **reprova** pelas regras do bloco que o
      consome (RFC §4.5, regra 2).
- [x] O módulo `conformance` é verificado por si mesmo e passa.
- [x] `pnpm biome ci .` e
      `pnpm nx affected -t lint,typecheck,test,build --exclude=@mateusmacedo/dmpf-source`
      passam.
- [x] `pnpm nx run conformance:fmt-check,vet,test-race,govulncheck` passam.

### Cenários de teste

1. **C1 reprova — `domain → port`.** Universo com `pedidos/domain` e
   `pedidos/port` no mesmo contexto; `domain` importa `port`.
   → `DMPF-D001`, saída ≠ 0. *É o caso que o ADR-014 nomeia como fronteira.*
2. **C1 aprova — `application → port`.** Mesmo universo; `application` importa
   `port`. → sem diagnóstico, saída 0.
3. **C2 reprova — contexts distintos.** `pedidos/domain` importa
   `faturamento/domain`, ambos com `public_integration_surface: false`.
   → `DMPF-D002`.
4. **C2 aprova — superfície pública.** Mesmo par, com o destino sendo `contract`
   e `public_integration_surface: true`. → sem diagnóstico.
5. **Transitividade entre módulos.** `mod-a/domain` importa `mod-b/util`
   (`domain`), que importa `net/http`. O gate do `KRN-01` passa verde; este
   reprova. → `DMPF-E001` na unidade `mod-b/util`. *Este cenário é a razão de ser
   da história.*
6. **Manifesto inválido encerra cedo.** `block: "core"` em uma unidade.
   → `DMPF-M002` e **nenhum** diagnóstico de aresta emitido.
7. **Superfície pública em `domain`.** `public_integration_surface: true` em
   unidade `domain`. → `DMPF-M002`.
8. **Cobertura do universo.** Package de produção fora de todo `include`.
   → `DMPF-U001`. Módulo com `.go` de produção e sem `dmpf-units.json`
   → `DMPF-U004`.
9. **Identidade da aresta.** O mesmo package importado por alias e por caminho
   completo produz uma aresta e um único diagnóstico.
10. **Exclusões são fechadas.** `_test.go` importando `net/http` em unidade
    `domain` não reprova; o mesmo import em arquivo de produção reprova.
11. **Código gerado não é isento.** Arquivo com marcador de geração importando
    `net/http` em unidade `domain` reprova.
12. **Baseline — divergência acidental.** `block` alterado no manifesto e não no
    baseline. → `DMPF-T001`.
13. **Baseline — mudança normativa sem autorização.** `block` alterado nos dois no
    mesmo commit, sem evidência de autorização distinta da autoria.
    → `DMPF-T002`, e a mensagem diz "não verificado", não "conforme".
14. **Import não resolvido.** Import para package inexistente. → `DMPF-E003`.
15. **Pureza transitiva.** Entrada `pure` na allowlist cujo fechamento, a partir
    do entrypoint declarado, alcança capability `io.*`. → `DMPF-E002`.
16. **Autoverificação.** Rodar `conformance` sobre o próprio módulo.
    → saída 0, e a suite falha se alguém introduzir `domain → port` nele.
17. **Módulo omitido do `go.work`.** Módulo com `go.mod`, código de produção e
    projeto Nx `stack:go`, ausente do `go.work` e sem manifesto. O inventário
    independente o alcança. → `DMPF-U004` — a omissão não o esconde.
18. **Baseline — remapeamento por `include`.** `include` reorganizado movendo
    packages entre unidades, sem tocar `block` nem `bounded_context`. O
    membership do baseline diverge. → mudança normativa; sem evidência,
    `DMPF-T002` (ADR-028).
19. **Perfil de produção.** Aresta presente apenas em arquivo `_windows.go`, com
    `windows/amd64` fora do conjunto de perfis. → excluída, sem diagnóstico;
    incluir o perfil no dado versionado passa a decidi-la.

<critical_constraints>

- [P0] Fail-closed em toda ramificação: lacuna de configuração, arquivo não
  coberto, import não resolvido, manifesto ausente e caso degenerado REPROVAM.
  Condição que o verificador não consegue avaliar é reportada como "não
  verificado", NUNCA como "conforme" (RFC §3.6, §10.2, §11.3).
- [P0] A resolução do destino ocorre ANTES da classificação e sobre o destino
  REAL RESOLVIDO pelo toolchain — em Go, o package (RFC §3.3, §3.5) —, nunca
  sobre o texto do import. Alias e caminhos distintos para o mesmo package
  produzem a MESMA aresta.
- [P0] A aresta `domain → port` é proibida, sem condicional e sem exceção
  (RFC §7.3; ADR-014). Nenhuma flag da CLI pode relaxá-la.
- [P0] A classificação vem do manifesto declarado, NUNCA de convenção de
  diretório, de nome de pacote ou de heurística (ADR-012; RFC §4.4). Herança não
  existe: cada unidade declara os seus campos por completo (RFC §10.1).
- [P0] O conjunto de seis blocos é FECHADO. Valor fora dele não é ignorado nem
  tratado como não classificado: reprova com `DMPF-M002` (RFC §10.1, §4.1).
- [P0] O baseline é cópia INDEPENDENTE, mantida FORA do `ownership_module` que
  descreve (RFC §10.2, T1). Divergência reprova (T3); mudança normativa sem
  evidência de autorização reprova (T5).
- [P0] Nenhum artefato desta entrega declara nem sugere exactly-once fim a fim; a
  garantia é at-least-once com efeitos idempotentes (RFC §2.3, P0-3).
- [P0] Esta spec NÃO reabre decisão da fundação nem da guarda-chuva: divergência
  encontrada vira ADR novo ou escape hatch declarado, nunca alteração silenciosa
  da norma.
- [P1] O binário NÃO se chama `dmpf-verify`: o nome está ocupado por
  `tools/dmpf-verify.mjs` e pelo workflow `dmpf-verify.yml`, que verificam a
  congruência do acervo `docs/dmpf/` — outro gate, outro objeto.
- [P1] NUNCA redeclarar em `project.json` um target que o `@nx-go/nx-go` ou os
  `targetDefaults` já forneçam com o mesmo executor (ADR-002).
</critical_constraints>

## Escopo fora

- **Verificador TypeScript.** O binding TS (RFC §3.4) e o pareamento Go ↔ TS dos
  vetores (§11.4) pertencem a `KRN-11`.
- **Preencher o manifesto de `KRN-03`..`KRN-12`.** Cada história declara as
  próprias unidades; aqui entra apenas o guia de escrita.
- **Nomear o titular da Autoridade de Classificação Arquitetural.** O processo
  já está definido pelo ADR-028 (função, rito e evidência); a nomeação do
  titular (G2), a revisão por Segurança e o fechamento da ANC-08 (G5) pertencem
  ao FND-10 (ARQ-447). Até a
  vigência, este verificador aplica o mecanismo mínimo de RFC §10.2.
- **Provar que a classificação corresponde à responsabilidade real do código.**
  Limitação declarada em RFC §10.4, item 1: o verificador prova compatibilidade
  entre imports e declaração, não adequação semântica.
- **Provar que `app` instancia provider concreto apenas no composition root.**
  Limitação declarada em RFC §10.4, item 2; a cláusula é
  `structurally reviewable`.
- **Qualquer propriedade de runtime**, incluindo a metade dinâmica de RFC §9 e o
  efeito idempotente de P0-3 (RFC §10.4, item 3).
- **Remover a regra `depguard` do `.golangci.yml`.** Ela permanece como defesa
  local rápida.
- **Alterar `tools/dmpf-verify.mjs` ou o workflow `dmpf-verify.yml`.** Outro gate,
  outro objeto.
- **Kernel de domínio, UoW, contratos wire, outbox, inbox, relay, resiliência,
  transporte e SDK** — `KRN-03` em diante.
