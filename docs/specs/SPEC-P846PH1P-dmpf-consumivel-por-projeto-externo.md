---
id: SPEC-P846PH1P
slug: dmpf-consumivel-por-projeto-externo
title: DMPF consumível por projeto externo — plugin publicado, conformance em modo consumidor e prova de consumo
stage: building
priority: P1
depends_on: []
ticket_url: https://linear.app/mmda/issue/DEVS-70/spec-p846ph1p-dmpf-consumivel-por-projeto-externo-plugin-publicado
subtask_urls: []
created: 2026-10-06
---

# SPEC-P846PH1P: DMPF consumível por projeto externo — plugin publicado, conformance em modo consumidor e prova de consumo

## Resumo

Tornar o DMPF consumível **por versão** por repositórios fora deste monorepo.
Hoje só o kernel Go é consumível de fato: o verificador reprova quem recebe o
kernel por tag, o plugin Nx é privado e preso ao layout do platform, e scripts,
infra, ativos de IA e CI só existem aqui dentro. Como mantenedor de um projeto
greenfield criado a partir do `nx-base-template`, quero instalar uma versão de
`@mateusmacedo/dmpf-plugin`, rodar `init` e `bounded-context` e obter um
contexto que compila, passa nos gates e se atualiza com `nx migrate`, sem
copiar nada do platform à mão.

## Contexto

- **Problema** (spike de consumo externo de 06/10/2026, Go 1.27.1 e
  `GOTOOLCHAIN=go1.26.6`, kernel em `v1.0.0-rc.1`):
  - **Funciona por versão:** `go get` e build das libs do kernel, sem
    `require` quebrado; `go run`/`go install` de
    `github.com/mateusmacedo/dmpf/tools/dmpf-conformance/cmd/{conformance,modsync,infrasync}@v1.0.0-rc.1`.
  - **Verificador reprova o consumidor.** Com o kernel por tag, o
    `conformance` sai com `DMPF-E001` (kernel como dependência externa sem
    capability); declarado `pure`, com `DMPF-E002`. Causas:
    `tools/dmpf-conformance/internal/conformance/check.go:170-185` (só
    unidades do universo local), `internal/fsstore/inventory.go:95,136`
    (inventário só local), `internal/rule/capability.go:44-49` (domain e port
    só aceitam `pure`), `internal/rule/external.go:116,154-170` (pureza
    transitiva reprova externo `pure`), `internal/rule/decide.go:56-60`
    (D002 exige `shared_kernel_units`, que só se designa editando o JSON —
    `internal/baseline/baseline.go:169`), `cmd/conformance/main.go:135` e
    `fitness/fitness.go:90` (perfis em caminho fixo).
  - **Plugin não publicável.** `tools/dmpf-plugin/package.json:4` é
    `private: true`; `generators.json` aponta para `./src/...`, mas `files`
    publica só `dist` e `generators.json`; o build não copia `files/` nem
    `schema.json`.
  - **Generator preso ao platform.** `MODULE_PREFIX` e `NPM_SCOPE` fixos
    (`generator.ts:21,24`); só aceita `apps/backend` (`:93-97`); roda
    `go run ./tools/dmpf-conformance/...` (`:25-26,490-495`); targets
    dependem de `testkit:test-infra-up`, `bff:infra-up` e `infra-session`
    (`files/module/project.json:52,78,82,113,133`); o Dockerfile copia
    `libs/backend/go` e `tools/dmpf-conformance` (`files/app/Dockerfile:24-25`);
    templates de deploy e contrato fixam `spiffe://dmpf/bff`,
    `ghcr.io/mateusmacedo/` e `buf.build/mateusmacedo`; versões de ferramentas
    fixas em `blocks.ts:35-47`, `files/contract/buf.gen.yaml:14`,
    `files/module/project.json:61`, `files/app/Dockerfile:20`,
    `tools/buf.sh:3` e `nx.json:173`.
  - **Scripts e infra presos ao layout.** `tools/dmpf-context-check.sh:26-27,46`,
    `tools/test-infra.sh:4,12`, `tools/test-env.sh:6`,
    `tools/buf-gate.sh:30,92`; `infra/observability/alloy/alloy-kubernetes.alloy:19`
    fixa `bff|orders|reservations|bookings`; o `infrasync` fixa o layout de
    infra (`infrasync/infrasync.go:24-26`, `infrasync/templates.go:11,29-36`).
  - **Ativos de IA citam o que só existe aqui.** `.agents/skills/dmpf-bounded-context/**`,
    `.agents/skills/dmpf-testkit/**`, `.claude/agents/dmpf-context-author.md`,
    `.claude/rules/dmpf-bounded-context.md` e `.claude/commands/dmpf-new-context.md`
    apontam para `apps/backend/{bookings,orders,reservations,bff}`,
    `libs/backend/go/**`, `tools/dmpf-*` e `docs/guides/dmpf-manifesto.md`.
  - **Sem prova automatizada.** O critério "repo externo faz `go get`" segue
    desmarcado e sem gate de CI
    (`docs/specs/SPEC-95AHV4D4-dmpf-tags-modulo-go-consumo.md:407,465`).
  - **Premissa vencida.** A ADR-059 removeu BOM e release do produto porque
    "não há consumidor externo" (`docs/adr/059-remover-o-bom-e-a-release-do-produto.md:11`).
    Projetos criados a partir do template são esse consumidor.
