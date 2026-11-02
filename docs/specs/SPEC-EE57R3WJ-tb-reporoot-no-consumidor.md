---
id: SPEC-EE57R3WJ
slug: tb-reporoot-no-consumidor
title: DMPF — tb.RepoRoot resolve a raiz do workspace consumidor, e não a do cache de módulos
stage: backlog
priority: P1
depends_on: []
ticket_url: https://linear.app/mmda/issue/DEVS-158
subtask_urls: []
created: 2026-10-23
---

# SPEC-EE57R3WJ: tb.RepoRoot resolve a raiz do workspace consumidor

## Resumo

Fazer o `tb.RepoRoot` do testkit achar o `go.work` a partir do diretório do package em
teste, e não do caminho do próprio `tb.go`. Hoje, em qualquer workspace que consome o
DMPF por versão, o `tb.go` mora no cache de módulos e não há `go.work` acima dele: o
`RepoRoot` aborta com `tb.RepoRoot: no go.work above …`, e com ele caem
`tb.ReadFixture`, `tb.LoadProjection`, `GoldenSuite` e `UpdateGolden`. Como autor de
bounded context num consumidor, quero que os testes de projeção e as suítes golden do
contrato rodem como no golden `bookings`, sem precisar de contorno local.

Origem: consumidor externo `marketplace` (DEVS-144), primeiro contexto `tenancy` gerado
pelo `@mateusmacedo/dmpf-plugin` 1.0.0-rc.7. O agente `dmpf-context-author` reproduziu a
falha com um teste de sonda. O contorno aplicado lá foi a fixture de projeção inline no
`projection_test.go`, e os testes de `contract/golden` ficaram por escrever.

## Contexto

- **Problema**: `RepoRoot` (`libs/backend/go/testkit/tb/tb.go:16-33`, commit `ddd3d0c4`)
  sobe diretórios a partir de `runtime.Caller(0)`, que é o arquivo `tb.go`. No monorepo
  do DMPF, o testkit é módulo do workspace e a subida acha o `go.work` da raiz. No
  consumidor, o testkit vem do proxy (`require …/testkit v1.0.0-rc.7`). O arquivo fica
  em `$GOMODCACHE/github.com/mateusmacedo/dmpf/libs/backend/go/testkit@v1.0.0-rc.7/tb/tb.go`,
  sem `go.work` acima, e todo helper que lê fixture aborta o teste.
- **Impacto**: os consumidores passam a usar `tb.LoadProjection` e as suítes golden do
  contrato sem contorno. Essas são as formas que a skill `dmpf-testkit` prescreve
  (`package-routing.md__tmpl__:44`) e que os goldens `orders`, `reservations` e
  `bookings` usam (`domain/projection_test.go`, `contract/golden/golden_test.go`). Com a
  correção, `Workspace(tb.RepoRoot(t), "")` (`package-routing.md__tmpl__:221`) também
  passa a valer no consumidor.
- **Por que escapou**: a prova do consumidor externo (`tools/external-consumer.sh`) roda
  `go test` no consumidor, mas o contexto `shop` que o template gera não chama nenhum
  helper de fixture. Nenhum teste do kit roda com o `tb.go` fora do workspace.
- **Garantia de raiz no consumidor**: o `init` do plugin sempre escreve o `go.work` na
  raiz do workspace consumidor (`tools/dmpf-plugin/src/migrations/upgrade.spec.ts:27`).
  O `go test` roda cada package com o diretório de trabalho no diretório do package, que
  fica abaixo dessa raiz.
- **Links relevantes**:
  - SPEC-SJ66880S: origem do módulo `testkit` e dos kits golden.
  - SPEC-P846PH1P: consumo por projeto externo e prova de consumo.
  - `docs/adr/040-test-kits-golden-e-fitness-function-em-go.md`: fixtures de projeção
    por contexto, relativas à raiz.
  - `tools/external-consumer.sh`: prova do consumidor externo (job `external-consumer`
    do `ci.yml`, PR de release para `master`).

