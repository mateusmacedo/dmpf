---
id: SPEC-MQA5HAXF
slug: dmpf-fundacao-nx-go
title: DMPF KRN-01 — Fundação Nx-Go: workspace, módulos, tags e cadeia de validação
stage: planning
priority: P0
depends_on: []
ticket_url: https://lider-cap.atlassian.net/browse/ARQ-520
subtask_urls: []
created: 2026-08-31
---

# SPEC-MQA5HAXF: DMPF KRN-01 — Fundação Nx-Go: workspace, módulos, tags e cadeia de validação

## Resumo

Estabelecer o terreno Go deste monorepo Nx — primeiro módulo, convenção de
import path, tags 3D, cadeia de validação de sete passos, gates de CI e ganchos
locais — para que todo módulo criado a partir de `KRN-02` nasça já verificado.
Como time de plataforma, queremos um workspace Go em que a regra de dependência
e a conformidade DMPF sejam exercidas por máquina, e não por revisão manual,
para que o kernel Go do épico [ARQ-519](https://lider-cap.atlassian.net/browse/ARQ-519)
não repita as violações que o `golibs` acumulou.

Esta é a primeira metade do Incremento 1 da spec guarda-chuva
[SPEC-YRJRADY9](./SPEC-YRJRADY9-dmpf-kernel-sdk-go.md): "Módulo Go compila,
valida e é verificado pela regra de dependência no CI".

## Contexto

- **Problema**: o repositório não tem terreno Go. O `go.work` declara apenas
  `go 1.25`, sem bloco `use`; não existe nenhum `go.mod` nem nenhum arquivo
  `.go` fora de `node_modules`; `libs/backend/` contém somente um `.gitkeep` de
  0 bytes. O `ci.yml` não tem nenhum passo de toolchain Go, e o `lefthook.yml`
  não cobre `*.go` em nenhum gancho. Sem isso, cada módulo do kernel nasceria
  sem gate, e a conformidade voltaria a depender de revisão humana — exatamente
  o modo de falha que o `golibs` demonstra, com `EventPublisher` em
  `goservice/domain` e `net/http` em `goweb/domain`.

- **Impacto**: a partir desta entrega, um módulo Go criado no workspace é
  descoberto pelo Nx, participa de `nx affected`, roda os sete passos de
  validação e reprova o PR quando qualquer um deles falha. O desenvolvedor
  deixa de precisar saber a cadeia de cor: `pnpm nx` a executa.

- **Inspiração**: o repositório `golibs` (`gitea.lidercap.com.br/lidercap-apps/golibs`)
  é referência de **forma** — 15 módulos, um `go.mod` por package, `go.work`
  com bloco `use` de 15 entradas — e não base a herdar: ele não pina nenhuma
  ferramenta Go, não usa gancho local para Go, redeclara targets que o plugin
  já infere e adota uma taxonomia de tags incompatível com a deste workspace
  (`runtime:go` + duas tags `scope:`).

- **Links relevantes**:
  - [SPEC-YRJRADY9](./SPEC-YRJRADY9-dmpf-kernel-sdk-go.md) — spec guarda-chuva do
    épico ARQ-519; fixa os blocos, a árvore de módulos e as decisões que esta
    sub-spec executa sem reabrir
  - [SPEC-QG2N8STY](./SPEC-QG2N8STY-dmpf-foundation.md) — fundação normativa
    (ARQ-436): RFC e ADRs 010–028 que esta entrega realiza em código
  - `docs/dmpf/rfc-dmpf-foundation-v0.1.md` §3.1, §3.3, §3.6, §10.1 —
    `verification_unit`, `canonical_key`, `ownership_module`, `metadata_container`
  - `docs/adr/002-nx-task-configuration.md` — vedação a redeclarar target inferido
  - `docs/adr/011-verification-unit-binding-por-stack.md` — em Go, a
    `verification_unit` é o package e a `canonical_key` é o import path completo
  - `docs/adr/012-classificacao-por-metadado-declarado.md` — classificação por
    metadado declarado, sem inferência
  - `docs/adr/027-bom-combinacao-certificada-compatibilidade-e-escape-hatch.md` —
    modelo de BOM que o BOM do Go instancia
  - `docs/nx-reference/tasks.md` — guia de tasks Nx, hoje sem receita para Go
  - `AGENTS.md` — taxonomia canônica de tags 3D e inventário de libs

<constraints>
- [P0] Nenhum artefato desta entrega declara nem sugere exactly-once fim a fim; a
  garantia é at-least-once com efeitos idempotentes (RFC §2.3, P0-3).
- [P0] Fail-closed: lacuna de configuração, classificação ausente, manifesto
  ausente e caso degenerado REPROVAM; nunca são lidos como conformidade
  (RFC §3.6).
- [P0] NUNCA redeclarar em `project.json` um target que o `@nx-go/nx-go` ou os
  `targetDefaults` já forneçam com o mesmo executor — isso sobrescreve cache e
  inputs em silêncio (ADR-002).
- [P0] A aresta `domain → port` é proibida, sem condicional e sem exceção
  (RFC §7; ADR-014).
- [P0] Esta spec NÃO reabre decisão da fundação nem da guarda-chuva: divergência
  encontrada vira ADR novo ou escape hatch declarado, nunca alteração silenciosa
  da norma.
- [P0] Todo módulo Go de produção DEVE nascer com o `metadata_container`
  `dmpf-units.json` (schema `dmpf/units@1`) na raiz do seu `ownership_module`,
  ainda que o verificador só chegue em `KRN-02` — manifesto ausente reprova
  (RFC §3.6; ADR-012).
- [P1] Todo módulo de produção carrega exatamente três tags 3D, uma por dimensão
  (`AGENTS.md`); para o primeiro módulo: `type:lib`, `scope:backend`, `stack:go`.
</constraints>

## Requisitos

### Funcionais

- [ ] **[P0] Granularidade de módulo decidida em ADR**: registrar em
  `docs/adr/030-granularidade-modulo-go-e-bom.md` a adoção de **um `go.mod` por
  lib Nx**, com a alternativa descartada (módulo único via
  `@nx-go/nx-go:convert-to-one-mod`) e o motivo.
  - Motivo da escolha: o `ownership_module` é o diretório com `go.mod` e é ele
    que porta o `metadata_container` (RFC §3.6); módulo único daria um
    manifesto só para todo o kernel, enfraquecendo a fronteira de ownership que
    o DMPF usa para classificar.
  - O ADR registra também que a decisão é de mão única: migrar depois de
    `KRN-03` reescreveria todos os import paths, ou seja, a `canonical_key` de
    toda unidade, invalidando manifesto e baseline.

- [ ] **[P0] Piso e toolchain do Go fixados como BOM**: `go.work` passa a
  declarar `go 1.26.4`; o BOM certifica `go1.26.4` como toolchain; o CI fixa
  essa mesma versão.
  - Piso e toolchain coincidem: esta entrega não mantém folga N-1.
  - Consequência declarada: quem tiver `go1.26.0`–`go1.26.3` baixa o toolchain
    automaticamente por `GOTOOLCHAIN=auto`. Isso é comportamento esperado, não
    falha.
  - Divergência com o `golibs` (`go 1.25.0`) é aceita e registrada: um módulo de
    piso maior consome um de piso menor sem impedimento.

- [ ] **[P0] Primeiro módulo Go criado**: `libs/backend/dmpf-domain`, gerado por
  `@nx-go/nx-go:library`, declarado no `go.work` via `use ./libs/backend/dmpf-domain`.
  - Module path reescrito para
    `gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/dmpf-domain`.
    O generator emite `module libs/backend/dmpf-domain` literal, que não resolve
    remotamente; `GOPRIVATE=gitea.lidercap.com.br` já cobre o host.
  - Tags exatamente `type:lib`, `scope:backend`, `stack:go`.
  - Conteúdo mínimo: um package compilável com ao menos um teste, suficiente
    para exercitar os sete passos. `KRN-03` preenche o módulo com UPR,
    `Decision` e `Rejection`.

- [ ] **[P0] Manifesto `dmpf-units.json` no módulo**: presente na raiz de
  `libs/backend/dmpf-domain` desde a criação, schema `dmpf/units@1`, com
  `block: domain` (já fixado pela guarda-chuva) e `bounded_context: dmpf-kernel`.
  - `include` usa **import paths**, não globs (RFC §3.3, §10.1).
  - `bounded_context: dmpf-kernel` é a convenção que esta spec fixa para
    unidades de kernel; unidades de negócio declararão os seus próprios.

- [ ] **[P0] Cadeia de sete passos integrada ao Nx**: `gofmt`, `go vet`,
  `golangci-lint`, `go build`, `go test`, `go test -race`, `govulncheck`,
  todos acionáveis por `pnpm nx`.
  - Do que o plugin cobre por inferência: `test` (`go test ./...`, sem `-race`)
    e `lint` (que roda `go fmt ./...`, ou seja, **reescreve** arquivos).
  - Uma lib Go **não** ganha target `build` inferido — o plugin só o gera para
    application, detectada por `package main`.
  - Targets novos a declarar, sem colidir com os inferidos: `fmt-check`
    (`gofmt -l`, reprova quando a saída não é vazia), `vet`, `build`,
    `test-race`, `govulncheck`.
  - O target `lint` inferido é reconfigurado por `options.linter: golangci-lint`,
    sem redeclarar `executor` — o merge preserva cache e inputs do plugin.

- [ ] **[P0] Ferramentas Go pinadas**: `golangci-lint` e `govulncheck` não estão
  instalados nem local nem documentados no runner. Fixar as versões em um único
  lugar, declarado no ADR-030 como parte do BOM, e consumi-las tanto no CI
  quanto localmente.
  - `.golangci.yml` versionado na raiz, com os linters habilitados declarados.

- [ ] **[P0] Gates de CI**: `.github/workflows/ci.yml` passa a instalar o
  toolchain Go e a rodar os targets Go em `nx affected`.
  - O `ci.yml` roda em `gitea-runner` e hoje só tem o composite
    `./.github/actions/setup-node-pnpm`; não há `actions/setup-go`, cache de
    módulos Go, nem qualquer chamada a `gofmt`/`vet`/`golangci-lint`/`govulncheck`
    em nenhum workflow.
  - Não há evidência documentada de que o `gitea-runner` traga Go pré-instalado;
    a entrega não pode presumir que traga.

- [ ] **[P1] Ganchos locais**: `lefthook.yml` passa a cobrir `*.go`.
  - `pre-commit` hoje tem um único command com glob
    `*.{js,ts,jsx,tsx,mjs,cjs,json,jsonc,css}` — sem `.go`.
  - `pre-push` roda `nx affected -t lint/typecheck/test/build`, que já alcançaria
    `lint` e `test` de um projeto Go, mas não `vet`, `golangci-lint`, `-race`
    nem `govulncheck`.
  - Repartição: passos rápidos (`fmt-check`, `vet`) no `pre-commit`; passos
    lentos (`golangci-lint`, `test-race`) no `pre-push`; `govulncheck` somente
    no CI.

- [ ] **[P1] Release do módulo Go**: a tag `type:lib` arma o `nx-release.yml` e,
  por consequência, o `nx-publish-libs.yml`. O módulo Go DEVE versionar e NÃO
  DEVE ser enviado ao Verdaccio.
  - `nx-publish-libs.yml` passa `build_projects_filter: "tag:type:lib,!tag:stack:go"`.
    Esse é um input deste repositório; o reusable
    `lidercap-apps/actions-templates/.github/workflows/publish-libs.yaml@main`
    permanece intocado — seu contrato não é versionado aqui.
  - A sintaxe de negação foi verificada como válida no Nx 23.1.0.

- [ ] **[P1] Documentação sincronizada**: `AGENTS.md` e `docs/nx-reference/tasks.md`.
  - `AGENTS.md`: o inventário de libs deixa de ser "Nenhuma"; a seção de comandos
    ganha a cadeia Go; o piso do Go passa a `1.26.4`.
  - `docs/nx-reference/tasks.md`: ganha a receita de lib Go via
    `@nx-go/nx-go:library`, hoje ausente — o guia só cobre `@nx/js:lib` e
    `@nx/nest:lib`.

- [ ] **[P2] Divergência de taxonomia registrada**: `AGENTS.md` lista
  `stack:go` na taxonomia canônica, mas o ADR-002 §2 traz a lista antiga
  (`node|react|angular|universal`). Registrar a divergência, apontando o
  `AGENTS.md` como fonte da verdade.

### Não-funcionais

- [ ] **Compatibilidade**: Go `1.26.4` como piso e toolchain; Node `^24`;
  `pnpm@11.14.0` resolvido pelo campo `packageManager`; Nx `23.1.0`;
  `@nx-go/nx-go` `4.0.0`.
- [ ] **Compatibilidade — risco declarado**: `@nx-go/nx-go` 4.0.0 declara
  `@nx/devkit: ">= 20 < 23"`, faixa que **não inclui** o Nx 23.1.0 deste
  workspace. Funciona na prática hoje; o BOM registra a combinação como
  observada, não como certificada, até haver evidência.
- [ ] **Performance**: os passos Go rodam sob `nx affected` com `--parallel=3`,
  no mesmo padrão dos passos TypeScript; o `ci.yml` tem `timeout-minutes: 30`
  para o job inteiro, que a cadeia Go não pode estourar.
- [ ] **Reprodutibilidade**: as versões de `golangci-lint` e `govulncheck` são
  idênticas na máquina local e no CI, resolvidas do mesmo lugar declarado.
- [ ] **Segurança**: `govulncheck` reprova diante de CVE em dependência,
  transitiva inclusive. O achado é tratado como real; desligar o passo é
  proibido.

## Camadas afetadas

| Camada | Afetada? | Descrição |
|--------|----------|-----------|
| Workspace Go (`go.work`) | [x] | Ganha `use ./libs/backend/dmpf-domain` e piso `go 1.26.4` |
| Módulo de domínio (`libs/backend/dmpf-domain`) | [x] | Criado: `go.mod`, package mínimo, teste, `project.json` com as 3 tags, `dmpf-units.json` |
| Configuração de tasks (`nx.json`) | [x] | Targets Go adicionais em `targetDefaults`, sem redeclarar o inferido |
| CI (`.github/workflows/ci.yml`) | [x] | Setup do Go, cache de módulos e os passos Go em `nx affected` |
| Release (`.github/workflows/nx-publish-libs.yml`) | [x] | `build_projects_filter` exclui `stack:go` |
| Ganchos locais (`lefthook.yml`) | [x] | `pre-commit` e `pre-push` passam a cobrir `*.go` |
| Documentação (`AGENTS.md`, `docs/nx-reference/tasks.md`) | [x] | Inventário, cadeia de comandos e receita de lib Go |
| Decisões (`docs/adr/`) | [x] | ADR-030: granularidade de módulo e BOM do Go |
| Acervo normativo (`docs/dmpf/`) | [ ] | Intocado — artefato promovido não é editado retroativamente |
| Verificador de conformidade | [ ] | `KRN-02` (ARQ-521) |
| Contratos `.proto` / Buf | [ ] | `KRN-05` |

## Localização de código

```text
lidercap-platform/
├── go.work                                  — piso go 1.26.4 + use ./libs/backend/dmpf-domain
├── .golangci.yml                            — NOVO: linters habilitados, versionado
├── libs/backend/
│   └── dmpf-domain/                         — NOVO: primeiro módulo Go do workspace
│       ├── go.mod                           — module gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/dmpf-domain
│       ├── project.json                     — apenas tags; targets ficam inferidos
│       ├── dmpf-units.json                  — metadata_container, schema dmpf/units@1
│       ├── <package>.go                     — package mínimo compilável
│       └── <package>_test.go                — teste que exercita a cadeia
├── nx.json                                  — targetDefaults dos targets Go novos
├── lefthook.yml                             — ganchos cobrindo *.go
├── .github/workflows/
│   ├── ci.yml                               — setup-go, cache e passos Go em nx affected
│   └── nx-publish-libs.yml                  — filtro de publish exclui stack:go
├── AGENTS.md                                — inventário, comandos e piso do Go
└── docs/
    ├── adr/030-granularidade-modulo-go-e-bom.md  — NOVO
    └── nx-reference/tasks.md                — receita de lib Go
```

**Arquivos a modificar**:

- `go.work` — de `go 1.25` (arquivo de 8 bytes, sem bloco `use`) para piso
  `go 1.26.4` mais a entrada do primeiro módulo
- `nx.json` — acrescenta os targets Go que o plugin não infere; **não** redeclara
  `test`, `lint`, `tidy` nem `generate`
- `.github/workflows/ci.yml` — hoje: `biome ci` e `nx affected` de
  lint/typecheck/test/build/e2e, sem nenhum passo Go
- `.github/workflows/nx-publish-libs.yml` — `build_projects_filter` de
  `"tag:type:lib"` para `"tag:type:lib,!tag:stack:go"`
- `lefthook.yml` — `pre-commit` ganha command com glob `*.go`; `pre-push` ganha
  os passos Go pesados
- `AGENTS.md` — seções de inventário, comandos e tooling
- `docs/nx-reference/tasks.md` — seção de criação de lib Go

**Arquivos a criar**:

- `libs/backend/dmpf-domain/` — o módulo, com `go.mod`, `project.json`,
  `dmpf-units.json`, fonte e teste
- `.golangci.yml` — configuração versionada do linter
- `docs/adr/030-granularidade-modulo-go-e-bom.md` — o ADR

## Design

### Arquitetura

O terreno reflete em diretórios a mesma regra de dependência da fundação. O que
esta entrega acrescenta é a materialização em Go, com o import path como
`canonical_key`.

```text
ownership_module            = diretório com go.mod   → libs/backend/dmpf-domain
  └── verification_unit     = package Go             → cada diretório de package
        canonical_key       = import path completo   → gitea.lidercap.com.br/lidercap-apps/
                                                        lidercap-platform/libs/backend/dmpf-domain
        metadata_container  = dmpf-units.json        → na raiz do ownership_module
```

Um `go.mod` por lib faz `ownership_module` e projeto Nx coincidirem. É essa
coincidência que dá, de graça: manifesto por lib, versionamento independente no
Nx Release, e um alvo natural para `nx affected`.

### Fluxo principal — o que acontece quando o módulo nasce

1. `@nx-go/nx-go:library` cria `libs/backend/dmpf-domain/go.mod` e acrescenta a
   entrada ao bloco `use` do `go.work`.
2. O module path emitido pelo generator (`module libs/backend/dmpf-domain`,
   literal) é reescrito para o host Gitea, que `GOPRIVATE` já cobre.
3. O plugin descobre o projeto pelo glob `**/go.mod` e infere `test`, `lint`,
   `tidy` e `generate`. Não infere `build`, porque o módulo é lib e não tem
   `package main`.
4. `project.json` recebe apenas as três tags. Nenhum target é redeclarado.
5. `nx.json` acrescenta os targets que faltam à cadeia, com nomes que não
   colidem com os inferidos.
6. O primeiro projeto `tag:type:lib` do workspace passa a existir: o
   `nx-release.yml` sai de ocioso para armado, e o filtro de publish impede que
   um artefato Go siga para o Verdaccio.

### Fluxo de validação — os sete passos

```text
passo            comando                    origem                       reprova quando
────────────────────────────────────────────────────────────────────────────────────────
1 gofmt          gofmt -l                   target novo fmt-check        saída não vazia
2 go vet         go vet ./...               target novo vet              exit != 0
3 golangci-lint  golangci-lint run ./...    lint inferido reconfigurado  exit != 0
4 go build       go build ./...             target novo build            exit != 0
5 go test        go test ./...              target inferido test         exit != 0
6 go test -race  go test -race ./...        target novo test-race        exit != 0
7 govulncheck    govulncheck ./...          target novo govulncheck      CVE encontrada
```

O passo 1 usa `gofmt -l`, e não `go fmt`. O `lint` inferido pelo plugin roda
`go fmt ./...`, que **reescreve** os arquivos — comportamento impróprio para um
gate, que precisa ser read-only e reprovar diante de diferença.

### Pseudocódigo — a decisão do gate de formatação

```text
saida = executar("gofmt", "-l", ".")

se saida.texto != "":
    // gofmt -l imprime o NOME de cada arquivo mal formatado e sai com 0.
    // O exit code não serve de gate; a saída é que serve.
    reprovar("arquivos fora do formato:\n" + saida.texto)
senão:
    aprovar()
```

## Decisões técnicas

- **Granularidade — um `go.mod` por lib Nx**: porque o `ownership_module` é o
  portador do `metadata_container` (RFC §3.6), e é a fronteira de ownership que
  o DMPF usa para classificar. Também preserva o versionamento independente que
  o `nx.json` já declara (`projectsRelationship: independent`, tag
  `{projectName}@{version}`).
  Alternativa descartada: módulo único via `@nx-go/nx-go:convert-to-one-mod`,
  porque colapsaria todo o kernel Go em um único `ownership_module`, com um
  manifesto só, e removeria o versionamento por lib.
  Custo aceito: cada lib exige entrada no `go.work` e, entre libs locais,
  diretivas `replace` — o `golibs` precisou de um script de sincronização para
  administrar isso com 15 módulos.

- **Piso e toolchain — ambos `1.26.4`**: o `go.work` passa a declarar
  `go 1.26.4`, o BOM certifica `go1.26.4`, e o CI fixa a mesma versão.
  Alternativa descartada: piso `1.25` com toolchain certificado `1.26.4` (modelo
  N/N-1 do ADR-027), porque exige raciocinar sobre dois números onde um resolve.
  Consequência: sem folga N-1, um bump de minor do Go passa a exigir decisão
  explícita de BOM — o que é o comportamento desejado.

- **Module path com host Gitea**: `gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/<nome>`,
  espelhando o padrão do `golibs`. Alternativa descartada: aceitar o
  `module libs/backend/<nome>` literal que o generator emite, porque não resolve
  em consumo remoto e tornaria a `canonical_key` dependente do layout de
  diretórios em vez do endereço público.

- **Primeiro módulo — `libs/backend/dmpf-domain`**: é o primeiro da ordem de
  dependências da guarda-chuva, e `KRN-03` o preenche. Alternativa descartada:
  criar um módulo de terreno descartável, porque seria removido logo depois e
  deixaria o `go.work` com histórico de entrada morta.
  O `block: domain` não é decisão desta spec — a guarda-chuva já o fixou.

- **`type:lib` com publish filtrado**: o módulo Go recebe `type:lib`, versiona e
  ganha tag, mas o `build_projects_filter` do `nx-publish-libs.yml` passa a
  `"tag:type:lib,!tag:stack:go"`.
  Alternativa descartada: deixar o filtro como está e confiar em que o reusable
  externo pule o projeto Go, porque o contrato dele não é versionado neste
  repositório — o próprio arquivo diz isso — e uma falha ali quebraria o CI no
  push da tag.
  Alternativa descartada: manter Go fora do `release.projects`, porque
  contraria o critério de aceite da ARQ-520, que exige `nx release --dry-run`
  concluindo com o primeiro `tag:type:lib` presente.

- **`lint` reconfigurado, não redeclarado**: o target `lint` inferido recebe
  `options.linter: golangci-lint` no `project.json`, sem declarar `executor`.
  Declarar o executor de novo sobrescreveria cache e inputs do plugin em
  silêncio (ADR-002). A formatação sai do `lint` e passa ao target `fmt-check`,
  read-only.

- **ADR numerado 030**: o `029` já está ocupado por
  `029-titular-da-revisao-de-seguranca-fnd-07.md`. A guarda-chuva diz "a partir
  de `029`" e está desatualizada nesse ponto; a correção é registrada aqui, sem
  editar o artefato promovido.

- **Manifesto desde o nascimento**: `dmpf-units.json` é criado junto com o
  módulo, ainda que o verificador só chegue em `KRN-02`. Fail-closed não admite
  um módulo de produção existir sem manifesto, mesmo durante a janela em que
  ninguém o verifica mecanicamente.

- **`bounded_context: dmpf-kernel`**: convenção fixada por esta spec para
  unidades de kernel, que não são bounded context de negócio. A RFC exige string
  estável e única no universo (§5.4); esta satisfaz sem fingir domínio de
  negócio onde não há.

## Regras relacionadas

- `AGENTS.md` — taxonomia canônica de tags 3D; vedação a redeclarar targets;
  Conventional Commits em PT-BR com scope igual ao projeto Nx; git-flow com
  `master` e `develop` protegidas
- `.claude/rules/process-enforcement.md` — cadeia de validação Go documentada
  para este workspace
- `docs/adr/002-nx-task-configuration.md` — a regra que governa o desenho dos
  targets desta entrega
- `docs/adr/027-bom-combinacao-certificada-compatibilidade-e-escape-hatch.md` —
  o modelo que o BOM do Go instancia
- [SPEC-YRJRADY9](./SPEC-YRJRADY9-dmpf-kernel-sdk-go.md) — guarda-chuva; esta
  spec executa o item `KRN-01` da sua decomposição
- ARQ-521 (`KRN-02`) — consome este terreno; o verificador Go precisa de nome
  distinto de `tools/dmpf-verify.mjs` e do workflow `dmpf-verify.yml`, que já
  existem e verificam o acervo `docs/dmpf/`

## Verificação e testes

### Critérios de aceite

- [ ] `pnpm nx show projects` lista `dmpf-domain` com exatamente as tags
  `type:lib`, `scope:backend` e `stack:go` — nem mais, nem menos.
- [ ] `go build ./libs/backend/dmpf-domain/...` compila a partir da raiz do
  repositório, e `go work sync` conclui sem erro. O padrão `./...` na raiz
  **não** serve: a raiz não é um módulo, e o Go reprova com `directory prefix .
  does not contain modules listed in go.work`. Da raiz vale o padrão de
  subárvore ou o module path completo; dentro do módulo, `./...` funciona — que
  é como os targets rodam, com `cwd` no `projectRoot`.
- [ ] O `go.work` declara `go 1.26.4` e contém `use ./libs/backend/dmpf-domain`.
- [ ] O `go.mod` do módulo declara
  `module gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/dmpf-domain`.
- [ ] Os sete passos rodam por `pnpm nx` e reprovam diante de arquivo mal
  formatado, achado do `vet`, achado do `golangci-lint` ou teste falhando.
- [ ] `pnpm nx show project dmpf-domain --json` mostra os targets inferidos
  (`test`, `lint`, `tidy`, `generate`) com `cache: true` e os `inputs` do plugin
  preservados — nenhum deles zerado por redeclaração.
- [ ] `pnpm nx show project dmpf-domain --json` mostra o target `lint`
  resolvendo para o executor do `@nx-go/nx-go`, e **não** para o
  `targetDefaults.lint` do `nx.json`, que roda `biome lint {projectRoot}` e não
  processa Go.
- [ ] `libs/backend/dmpf-domain/dmpf-units.json` existe, valida contra o schema
  `dmpf/units@1`, declara `block` e `bounded_context`, e usa import paths em
  `include`.
- [ ] O `ci.yml` instala o toolchain Go na versão do BOM e roda os targets Go em
  `nx affected`, reprovando o PR quando qualquer passo falha.
- [ ] O `pre-push` do `lefthook.yml` bloqueia o push quando a cadeia Go reprova.
- [ ] `pnpm nx release --dry-run` conclui sem erro com o primeiro projeto
  `tag:type:lib` presente.
- [ ] `pnpm nx show projects --projects="tag:type:lib,!tag:stack:go" --json`
  retorna `[]` com o módulo Go presente — provando que o filtro de publish o
  exclui.
- [ ] As versões de `golangci-lint` e `govulncheck` estão declaradas em um único
  lugar, e a mesma versão é usada localmente e no CI.
- [ ] `pnpm biome ci .` e `pnpm nx affected -t lint,typecheck,test,build` passam.
- [ ] O ADR-030 existe com status "Aceito", a alternativa descartada e as
  consequências.
- [ ] `AGENTS.md` não diz mais "Nenhuma" no inventário de libs, e
  `docs/nx-reference/tasks.md` tem a receita de lib Go.
- [ ] Nenhum artefato criado declara ou sugere exactly-once fim a fim.

### Cenários de teste

```text
DADO o módulo libs/backend/dmpf-domain criado, com go.work declarando
     go 1.26.4 e a entrada use, e o toolchain go1.26.4 disponível
QUANDO rodar pnpm nx run dmpf-domain:fmt-check, :vet, :lint, :build, :test,
     :test-race e :govulncheck
ENTÃO os sete passos concluem com êxito e o projeto aparece em
     pnpm nx show projects com as três tags 3D

DADO um arquivo .go do módulo com indentação por espaços em vez de tabulação
QUANDO rodar pnpm nx run dmpf-domain:fmt-check
ENTÃO o target REPROVA imprimindo o caminho do arquivo, e o arquivo
     permanece inalterado no disco (o gate é read-only, não reformata)

DADO o targetDefaults.lint do nx.json, que roda biome lint {projectRoot}
QUANDO rodar pnpm nx show project dmpf-domain --json
ENTÃO o target lint resolve para o executor do @nx-go/nx-go com
     options.linter igual a golangci-lint, e não para o comando do Biome

DADO o módulo Go com tag type:lib presente no workspace
QUANDO rodar pnpm nx show projects --projects="tag:type:lib,!tag:stack:go" --json
ENTÃO o retorno é [], confirmando que o filtro do nx-publish-libs.yml
     exclui o módulo Go do envio ao Verdaccio

DADO que o repositório nunca teve um projeto tag:type:lib
QUANDO o primeiro módulo Go passa a existir e roda pnpm nx release --dry-run
ENTÃO o comando conclui sem erro, e o step "Detect lib release candidates"
     do nx-release.yml passa a reportar has_libs=true

DADO um package do módulo que importa net/http, violando a regra de dependência
     para o bloco domain
QUANDO rodar a cadeia de validação
ENTÃO o gate REPROVA — e, enquanto o verificador de KRN-02 não existir, a
     reprovação vem do golangci-lint com a regra de import proibido configurada
     no .golangci.yml

DADO uma dependência do módulo com CVE conhecida
QUANDO rodar pnpm nx run dmpf-domain:govulncheck
ENTÃO o target REPROVA reportando a vulnerabilidade, e o passo NÃO é
     desligado nem tem a versão fixada para trás sem decisão registrada

DADO um desenvolvedor com go1.26.0 instalado e GOTOOLCHAIN=auto
QUANDO rodar go build ./libs/backend/dmpf-domain/... na raiz, com o go.work
     declarando go 1.26.4
ENTÃO o Go baixa o toolchain 1.26.4 automaticamente e a compilação conclui,
     sem exigir instalação manual

DADO um arquivo .go mal formatado no stage do git
QUANDO tentar o commit
ENTÃO o gancho pre-commit do lefthook REPROVA o commit apontando o arquivo

DADO um go.mod novo criado sem a entrada correspondente no go.work
QUANDO rodar pnpm nx show projects e, em seguida, go build sobre o caminho
     desse módulo a partir da raiz
ENTÃO o projeto é descoberto pelo plugin (que usa o glob **/go.mod), mas o
     build REPROVA porque o módulo não consta do go.work — evidenciando que a
     declaração é obrigatória e não opcional
```

<critical_constraints>
- [P0] Nenhum artefato desta entrega declara nem sugere exactly-once fim a fim; a
  garantia é at-least-once com efeitos idempotentes (RFC §2.3, P0-3).
- [P0] Fail-closed: lacuna de configuração, classificação ausente, manifesto
  ausente e caso degenerado REPROVAM; nunca são lidos como conformidade
  (RFC §3.6).
- [P0] NUNCA redeclarar em `project.json` um target que o `@nx-go/nx-go` ou os
  `targetDefaults` já forneçam com o mesmo executor — isso sobrescreve cache e
  inputs em silêncio (ADR-002).
- [P0] A aresta `domain → port` é proibida, sem condicional e sem exceção
  (RFC §7; ADR-014).
- [P0] Esta spec NÃO reabre decisão da fundação nem da guarda-chuva: divergência
  encontrada vira ADR novo ou escape hatch declarado, nunca alteração silenciosa
  da norma.
- [P0] Todo módulo Go de produção DEVE nascer com o `metadata_container`
  `dmpf-units.json` (schema `dmpf/units@1`) na raiz do seu `ownership_module`,
  ainda que o verificador só chegue em `KRN-02` — manifesto ausente reprova
  (RFC §3.6; ADR-012).
- [P1] Todo módulo de produção carrega exatamente três tags 3D, uma por dimensão
  (`AGENTS.md`); para o primeiro módulo: `type:lib`, `scope:backend`, `stack:go`.
</critical_constraints>

## Escopo fora

- **Verificador de conformidade DMPF em Go**: pertence a `KRN-02` (ARQ-521).
  Esta entrega cria o terreno e o manifesto que o verificador vai consumir, mas
  não o verificador. O binário de `KRN-02` precisa de nome distinto de
  `dmpf-verify`, já ocupado por `tools/dmpf-verify.mjs` e pelo workflow
  `dmpf-verify.yml`, que verificam o acervo `docs/dmpf/` e nada têm com Go.

- **Conteúdo do domínio — UPR, `Decision`, `Rejection`**: pertence a `KRN-03`.
  Esta entrega cria o módulo `dmpf-domain` com package mínimo e teste, apenas o
  suficiente para exercitar a cadeia.

- **Demais módulos da árvore** (`dmpf-application`, `dmpf-ports`, os
  `dmpf-provider-*`, `dmpf-observability`, `dmpf-contracts`, `dmpf-testkit`,
  `dmpf-reference`): cada um nasce na sua sub-spec. `KRN-01` fixa a convenção
  que todos seguirão, e materializa apenas o primeiro.

- **`.proto`, `buf.yaml` e gates do Buf**: pertencem a `KRN-05`. O `catalog:` do
  `pnpm-workspace.yaml` só entra em escopo se a geração introduzir dependência
  Node, o que não acontece nesta entrega.

- **Correção das violações do `golibs`** (`EventPublisher` em `goservice/domain`,
  `net/http` em `goweb/domain`): são de outro repositório. Aqui elas servem
  como motivação do gate, não como trabalho a executar.

- **Publicação de módulos Go em registry Go**: fora do épico. Esta entrega
  apenas garante que o módulo Go não seja empurrado ao Verdaccio, que é
  npm-only.

- **Renomear o pacote raiz** de `@nx-base-template/source` para um nome da
  organização: mudança transversal que afeta `--exclude` em `lefthook.yml`, CI e
  workflows. Merece decisão própria, fora do escopo de `KRN-01`.