- **Impacto**: um projeto novo passa de "copiar e adaptar o platform" para
  `nx add` + `init` + `bounded-context`, com atualização por `nx migrate`. O
  platform deixa de ter duas cópias de scripts, infra e ativos de IA.
- **Inspiração**: plugins publicáveis do próprio Nx (`@nx/*`), que combinam
  generator `init` com migrations; módulos Go por tag (ADR-047); workflows
  reutilizáveis do GitHub Actions (`workflow_call`).
- **Links relevantes**:
  - SPEC-TXJ6A4XX (repo `nx-base-template`) — o template que consome esta
    entrega; depende desta spec.
  - SPEC-95AHV4D4 — tags de módulo Go e consumo fora do workspace.
  - ADR-030, ADR-045, ADR-046, ADR-047, ADR-048, ADR-053, ADR-054, ADR-059.
  - `docs/guides/dmpf-composicao.md`, `docs/guides/dmpf-manifesto.md`,
    `docs/guides/dmpf-novo-app-servico.md`.

<constraints>
- [P0] Consumo só por versão: o consumidor não tem `replace` para módulos do
  kernel, caminho relativo ao platform nem arquivo copiado à mão.
- [P0] Fonte única por ativo: scripts, infra, ativos de IA e `.golangci.yml`
  têm uma fonte canônica neste repo; o pacote e o próprio platform consomem a
  mesma fonte, e o CI reprova divergência.
- [P0] O platform continua verde: `conformance`, `modsync --check`,
  `infrasync --check`, context-check, `nx affected -t lint,typecheck,test,build`
  e a pirâmide Go sem regressão; o golden `bookings` regenerado sai com diff
  zero.
- [P0] Registry npm é o GitHub Packages (`https://npm.pkg.github.com`, escopo
  `@mateusmacedo`); token só por variável de ambiente ou secret, nunca em
  arquivo versionado.
- [P1] Uma versão do plugin fixa as versões do kernel, do `dmpf-conformance` e
  das ferramentas (lockstep); o consumidor não escolhe versões avulsas.
- [P1] A prova de consumo externo é um job de CI, não um artefato de release:
  BOM e certificação de produto continuam fora (ADR-059 vale nesse ponto).
- [P1] Targets novos não redeclaram o que plugins de inferência ou
  `targetDefaults` já fornecem (ADR-002).
</constraints>

## Requisitos

### Funcionais

**A. Conformance em modo consumidor**

- [ ] **[P0] A1 — Kernel por tag entra no universo**: o inventário resolve,
  pelos pacotes do `go list -deps -json` que o verificador já executa (campos
  `Module.Dir`, `Module.Version` e `Module.Main`), os módulos fora do workspace
  que publicam `dmpf-units.json` e carrega as unidades deles como **shared
  kernel de leitura**: servem de destino de arestas e não são verificadas.
  - Edge: `dmpf-units.json` inválido no módulo baixado → exit 2 citando
    `<módulo>@<versão>/dmpf-units.json`.
  - Edge: módulo ausente do cache e sem rede → exit 2 com
    `NAO VERIFICADO`, nunca exit 0.
- [ ] **[P0] A2 — Designação automática**: unidades vindas de A1 dispensam
  `shared_kernel_units` no baseline e satisfazem a D002. A designação manual
  continua valendo para kernel local (caso do platform).
- [ ] **[P0] A3 — Perfis embutidos**: `build-profiles.json` vai embutido com
  `go:embed` em `conformance` e `fitness`; `--profiles` continua sobrescrevendo.