<constraints>
- [P0] A raiz DEVE ser achada a partir do diretório de trabalho do teste (`os.Getwd()`), subindo até o primeiro diretório com `go.work`
- [P0] A assinatura pública de `RepoRoot`, `ReadFixture`, `LoadProjection`, `GoldenSuite` e `UpdateGolden` NÃO muda: o consumidor sobe de versão sem editar teste
- [P0] No monorepo do DMPF, `RepoRoot` DEVE devolver o mesmo diretório de hoje (a raiz com o `go.work`), para todo package que o chama hoje
- [P0] `ReadFixture` mantém a recusa de caminho que sai da raiz (`..`, absoluto)
- [P1] A falha, quando nenhuma raiz é achada, DEVE nomear os pontos de partida tentados
- [P1] O kit NÃO executa o binário `go` para achar a raiz: sem subprocesso no caminho do teste
</constraints>

## Requisitos

### Funcionais

- [ ] **[P0] Raiz pelo diretório do teste**: `RepoRoot` sobe a partir de `os.Getwd()` e
  devolve o primeiro ancestral, inclusive o próprio diretório, que contém `go.work`.
  - Edge case: `go.work` no próprio diretório de trabalho devolve esse diretório.
  - Edge case: `os.Getwd()` com erro passa direto para o fallback.
- [ ] **[P1] Fallback pelo arquivo do kit**: sem `go.work` acima do diretório de
  trabalho, `RepoRoot` tenta a subida atual, a partir de `runtime.Caller(0)`. Isso cobre o
  binário de teste executado fora do diretório do package (`go test -c` rodado de outro
  lugar).
- [ ] **[P1] Falha nomeada**: sem raiz pelas duas subidas, `t.Fatalf` com
  `tb.RepoRoot: no go.work above <diretório de trabalho> nor <arquivo do kit>`.
- [ ] **[P0] Regressão no kit**: teste do package `tb` prova que a raiz vem do diretório
  de trabalho. Ele cria um workspace temporário com `go.work` e uma fixture, muda para um
  subdiretório (`t.Chdir`) e verifica que `RepoRoot` e `ReadFixture` resolvem nesse
  workspace, e não no monorepo.
- [ ] **[P0] Regressão no consumidor**: `tools/external-consumer.sh` grava no contexto
  `shop` gerado um teste de sonda que chama `tb.ReadFixture` sobre um arquivo gravado
  sob a raiz do consumidor. Ele confere o conteúdo lido e que `tb.RepoRoot` devolve a
  raiz do consumidor. O `go test` que o script já roda executa a sonda contra o kernel
  do PR.
- [ ] **[P2] Armadilha documentada**: nova entrada em
  `tools/dmpf-plugin/templates/ai/.agents/skills/dmpf-bounded-context/references/armadilhas.md__tmpl__`.
  Ela registra que o caminho de fixture é relativo à raiz do workspace consumidor (o
  diretório do `go.work`), que o kernel até 1.0.0-rc.7 aborta com `no go.work above`
  fora do monorepo e qual versão corrige.
- [ ] **[P2] Godoc**: o comentário de `RepoRoot` descreve a nova ordem de busca.

### Não-funcionais

- [ ] Compatibilidade: Go da `go.work` do repositório (1.26.x); `t.Chdir` disponível
  desde o Go 1.24.
- [ ] Custo: no máximo um `os.Getwd()` e uma subida de diretórios por chamada, sem
  subprocesso, rede ou variável de ambiente nova.
- [ ] Conformidade: o verificador DMPF, o `modsync --check` e os gates do testkit seguem
  verdes.

## Camadas afetadas

| Camada | Afetada? | Descrição |
|--------|----------|-----------|
| Kernel `testkit` (`layer:providers`) | [x] | `tb.RepoRoot` muda a ordem de busca; teste de regressão no package `tb` |
| Tooling `dmpf-plugin` (templates de IA) | [x] | Entrada nova nas armadilhas do bounded context |
| Prova do consumidor externo (`tools/`) | [x] | Sonda de fixture no contexto `shop` gerado |
| Apps (`bookings`, `orders`, `reservations`) | [ ] | Sem mudança: continuam chamando os mesmos helpers |
| Generator `bounded-context` | [ ] | Sem mudança no código gerado |

## Localização de código

