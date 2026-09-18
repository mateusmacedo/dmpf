# ADR-047: Separar a tag de módulo Go da tag de release do produto e abrir o consumo dos módulos fora do workspace

## Status

Aceito — 2026-09-17. Supersede parcialmente o ADR-041 (2026-09-13): o `go.mod` de um módulo deixa de ser workspace-only e passa a declarar `require` versionado dos irmãos que importa. Resolve o tema que o ADR-034 adiou para a task sucessora ARQ-550 (`KRN-14`). Mantém do ADR-030 o `package.json` privado por módulo Go, sem o qual o Nx Release aborta o versionamento de um `tag:type:lib`.

## Contexto

O repositório tinha uma linha só de tag. O `nx.json` declarava `release.projects: tag:type:lib` e o padrão global `{projectName}@{version}`, e a tag do produto — `dmpf@X.Y.Z`, que certifica um BOM e a sua evidência — era cunhada à mão pelo rito do `bom/README.md`. Os dois contextos competiam pelo mesmo espaço de nomes sem que nenhum servisse ao Go: para um módulo aninhado, o toolchain só reconhece a tag `<diretório>/v<semver>` (`libs/backend/go/domain/v0.1.0`), forma que `{projectName}@{version}` não produz.

O consumo fora do workspace estava fechado pela outra ponta. Pelo ADR-041, o módulo gerado nascia workspace-only: `go.mod` com `module` e `go`, sem `require`, e a resolução inteira delegada ao `go.work` da raiz. Dentro do repositório isso funciona; fora, não há o que resolver. O spike de 2026-09-15 mediu as duas falhas: um consumidor externo que declara `require` de um módulo do kernel sem tag publicada não compila, e um clone local só compila se o próprio consumidor montar um `go.work` cobrindo o módulo e todos os irmãos que ele alcança.

Só a tag `dmpf@0.1.0` existia; nenhum módulo Go tinha tag própria. Nada de versionamento se quebra ao mudar o esquema agora.

A mecânica adotada é a que o repositório `golibs` da organização já usa: versão no `package.json` privado, `require` versionado dos irmãos no `go.mod` e `replace` versionado só no `go.work`.

## Decisão

**Duas linhas de tag, três release groups.** `release.projects` sai do `nx.json` e dá lugar a `groups`: `go-libs` seleciona por diretório menos as apps (`directory:libs/backend/go/*` e `!tag:type:app`, as 14 libs do kernel) e cunha `libs/backend/go/{projectName}/v{version}`; `go-tools` cobre o `conformance` com o padrão literal `tools/dmpf-conformance/v{version}`, porque o diretório não casa o nome do projeto; `npm` fica com `tag:type:lib` menos `tag:stack:go` e mantém `{projectName}@{version}`. Os três usam as version actions do `@nx/js` e herdam `conventionalCommits`, `git-tag` e `updateDependents` do `version` da raiz — declarar o resolver ao lado do atalho é erro de config no Nx (`CONVENTIONAL_COMMITS_SHORTHAND_MIXED_WITH_OVERLAPPING_OPTIONS`). O seletor por diretório casa lib nova sozinha, sem editar o `nx.json`. Nenhum `go.mod` é tocado pelo release: a versão vive no `package.json` privado.

**O `go.mod` declara o que o módulo importa.** Cada `go.mod` traz `require` versionado dos irmãos alcançados pelos seus imports; o `replace` que aponta para o diretório local vive só no `go.work` da raiz, versionado (`… v0.1.0 => ./libs/backend/go/domain`). Assim o mesmo arquivo serve aos dois consumos: dentro do workspace o `replace` vence, fora dele o `require` resolve pela tag publicada. O dono desse estado é o `dmpf-modsync` (`tools/dmpf-conformance/cmd/modsync`), que deriva os `require` esperados dos imports reais, grava com `--write` e reprova com `--check` — gate fail-closed no `ci.yml`.

**A tag do produto vira workflow.** `dmpf-release.yml` (`workflow_dispatch`, input `release`) substitui o rito manual: guarda de `master`, `bom/dmpf/<semver>.json` presente, `dmpf-bom --release <semver> --commit HEAD` conforme, evidência regerada por `workflow_call` do `dmpf-evidence.yml` e idêntica à publicada, `dmpf@<semver>` ainda inexistente. Só então a tag anotada é cunhada e empurrada sozinha.

**`DMPF-B012` amarra o BOM às tags Go.** Para toda entrada `subject: kernel` do BOM que não esteja `rejeitada`, a regra exige que a tag `<diretório do módulo>/v<versão>` exista e seja ancestral do commit da release. O diretório sai do `module` do `go.mod` na árvore, nunca do `nx.json`. O pacote `bom` não fala com git: recebe a ancestralidade por `Input.Ancestry`, que o `cmd/bom` injeta a partir de `--commit`; `nil` desliga a regra, como `Base == nil` já desligava o `DMPF-B003`. O corte é `≥ 0.2.0`, porque o BOM `0.1.0` é imutável e nasceu sem tags Go.