- [ ] **[P1] A4 — `modsync` no consumidor**: o primeiro contexto de um repo
  sem kernel na build list recebe os `require` do kernel na versão de
  `versions.json` (B2), sem o erro "rode go get antes"
  (`modsync/modsync.go:145-148`). Tags `-rc.N` cobertas por teste de unidade.
- [ ] **[P1] A5 — `infrasync` parametrizado**: lê de `dmpf.json` (C1) o
  diretório de apps, a borda pública, o trust domain SPIFFE e o registry de
  imagens; gera a lista de serviços do Alloy em substituição à regex fixa de
  `alloy-kubernetes.alloy:19`.

**B. Plugin publicável e versões fixadas**

- [ ] **[P0] B1 — Pacote publicável**: `@mateusmacedo/dmpf-plugin` sem
  `private`; `generators.json`, `executors.json` e `migrations.json` apontam
  o código para `dist` e os schemas para `src`; templates, scripts e
  `versions.json` ficam na raiz do pacote e entram pelo campo `files`, sem
  target de cópia no build (ADR-002).
  `pnpm nx add @mateusmacedo/dmpf-plugin` num workspace Nx 23.x externo
  instala e lista os generators.
- [ ] **[P0] B2 — Manifesto de versões**: o pacote carrega `versions.json`
  com a tag do kernel, a versão do `dmpf-conformance`, a versão Go (diretiva e
  imagem), buf, protoc-gen-go, golangci-lint, govulncheck e o `workflowRef`,
  SHA do commit-fonte da release usado pelo workflow reutilizável (F1).
  Generators, executors e migrations leem dele; nenhum template fixa versão.
- [ ] **[P1] B3 — Release no GitHub Packages**: o release group `npm` publica
  o plugin com `.github/workflows/publish-libs.yaml`, num job encadeado ao
  versionamento no `nx-release.yml` (`needs`), e não pelo push da tag, que
  com o `GITHUB_TOKEN` não dispara outro workflow. O dist-tag deriva da
  versão: pré-release vai para `next`, estável para `latest`. `versions.json`
  é preenchido no fluxo de release, nunca à mão: `sync-versions --target
  <versão> --workflow-ref <SHA do commit-fonte>` roda antes do `nx release`,
  com a versão-alvo explícita, e o arquivo entra no commit de versão.

**C. Generators**

- [ ] **[P0] C1 — `init`**: idempotente. Opções: `--modulePrefix`
  (opcional, com `x-prompt` e default reservado `example.com/change-me`,
  porque o `nx add` roda o `init` sem repassar opções), `--appsDir` (default
  `apps/backend`), `--imageRegistry`, `--spiffeTrustDomain`, `--bufModule`,
  `--force`. Escreve ou atualiza:
  - `dmpf.json` (prefix, appsDir, borda, registry, trust domain, versão do
    plugin e `tooling.mode`: `local` no platform, `version` no consumidor);
  - `go.work` com a diretiva de `versions.json`;
  - `nx.json` (namedInput `go`, `targetDefaults` Go, plugin `@nx-go/nx-go`);
  - `.golangci.yml` com depguard por bloco e allow do kernel e do
    `--modulePrefix`;
  - o esqueleto estático de `infra/local`, `infra/test`,
    `infra/observability` e `infra/k8s/base`; os arquivos de infra gerados a
    partir dos contextos continuam pertencendo ao `infrasync`;
  - `.env.example` e `tools/dmpf-baseline/units-baseline.json` vazio;
  - `.github/workflows/dmpf-ci.yml` chamador de F1;
  - ativos de IA (E1);
  - `dmpf.rendered.json`, com o hash de cada arquivo gerenciado que escreveu.
  - Edge: segunda execução sem mudança de entrada altera 0 arquivos.
  - Edge: arquivo gerado pelo `init` e não editado (hash igual ao registrado
    em `dmpf.rendered.json`) é re-renderizado sem `--force`, o que cobre a
    transição do prefixo reservado para o real e a troca de versão. Arquivo editado não é
    sobrescrito sem `--force`; o generator lista cada arquivo editado e sai
    com erro.
  - Edge: `modulePrefix` igual ao valor reservado `example.com/change-me`
    grava o arquivo, e `bounded-context` recusa rodar com a instrução
    `pnpm nx g @mateusmacedo/dmpf-plugin:init --modulePrefix=<path>`.