```text
libs/backend/go/testkit/tb/
  tb.go          — RepoRoot: busca pelo diretório de trabalho, fallback pelo arquivo do kit
  tb_test.go     — regressão com workspace temporário e t.Chdir
tools/
  external-consumer.sh — sonda de fixture no consumidor
tools/dmpf-plugin/templates/ai/.agents/skills/dmpf-bounded-context/references/
  armadilhas.md__tmpl__ — entrada nova
```

**Arquivos a modificar**:
- `libs/backend/go/testkit/tb/tb.go`: `RepoRoot` passa a delegar a uma função não
  exportada que recebe os pontos de partida, para o teste cobrir o caminho de falha sem
  depender do monorepo.
- `libs/backend/go/testkit/tb/tb_test.go` (ou `root_internal_test.go` no package `tb`):
  casos do diretório de trabalho, do fallback e da falha.
- `tools/external-consumer.sh`: grava a fixture e o teste de sonda antes do passo
  "gates do consumidor".
- `tools/dmpf-plugin/templates/ai/.agents/skills/dmpf-bounded-context/references/armadilhas.md__tmpl__`:
  entrada nova, numerada em sequência.

## Design

### Arquitetura

`RepoRoot(t)` monta a lista de partidas, `[os.Getwd(), dir(runtime.Caller(0))]`, e
chama `findRoot(partidas)`. `findRoot` sobe cada partida até a raiz do sistema de
arquivos e devolve o primeiro diretório com `go.work`. Se nenhuma partida acha um, a
função devolve erro com as partidas. `RepoRoot` converte o erro em `t.Fatalf`.
`ReadFixture`, `LoadProjection` e `UpdateGolden` não mudam: continuam chamando
`RepoRoot`.

### Fluxo principal

1. O teste do consumidor chama `tb.LoadProjection(t, "apps/backend/<ctx>/contract/fixtures/projection/v1/x.golden")`.
2. `ReadFixture` valida o caminho relativo e chama `RepoRoot`.
3. `RepoRoot` sobe a partir de `$CONSUMER/apps/backend/<ctx>/domain` e acha `$CONSUMER/go.work`.
4. `ReadFixture` lê `$CONSUMER/apps/backend/<ctx>/contract/fixtures/projection/v1/x.golden`.

### Pseudocódigo

```text
RepoRoot(t):
  partidas = []
  if wd, err = Getwd(); err == nil: partidas += wd
  if _, file, ok = Caller(0); ok: partidas += Dir(file)
  root, err = findRoot(partidas)
  if err: t.Fatalf("tb.RepoRoot: no go.work above %s", join(partidas, " nor "))
  return root

findRoot(partidas):
  for p in partidas:
    for dir = p; ; dir = Parent(dir):
      if exists(dir/go.work): return dir
      if Parent(dir) == dir: break
  return erro
```

## Decisões técnicas

- **Diretório de trabalho primeiro**: o `go test` fixa o diretório de trabalho no
  package em teste, e esse diretório está sempre dentro do workspace de quem roda o
  teste, seja o monorepo ou o consumidor.
  Alternativa descartada: `go env GOWORK`, porque abre subprocesso a cada chamada e
  depende do binário `go` no `PATH` do teste.
- **Fallback pelo arquivo do kit mantido**: preserva o comportamento atual quando o
  binário de teste roda de fora do package. O custo é uma segunda subida, só quando a
  primeira falha.
  Alternativa descartada: remover o `runtime.Caller`, porque tiraria um caminho que hoje
  funciona no monorepo sem ganho para o consumidor.
- **`findRoot` não exportado com partidas como parâmetro**: torna o caminho de falha
  testável. No monorepo o fallback sempre acha o `go.work`, então só uma função com
  partidas injetadas prova a mensagem de falha.
- **Sonda no `external-consumer.sh`, e não no template do generator**: a sonda prova o
  kit contra o kernel do PR sem mudar o código gerado para todo consumidor.

## Regras relacionadas

- `AGENTS.md`: commits separados por projeto (`testkit`, `dmpf-plugin`); `tools/` fora
  de `dmpf-plugin` segue o escopo do projeto dono do script.