**O generator fecha o ciclo.** `bounded-context` devolve um `GeneratorCallback` que roda `go run ./tools/dmpf-conformance/cmd/modsync --root . --write` a partir da raiz do workspace: só depois do flush em disco o módulo novo existe para o modsync, que lê `go.mod` e `go.work`, não a Tree. O contexto gerado é `type:app` e não entra em release group — e uma lib nova em `libs/backend/go/` seria casada pelo seletor de diretório —, então o generator nunca escreve no `nx.json`. Quem garante isso é o `!tag:type:app` do grupo, não o generator: `--directory` aceita qualquer caminho relativo, e sem a exclusão um contexto criado fora da convenção seria versionado como lib.

**A cascata de versão pelo grafo bruto é aceita.** O Nx mapeia commit a projeto pelo grafo de dependências sem distinguir aresta de teste, de modo que um commit no `testkit` sobe a versão de quase todos os módulos. Removê-la exigiria tirar as arestas de teste do grafo, o que quebraria o `nx affected`.

## Alternativas descartadas

- **Um release group por lib, com padrão literal em cada.** Era a escolha original, necessária apenas enquanto o nome do projeto divergia do diretório. O ADR-045 alinhou os dois e um grupo único com `directory:` passou a bastar.
- **Renomear `tools/dmpf-conformance` para `tools/conformance`**, casando `{projectName}` e dispensando o grupo `go-tools`. O module path é chave canônica e estável pela RFC, e o ADR-046 já o regravou uma vez.
- **`VersionActions` Go próprio** (ou `@naxodev/gonx`, ou um script pós-`nx release`) para escrever a versão no `go.mod`. Código a mais sem ganho: o `package.json` privado permanece e é dele que o Nx lê a versão corrente.
- **`replace` relativo no `go.mod` com `tidy-check` sob `GOWORK=off`.** Foi o desenho do spike; publica um `go.mod` que só compila dentro do repositório.
- **Reescrever os `require` a cada release.** Manteria os irmãos sempre na última versão, ao custo de um `VersionActions` próprio. Os `require` ficam na versão mínima, como no `golibs`.
- **`DMPF-B012` retroativo.** Reprovaria o BOM `0.1.0`, cujo kernel está em `0.0.0` e sem tag, e que é registro imutável de `8e171d31`.
- **Manter o rito manual da tag do produto, com gate só no `dmpf-bom`.** Deixa um passo humano entre a validação e a tag.

## Consequências

- `release.docker` saiu da raiz do `nx.json`. Na raiz, a config de Docker alcança **todo** release group e o Nx força `releaseTag.requireSemver = false` por atribuição direta; com isso `extractTagAndVersion` leria `domain` como versão de `libs/backend/go/domain/v0.1.0`. O defeito era pré-existente e nunca apareceu porque não houve segunda release de lib. Quando existir app não-Go, ela declara `docker` no próprio grupo.
- `nx-release.yml` detecta a primeira release por família — a tag `dmpf@0.1.0` casava o glob `*@*` e teria impedido `--first-release` na estreia das libs — e empurra só o delta de tags da execução, com guarda que aborta se uma `dmpf@*` aparecer no delta.
- `nx-publish-libs.yml` dispara em `["**@*", "!dmpf@*"]`: as tags de módulo Go não casam `**@*` e por construção nunca acionam a publicação npm.
- Um módulo que passe a usar API nova de um irmão sem subir o `require` compila dentro do workspace e entrega ao consumidor externo a versão antiga. Mitigação de processo: subir o `require` no mesmo PR.
- A estreia de cada família leva a versão explícita `0.1.0` no `nx-release.yml`, porque sem tag o resolver cai no fallback `disk` (`0.0.0` do `package.json`) e o specifier de conventional commits decidiria por projeto. A guarda é **por família, não por projeto**: um módulo que nasça depois, já com a família tagueada, entra pela resolução automática a partir de `0.0.0` e pode estrear fora da numeração dos irmãos (`0.0.1`, se o primeiro commit for `fix`). Quem criar um módulo nessas condições ajusta a versão no `package.json` antes da release.
- O grupo `go-libs` seleciona `directory:libs/backend/go/*` **menos** `tag:type:app`. O seletor de diretório não filtra por tag, e `--directory` do generator não recusa `libs/backend/go`: sem a exclusão, um bounded context criado fora da convenção do ADR-046 seria versionado como lib do kernel. Hoje a exclusão é inócua — os 14 módulos do kernel não são `type:app`.
- `dmpf@0.2.0` é o primeiro BOM sob o `DMPF-B012`; `bom/dmpf/0.1.0.json` e a sua evidência permanecem como estão.
- Criar um bounded context passa a executar um `go run` ao final do generator. O comando falhando derruba o `nx g` — fail-closed, deliberado.

## Referências

- ADR-030, ADR-034, ADR-041, ADR-043, ADR-045 e ADR-046.
- SPEC-95AHV4D4 (`KRN-14` / ARQ-550) e `bom/README.md` (rito da release).