- [ ] **[P0] C2 — `bounded-context` desacoplado**: lê `dmpf.json`; não tem
  prefixo, escopo nem diretório fixos; chama `conformance`, `modsync` e
  `infrasync` por `go run <módulo>@<versão de versions.json>`; targets usam
  executors (D1) em vez de `bash ../tools/*.sh`; não depende dos projetos
  `bff`, `testkit`, `postgres` nem `app`; Dockerfile sem `COPY` de `libs/`
  e de `tools/`; deploy e contrato sem `bff`, `ghcr.io/mateusmacedo`,
  `buf.build/mateusmacedo` e `spiffe://dmpf` fixos.
  - No platform, `dmpf.json` declara o layout atual, e
    `tools/dmpf-generator-check.sh` comprova diff zero contra o golden.
- [ ] **[P1] C3 — Infra exige `init`**: o generator falha com instrução de
  rodar `init` quando `infra/local/docker-compose.yml` não existe (hoje ignora
  em silêncio, `generator.ts:452-458`).

**D. Executors e dogfooding**

- [ ] **[P0] D1 — Scripts viram executors**: `dmpf-context-check.sh`,
  `dmpf-gate-check.sh`, `buf-gate.sh`, `buf.sh`, `go-tidy.sh`, `test-env.sh` e
  `test-infra.sh` passam a ser executors `@mateusmacedo/dmpf-plugin:<nome>`,
  parametrizados por `dmpf.json` em vez de paths fixos.
- [ ] **[P0] D2 — Uma cópia só**: o platform usa os mesmos executors; os
  `.sh` deixam de existir em `tools/` como segunda cópia. Ficam só no platform:
  `otel-env-check.sh`, `grafana-provisioning-check.sh`, `dmpf-cell-check.sh`,
  `dmpf-shared-kernel-check.sh`, `dmpf-generator-check.sh`,
  `dmpf-harness-check.sh`, `dmpf-verify*.mjs` e `adr-verify.mjs`.

**E. Ativos de IA**

- [ ] **[P0] E1 — Fonte no pacote**: os 5 ativos (skills `dmpf-bounded-context`
  e `dmpf-testkit`, agente `dmpf-context-author`, rule `dmpf-bounded-context`,
  comando `dmpf-new-context`) têm fonte canônica em
  `tools/dmpf-plugin/templates/ai/`; `init` e as migrations escrevem em
  `.claude/{agents,rules,commands}` e `.agents/skills` do consumidor.
- [ ] **[P0] E2 — Referências portáveis**: exemplos e fontes do platform viram
  URLs `https://github.com/mateusmacedo/dmpf/blob/<tag>/<path>` resolvidas na
  geração; `./tools/dmpf-conformance/cmd/...` vira `go run ...@<versão>`; o
  `paths:` da rule passa a `<appsDir>/**`; referências a skills que o
  consumidor não tem (`skill-go`, `skill-ddd`) saem do agente.
- [ ] **[P1] E3 — Platform sem divergência**: o platform recebe os ativos
  pelo mesmo generator, e o CI reprova diferença entre a fonte do pacote e
  `.claude/` e `.agents/` do platform.

**F. Workflow reutilizável**

- [ ] **[P0] F1 — `dmpf-go-ci.yml`**: workflow `workflow_call` com a pirâmide
  Go por `layer:*`, gates Buf, `modsync --check`, `infrasync --check`,
  `conformance` e context-check, com inputs para versão Go, versão do plugin
  e base de comparação. O `ci.yml` do platform passa a chamá-lo; o `init`
  escreve no consumidor um chamador fixado pelo SHA de `versions.json`.

**G. Migrations**

- [ ] **[P1] G1 — Atualização por `nx migrate`**: toda versão que muda arquivo
  copiado pelo `init` (infra, `.golangci.yml`, ativos de IA, chamador de CI,
  versões do kernel) traz migration; `pnpm nx migrate @mateusmacedo/dmpf-plugin@<nova>`
  seguido de `pnpm nx migrate --run-migrations` atualiza os arquivos e sobe os
  `require` do kernel nos `go.mod` das apps. As migrations editam só a `Tree`,
  porque não podem rodar comando depois do flush: os comandos externos (o
  tidy pelo executor `go-tidy`) voltam em `nextSteps`, para o usuário rodar.
  Cada migration tem uma entrada só, na versão do pacote, mantida pelo
  `sync-versions`; arquivo editado não é sobrescrito e vai para o log.

**H. Prova de consumo externo**