- `.claude/rules/dmpf-bounded-context.md`: forma do contexto (sem mudança).
- ADR-040: fixtures relativas à raiz do repositório.

## Verificação e testes

### Critérios de aceite

- [ ] No monorepo, `TestReadFixtureResolvesFromTheRepoRoot` e todos os testes que usam
  `RepoRoot` (`fitness/*_test.go`, `tb/*_test.go`, goldens das 3 apps) passam sem
  alteração.
- [ ] Novo teste no package `tb`: com `t.Chdir` num subdiretório de um workspace
  temporário com `go.work`, `RepoRoot` devolve a raiz temporária e `ReadFixture` lê a
  fixture dela.
- [ ] Novo teste de `findRoot`: partidas sem `go.work` acima devolvem erro que contém
  todas as partidas.
- [ ] Novo teste de `findRoot`: a primeira partida sem `go.work` e a segunda com
  `go.work` devolvem a raiz da segunda.
- [ ] `bash tools/external-consumer.sh` passa com a sonda. A sonda reprova quando
  executada contra o kernel 1.0.0-rc.7, o que comprova que pega a regressão.
- [ ] Entrada nova em `armadilhas.md__tmpl__` com a versão corrigida.
- [ ] `pnpm nx run testkit:lint`, `testkit:test`, `testkit:test-race`,
  `testkit:fmt-check` e `testkit:vet` passam; o verificador DMPF e o `modsync --check`
  passam.

### Cenários de teste

```text
DADO um workspace temporário W com go.work e a fixture W/fx/a.golden
QUANDO o teste faz t.Chdir(W/apps/x) e chama tb.ReadFixture(t, "fx/a.golden")
ENTÃO o conteúdo lido é o de W/fx/a.golden e tb.RepoRoot(t) == W

DADO o workspace do monorepo do DMPF
QUANDO testkit/tb roda TestReadFixtureResolvesFromTheRepoRoot
ENTÃO RepoRoot devolve a raiz do monorepo, como antes da mudança

DADO as partidas [A, B], nenhuma com go.work acima
QUANDO findRoot([A, B]) roda
ENTÃO devolve erro que contém A e B, e RepoRoot falha com "no go.work above A nor B"

DADO o consumidor gerado pelo external-consumer.sh com o contexto shop
QUANDO go test roda a sonda que lê uma fixture sob a raiz do consumidor
ENTÃO a sonda passa com o kernel do PR e falha com o kernel 1.0.0-rc.7
```

<critical_constraints>
- [P0] A raiz DEVE ser achada a partir do diretório de trabalho do teste (`os.Getwd()`), subindo até o primeiro diretório com `go.work`
- [P0] A assinatura pública de `RepoRoot`, `ReadFixture`, `LoadProjection`, `GoldenSuite` e `UpdateGolden` NÃO muda: o consumidor sobe de versão sem editar teste
- [P0] No monorepo do DMPF, `RepoRoot` DEVE devolver o mesmo diretório de hoje (a raiz com o `go.work`), para todo package que o chama hoje
- [P0] `ReadFixture` mantém a recusa de caminho que sai da raiz (`..`, absoluto)
- [P1] A falha, quando nenhuma raiz é achada, DEVE nomear os pontos de partida tentados
- [P1] O kit NÃO executa o binário `go` para achar a raiz: sem subprocesso no caminho do teste
</critical_constraints>

## Escopo fora

- **Consumidor sem `go.work` (`GOWORK=off`, módulo único)**: o `init` sempre escreve o
  `go.work` e a forma do contexto depende dele (armadilha 3). Suportar a raiz por
  `go.mod` é decisão de outro escopo.
- **Gerar `projection_test.go` com `tb.LoadProjection` no template do generator**: muda
  o código gerado para todo consumidor e pertence à evolução do generator, não à
  correção do kit.
- **Corrigir o contexto `tenancy` do marketplace**: o consumidor ajusta o próprio código
  ao subir de versão; a sessão do marketplace declarou que não precisa de mudança lá.
- **Release que publica a correção**: segue o fluxo de release normal (`CONTRIBUTING.md`),
  fora desta spec.