- [ ] **[P0] H1 — Job `external-consumer` em PR**: cria um workspace Nx vazio
  fora da árvore, instala o plugin por `pnpm pack` (sem publicar), roda
  `init --modulePrefix=example.com/consumer` e `bounded-context` com 1 agregado
  e exige `go build ./...`, `go vet ./...`, testes unitários, `conformance`
  exit 0, `modsync --check` e `infrasync --check` conformes, context-check
  verde e `buf lint`. Como a tag ainda não existe em PR, o kernel e o
  `dmpf-conformance` do próprio PR entram por um proxy Go `file://`, numa
  versão efêmera derivada do SHA, com um `versions.json` exclusivo do teste
  apontando para ela: o consumidor resolve o kernel pelo mesmo caminho por
  tag de A1.
- [ ] **[P1] H2 — Job pós-release**: depois do `nx-release`, o mesmo cenário
  roda com kernel pelo proxy público e plugin do GitHub Packages, executando
  os comandos que as migrations devolvem em `nextSteps`; falha marca o run
  como vermelho, sem rollback automático. O teste de upgrade de X para Y
  começa na release seguinte à primeira publicada.

**I. Decisão e documentação**

- [ ] **[P0] I1 — ADR-060 "O DMPF tem consumidor externo"**: registra consumo
  por versão, o plugin como âncora única, GitHub Packages e ativos de IA pelo
  plugin; supersede a premissa da ADR-059 (`:11`) mantendo fora BOM e
  certificação; atualiza a ADR-046 no ponto "pacote npm privado, como o
  plugin". A ADR-030 fica: o `package.json` privado dos módulos Go continua.
- [ ] **[P1] I2 — Guia de consumo**: `docs/guides/dmpf-consumo-externo.md`, do
  token do GitHub Packages ao primeiro contexto verde e ao `nx migrate`;
  `tools/dmpf-plugin/README.md` reescrito (hoje desatualizado em `:48-113`).

### Não-funcionais

- [ ] Compatibilidade: Nx 23.x, Node 24, pnpm 11.14.0, diretiva Go 1.26.6.
- [ ] Desempenho: o job `external-consumer` termina em até 10 min no
  `ubuntu-latest` com cache de Go e pnpm.
- [ ] Segurança: workflows de consumo com `permissions: packages: read`;
  workflow reutilizável referenciado por SHA; nenhum token em log.
- [ ] Reprodutibilidade: `init` e `bounded-context` determinísticos — a mesma
  entrada gera o mesmo diff byte a byte.

## Camadas afetadas

| Camada | Afetada? | Descrição |
|--------|----------|-----------|
| Kernel (`libs/backend/go/*`) | [x] | Só bump de `require` do `testkit` para a nova versão do `dmpf-conformance` |
| Tooling Go (`tools/dmpf-conformance`) | [x] | Modo consumidor, perfis embutidos, `modsync` e `infrasync` parametrizados |
| Plugin Nx (`tools/dmpf-plugin`) | [x] | Publicação, `versions.json`, `init`, `bounded-context` desacoplado, executors, migrations, ativos de IA |
| Scripts (`tools/*.sh`) | [x] | Viram executors; a cópia em `tools/` sai |
| Infra (`infra/`) | [x] | Fonte canônica copiada pelo `init`; Alloy gerado |
| Apps (`apps/backend/*`) | [x] | Targets passam a usar executors; golden com diff zero |
| CI (`.github/workflows`) | [x] | `dmpf-go-ci.yml` reutilizável e jobs `external-consumer` |
| Ativos de IA (`.claude/`, `.agents/`) | [x] | Fonte migra para o pacote; platform recebe pelo generator |
| Docs e ADR | [x] | ADR-060, guia de consumo, README do plugin |

## Localização de código

```text
tools/dmpf-conformance/
  internal/golist/         — módulos do kernel pelos pacotes do go list -deps (A1)
  internal/fsstore/        — perfis de build embutidos (A3)
  internal/conformance/    — universo com shared kernel de leitura (A1, A2)
  internal/rule/           — D002 e pureza para shared kernel (A2)
  cmd/conformance/, fitness/ — perfis embutidos (A3)
  modsync/, infrasync/     — consumidor e layout parametrizado (A4, A5)
tools/dmpf-plugin/
  versions.json            — manifesto de versões (B2)
  generators.json, executors.json, migrations.json
  src/generators/init/     — novo (C1)
  src/generators/bounded-context/ — desacoplamento (C2, C3)
  src/executors/           — novo, um por script (D1)
  src/migrations/          — novo (G1)
  templates/ai/            — fonte dos ativos de IA (E1, E2)
  templates/init/          — fonte do que o init escreve (C1, F1)
  templates/bounded-context/ — templates do contexto, movidos de src/ (C2)
  scripts/                 — scripts dos executors e sync-versions (B3, D1)
.github/workflows/
  dmpf-go-ci.yml           — novo, workflow_call (F1)
  ci.yml                   — chama dmpf-go-ci.yml e roda external-consumer (H1)
  nx-release.yml           — job pós-release (H2)
docs/adr/060-o-dmpf-tem-consumidor-externo.md — novo (I1)
docs/guides/dmpf-consumo-externo.md           — novo (I2)
dmpf.json                  — novo, layout do platform (C1, C2)
```

**Arquivos a modificar**:

- `tools/dmpf-plugin/package.json` — remover `private`, apontar entradas para `dist` e publicar `templates/`, `scripts/` e `versions.json` pelo campo `files`.
- `tools/dmpf-plugin/src/generators/bounded-context/generator.ts` e os templates — ler `dmpf.json` e `versions.json`.
- `apps/backend/*/project.json` — trocar `bash tools/*.sh` por executors.
- `infra/observability/alloy/alloy-kubernetes.alloy` — lista gerada pelo `infrasync`.
- `.claude/agents/dmpf-context-author.md`, `.claude/rules/dmpf-bounded-context.md`, `.claude/commands/dmpf-new-context.md`, `.agents/skills/dmpf-*/**` — passam a ser saída do generator.
- `docs/adr/059-remover-o-bom-e-a-release-do-produto.md` e `docs/adr/046-libs-somente-kernel-de-reuso.md` — status apontando para a ADR-060.

## Design

### Arquitetura

```text
             platform (github.com/mateusmacedo/dmpf)
 ┌──────────────────────────────────────────────────────────────┐
 │ kernel libs ──tag──▶ proxy.golang.org                         │
 │ dmpf-conformance ──tag──▶ proxy.golang.org (go run @vX)       │
 │ dmpf-plugin ──nx release──▶ npm.pkg.github.com                │
 │   versions.json · init · bounded-context · executors          │
 │   migrations · templates/{ai,init,bounded-context} · scripts  │
 │ dmpf-go-ci.yml (workflow_call, fixado por SHA)                │
 └──────────────────────────────────────────────────────────────┘
                        │ por versão
                        ▼
             consumidor (nx-base-template e derivados)
 ┌──────────────────────────────────────────────────────────────┐
 │ .npmrc ─▶ @mateusmacedo/dmpf-plugin@X                         │
 │ dmpf.json · go.work · .golangci.yml · infra/ · .claude/       │
 │ apps/backend/<ctx>/go.mod  require kernel vK (sem replace)    │
 │ .github/workflows/dmpf-ci.yml ─uses─▶ dmpf-go-ci.yml@<sha>    │
 └──────────────────────────────────────────────────────────────┘
```

### Fluxo principal

1. **Bootstrap**: `pnpm nx add @mateusmacedo/dmpf-plugin@X` →
   `pnpm nx g @mateusmacedo/dmpf-plugin:init --modulePrefix=github.com/org/repo`.
2. **Primeiro contexto**: `pnpm nx g @mateusmacedo/dmpf-plugin:bounded-context`
   gera módulo, `contract/` e `deploy/`, roda `go get` do kernel vK,
   `modsync --write` e `infrasync --write` por `go run ...@vX`.
3. **Gates**: `dmpf-ci.yml` chama `dmpf-go-ci.yml@<sha>`; o `conformance` lê o
   kernel pelos pacotes do `go list -deps -json`.
4. **Atualização**: `pnpm nx migrate @mateusmacedo/dmpf-plugin@Y` →
   `--run-migrations` reescreve arquivos copiados e sobe `require` do kernel;
   o usuário roda os comandos de `nextSteps`.
5. **Release do platform**: `sync-versions --target` grava `versions.json`
   no commit de versão; `nx release` cria as tags Go e um job encadeado
   publica o pacote com o dist-tag derivado da versão; o job H2 prova o
   consumo.

### Pseudocódigo (A1 + A2)

```text
universo = unidades_locais(git ls-files, go.work)
para cada modulo distinto nos pacotes de go list -deps -json:
    se não modulo.Main e existe modulo.Dir/dmpf-units.json:
        manifesto = ler(modulo.Dir/dmpf-units.json)   // exit 2 se inválido
        para cada unidade em manifesto:
            universo.adicionar(unidade, papel=shared_kernel_leitura)
para cada aresta (origem local → destino):
    se destino em universo com papel shared_kernel_leitura:
        aplicar regras de dependência para shared kernel (D002 satisfeita)
    senão se destino fora do universo:
        avaliar como external (comportamento atual)
verificar apenas unidades locais
```

## Decisões técnicas

- **Âncora de versão única no pacote npm**: o pacote fixa kernel,
  conformance e ferramentas porque os templates do generator importam a API
  do kernel. Alternativa descartada: versões avulsas escolhidas pelo
  consumidor, que quebram em silêncio quando a API muda.
- **GitHub Packages**: decisão do mantenedor; mantém publicação e registry no
  mesmo host do código. Custo aceito: todo consumidor precisa de token com
  `read:packages`, inclusive para pacote público. Alternativa descartada:
  npmjs.com com trusted publishing.
- **Ativos de IA pelo plugin Nx**: um canal e uma versão; a rule mantém a
  semântica de carregamento por `paths:`. Alternativa descartada: marketplace
  de plugin Claude Code no repo, que seria um segundo canal de versão e
  transformaria a rule em skill.
- **Kernel lido pelos pacotes do `go list -deps -json`**: atende "consumo por
  versão" sem dependência nova no verificador.
  Alternativa descartada: kernel como submódulo no `go.work`, que passa no
  `conformance` hoje com o baseline ajustado à mão, mas reintroduz cópia local.
- **Scripts empacotados como executors**: menor custo, uma cópia.
  Alternativas descartadas: reescrever em Go agora (custo); copiar no template
  (divergência garantida).
- **Infra copiada pelo `init` com migrations**: o compose usa bind mounts
  relativos (`infra/local/compose/grafana.yml:19-21`, `loki.yml:12`,
  `tempo.yml:13`, `alloy.yml:16`), então os arquivos precisam estar em disco.
  O `init` escreve só o esqueleto estático; os arquivos gerados a partir dos
  contextos seguem com o `infrasync`. Alternativa descartada: `include` remoto.
- **Workflow reutilizável fixado por SHA**: ref imutável e sem ambiguidade de
  tags com `@` e `/`; as actions internas vêm do commit do próprio
  reutilizável (checkout de `job.workflow_repository` em `job.workflow_sha`),
  porque o reutilizável chamado de outro repo resolve `uses: ./...` no
  chamador, e um `@<sha>` literal não aponta para o commit que o contém.
  Alternativa descartada: copiar o `ci.yml` inteiro no consumidor.
- **Proxy `file://` no job de PR**: a combinação plugin + kernel do PR ainda
  não tem tag; o proxy serve o kernel e o `dmpf-conformance` do PR numa
  versão efêmera e exercita o caminho por tag. Alternativa descartada:
  overlay do workspace com o kernel do PR, que não exercita A1. A prova
  contra o proxy público roda depois da release (H2).
- **Ferramentas por modo**: `dmpf.json.tooling.mode` é `local` no platform
  (`go run ./tools/...`, testa mudança sem tag) e `version` no consumidor
  (`go run <módulo>@<versão>`).
- **Publicação encadeada**: o job de publicação depende do versionamento no
  `nx-release.yml`, porque a tag empurrada com o `GITHUB_TOKEN` não dispara
  outro workflow; o dist-tag deriva da versão, para que pré-release não
  caia em `latest`.

## Regras relacionadas

- `.claude/rules/dmpf-bounded-context.md` e `tools/dmpf-context-check.sh` — forma do contexto (ADR-053).
- `CONTRIBUTING.md:70-79` — subir `require` dos irmãos antes de promover pré-release.
- `AGENTS.md` — não redeclarar targets (ADR-002), nomes sem prefixo (ADR-045), escopo `@mateusmacedo/`.
- SPEC-TXJ6A4XX (repo `nx-base-template`) — consumidor desta entrega.

## Verificação e testes

### Critérios de aceite

- [ ] Consumidor com kernel por tag, sem `replace`: `conformance --root .` exit 0 e sem `DMPF-E001`/`DMPF-E002`.
- [ ] `conformance` sem `--profiles` e sem `tools/dmpf-conformance/build-profiles.json` no consumidor funciona.
- [ ] `pnpm nx add @mateusmacedo/dmpf-plugin` num workspace Nx 23.x externo lista `init` e `bounded-context`.
- [ ] `init` rodado 2 vezes seguidas: a segunda altera 0 arquivos.
- [ ] `rg 'apps/backend/bff|ghcr.io/mateusmacedo|buf.build/mateusmacedo|spiffe://dmpf' tools/dmpf-plugin/src/generators` retorna 0 linhas fora de testes.
- [ ] Nenhum template do plugin contém versão literal de ferramenta; todas vêm de `versions.json`.
- [ ] `tools/dmpf-generator-check.sh` comprova diff zero no golden `bookings`.
- [ ] Job `external-consumer` verde em PR, em até 10 min.
- [ ] Job pós-release verde contra proxy e GitHub Packages.
- [ ] `git ls-files 'tools/*.sh'` não lista os 7 scripts de D1.
- [ ] CI reprova divergência entre `tools/dmpf-plugin/templates/ai/` e `.claude/`/`.agents/` do platform.
- [ ] ADR-060 publicada; ADR-059 e ADR-046 com status atualizado.
- [ ] Testes unitários para leitura do kernel pelos pacotes do `go list -deps -json`, designação automática, perfis embutidos, `modsync` com tag `-rc.N`, `init` idempotente e migrations.
- [ ] Validação do projeto passando: `pnpm biome check .`, `pnpm nx affected -t lint,typecheck,test,build --exclude=@mateusmacedo/dmpf-source`, `conformance`, `modsync --check`.

### Cenários de teste

```text
DADO um workspace Nx vazio fora do platform com o plugin instalado
QUANDO roda init --modulePrefix=example.com/consumer e bounded-context com 1 agregado
ENTÃO go build ./..., go vet ./..., conformance (exit 0), modsync --check e
      infrasync --check passam, e os go.mod requerem o kernel por tag sem replace

DADO um consumidor já inicializado com o plugin na versão X
QUANDO roda init de novo com as mesmas opções
ENTÃO nenhum arquivo é alterado

DADO um consumidor na versão X com um contexto gerado
QUANDO roda nx migrate para a versão Y e --run-migrations
ENTÃO infra, .golangci.yml, ativos de IA e chamador de CI ficam idênticos à
      saída de init na versão Y e os require do kernel sobem para a tag de Y

DADO um consumidor cujo dmpf.json tem modulePrefix example.com/change-me
QUANDO roda bounded-context
ENTÃO o generator falha antes de escrever qualquer arquivo e imprime o comando init

DADO um módulo do kernel no cache com dmpf-units.json inválido
QUANDO roda conformance
ENTÃO sai com exit 2 citando <módulo>@<versão>/dmpf-units.json

DADO o job external-consumer sem acesso à rede para o proxy
QUANDO o kernel não está no cache
ENTÃO conformance sai com exit 2 e NAO VERIFICADO, nunca exit 0
```

<critical_constraints>
- [P0] Consumo só por versão: o consumidor não tem `replace` para módulos do
  kernel, caminho relativo ao platform nem arquivo copiado à mão.
- [P0] Fonte única por ativo: scripts, infra, ativos de IA e `.golangci.yml`
  têm uma fonte canônica neste repo; o pacote e o próprio platform consomem a
  mesma fonte, e o CI reprova divergência.
- [P0] O platform continua verde, e o golden `bookings` regenerado sai com
  diff zero.
- [P0] Registry npm é o GitHub Packages; token só por variável de ambiente
  ou secret.
- [P1] Uma versão do plugin fixa as versões do kernel, do `dmpf-conformance` e
  das ferramentas.
- [P1] BOM e certificação de produto continuam fora.
- [P1] Targets novos não redeclaram o que a inferência já fornece (ADR-002).
</critical_constraints>

## Escopo fora

- **BOM e certificação de produto**: a ADR-059 segue valendo nesse ponto; a
  prova de consumo é um job de CI.
- **Reescrever scripts em Go**: os scripts entram como executors empacotados;
  a reescrita, se vier, é outra spec.
- **Marketplace de plugin Claude Code e npmjs.com**: descartados nas decisões
  técnicas.
- **Mudanças no `nx-base-template`**: pertencem à SPEC-TXJ6A4XX.
- **App de exemplo, BFF e borda REST no consumidor, suíte de carga k6**: fora
  de escopo das duas specs; a referência viva continua sendo `bookings`.
- **Migração de projetos existentes**: a entrega vale só para projetos novos.
