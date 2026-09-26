# Referência: Configuração de Tasks no NX

Guia de referência para configurar tasks, cache e pipelines neste workspace.
Baseado na documentação oficial do NX v23 — a versão fixada no `catalog:` deste
workspace.

---

## Modelo mental

```
Plugin (inferido)  →  targetDefaults (nx.json)  →  project.json / package.json
    [prioridade 1]          [prioridade 2]                  [prioridade 3]
       (mais baixa)                                            (mais alta)
```

**Regra prática:** coloque o máximo possível em `targetDefaults`. Use `project.json`
apenas para *exceções* ou opções que variam por projeto (ex: `passWithNoTests`).

---

## O que já está em targetDefaults (não repetir em project.json)

| Target | O que o default já cobre |
|--------|--------------------------|
| `build` | `dependsOn: ["^build"]`, `cache: true` |
| `test` | `dependsOn: ["^build"]`, `cache: true` |
| `lint` | `executor`, `command: biome lint`, `cache: true`, `inputs` |
| `typecheck` | `dependsOn: ["^typecheck"]`, `cache: true` |
| `e2e` | `cache: true` |

### O que o plugin `@nx/js/typescript` já infere automaticamente

Para targets `build` e `typecheck`, o plugin lê o `tsconfig.lib.json` e infere:

- `inputs` — arquivos `.ts` do projeto, excluindo spec/test
- `outputs` — `{projectRoot}/dist/**/*.d.ts`, `tsbuildinfo`
- `executor` — `nx:run-commands` (tsc via tsconfig references)

**Não declare esses campos em `targetDefaults` nem em `project.json`** — você
sobrescreveria informações mais precisas que o plugin derivou do tsconfig.

### O que o plugin `@nx/jest/plugin` já infere automaticamente

Para o target `test`, o plugin lê o `jest.config.cts` e infere:

- `inputs` — inclui `jest.preset.js` e dependências externas (`jest`, `ts-jest`, `@swc/jest`)
- `outputs` — `{projectRoot}/test-output/jest/coverage`

---

## O que colocar em project.json

Apenas overrides específicos do projeto. Todo projeto **deve** declarar `"lint": {}`
para que o `targetDefault` seja aplicado (`targetDefaults` estende targets existentes,
não cria novos):

```json
{
  "name": "@mateusmacedo/minha-lib",
  "$schema": "../../../node_modules/nx/schemas/project-schema.json",
  "sourceRoot": "libs/minha-lib/src",
  "projectType": "library",
  "tags": ["type:lib", "scope:shared", "stack:node"],
  "targets": {
    "lint": {},
    "test": {
      "options": { "passWithNoTests": true }
    }
  }
}
```

**`"lint": {}`** — target vazio: o `targetDefault` preenche `executor`, `options`,
`cache` e `inputs` automaticamente. Nunca declare o `executor` ou `command` aqui —
isso sobrescreve as propriedades herdadas do default, incluindo `cache: true`.

---

## Convenção de tags (obrigatórias em todo projeto)

Todo projeto deve ter **exatamente três tags**, uma de cada dimensão:

Esta tabela é a fonte canônica da taxonomia; o `AGENTS.md` e o `README.md`
apontam para cá.

| Dimensão | Valores válidos | Exemplo |
|----------|----------------|---------|
| `type:` | `lib`, `app`, `e2e` | `type:lib` |
| `scope:` | `shared`, `backend`, `frontend` | `scope:shared` |
| `stack:` | `node`, `express`, `fastify`, `nest`, `next`, `react`, `angular`, `go`, `universal` | `stack:node` |

### Significado de `stack:`

| Valor | Quando usar |
|-------|-------------|
| `node` | Usa APIs de Node.js (fs, http, process) sem se prender a um framework |
| `express` / `fastify` / `nest` | Projeto acoplado ao respectivo framework de servidor |
| `next` | App ou lib Next.js |
| `react` | Lib com componentes React ou hooks |
| `angular` | Lib com módulos/componentes Angular |
| `go` | Projeto Go, gerenciado via `go.work` / `@nx-go/nx-go` |
| `universal` | Sem dependência de runtime (tipos puros, utilitários de string/data) |

Libs sem dependência de runtime usam `stack:universal`; as que tocam APIs de Node ou frameworks de servidor usam `stack:node`. `scope:` acompanha a pasta de primeiro
nível sob `libs/` ou `apps/` (`shared`, `backend`, `frontend`).

### A quarta tag: `layer:` em módulos Go

Todo projeto `stack:go` declara **também** uma tag `layer:`, aditiva às três
acima. Ela não altera `release.projects` nem os `targetDefaults` — a taxonomia
canônica continua sendo a de três dimensões. O que ela decide é o **estágio do
CI** em que o módulo roda: o `ci.yml` seleciona os projetos de cada estágio da
pirâmide de testes por essa tag. Um módulo `stack:go` sem ela não entra em
nenhum estágio e passaria despercebido, então o CI tem um guard que compara os
dois conjuntos e reprova quando algum projeto Go fica de fora
(`.github/workflows/ci.yml`).

| Valor | Estágio |
|-------|---------|
| `layer:domain` | Domínio e portas — sem infraestrutura |
| `layer:services` | Casos de uso e primitivas de transporte |
| `layer:contract` | Contratos de wire |
| `layer:providers` | Realizações que exigem infraestrutura (Postgres, Kafka, SQS) |
| `layer:apps` | Composition roots, bordas e os bounded contexts de `apps/backend` |

A camada é a do **estágio em que o módulo precisa rodar**, não a do bloco DMPF
de cada unidade que ele declara. Um módulo com unidades em quatro blocos, cujas
suítes exigem Postgres, é `layer:providers` — quem manda é a infraestrutura que
o teste pede.

```bash
# Os projetos de um estágio, como o ci.yml os seleciona
pnpm nx show projects --projects=tag:layer:domain --json | jq -r 'join(",")'
```

Um bounded context é **um** módulo em `apps/backend/<ctx>` (`type:app`), com um
package por bloco, e recebe a camada do seu bloco mais alto: `layer:apps` quando
tem `app`, `layer:providers` quando para no `provider`, e assim por diante. O
generator `bounded-context` já emite a tag correta no `project.json`; ver
`docs/guides/dmpf-composicao.md`. O módulo `conformance`
(`tools/dmpf-conformance`) é a exceção deliberada: tooling de workspace, sem
`layer:*`, excluído do guard e rodado em step próprio do `ci.yml`.

---

## Como criar uma nova lib

### Via generator Nx (recomendado)

```bash
# lib compartilhada (TypeScript puro, buildable via tsc)
pnpm nx g @nx/js:lib libs/shared/<name> \
  --importPath=@mateusmacedo/shared-<name> \
  --bundler=tsc --unitTestRunner=jest --linter=none \
  --tags=type:lib,scope:shared,stack:node

# lib backend (NestJS)
pnpm nx g @nx/nest:lib libs/backend/<name> \
  --importPath=@mateusmacedo/backend-<name> \
  --unitTestRunner=jest --linter=none \
  --tags=type:lib,scope:backend,stack:node
```

Cria `libs/<scope>/<name>/` com `tsconfig`, `package.json`, `src/index.ts`,
`.spec.swcrc` próprio e as tags 3D já preenchidas. A variante Nest adiciona
`jest.config.cts` e o módulo inicial. Depois rode `pnpm nx sync` para atualizar as
TypeScript Project References da raiz.

`--linter=none` é deliberado: o lint deste workspace é o Biome, configurado em
`targetDefaults` (ver `nx.json`), não um linter por projeto.

`tools/generators/` fica vazio de propósito: é o destino de generators próprios
do workspace, quando houver. Registre a coleção em `nx.generators` do
`package.json` ao criar o primeiro.

### Lib Go

```bash
pnpm nx g @nx-go/nx-go:library libs/backend/go/<name> \
  --tags=type:lib,scope:backend,stack:go \
  --skipFormat
```

`--skipFormat` não é opcional: sem ele o generator chama `formatFiles()`, que
roda Prettier com largura 80 e reformata o `nx.json` inteiro — e o `biome ci`
do CI, que usa largura 100, reprova em seguida.

Módulos Go ficam em `libs/<scope>/go/<name>` e o nome do projeto Nx, do pacote
npm privado e do package Go raiz é o basename do diretório, **sem sufixo nem
prefixo** (`docs/adr/045-nomes-bare-e-contexto-em-modulo-unico.md`): a
contraparte TypeScript, quando existir, vive em `libs/<scope>/ts/`, então o nome
não colide no Nx. Contextos de negócio não são libs, e sim apps em
`apps/backend/<contexto>` (`docs/adr/046-libs-somente-kernel-de-reuso.md`).

Go é `scope:backend` neste workspace: os quinze módulos do kernel vivem em
`libs/backend/go/`. O módulo `contracts` (projeto `contracts`) é o
único com targets além da cadeia Go — `buf-lint`, `buf-pins`,
`buf-generate-check` e `buf-breaking` chamam os subcomandos de
`tools/buf-gate.sh`, `buf-gate-selftest` roda
`tools/tests/buf-gate/buf-gate.test.sh`, e `buf-warmup` compila a CLI e o
plugin uma vez antes dos três mais pesados (`dependsOn`); todos têm
`contracts/**` nos `inputs`,
para que uma mudança só em `.proto` torne o módulo afetado (o gerado vive em
`gen/go/` dentro do módulo, mas a fonte vive em `contracts/`, na raiz).

O comando acima **não** passa `--name`: o generator deriva do diretório o nome do
projeto Nx e o `package` Go (`src/utils/normalize-options.js:18`), e esse nome já
é o correto.

Depois de gerar, quatro ajustes que o generator não faz:

1. **Module path** — o generator emite `module libs/backend/go/<name>` literal,
   que não resolve em consumo remoto. Corrija para o import path do repositório no GitHub (a flag `-C` é
   obrigatória: na raiz do workspace `GOMOD` aponta para `/dev/null` e o
   `go mod edit` falha):

   ```bash
   go -C libs/backend/go/<name> mod edit \
     -module github.com/mateusmacedo/dmpf/libs/backend/go/<name>
   go -C libs/backend/go/<name> mod edit -go=1.26.6   # o generator descarta o patch
   ```

2. **`package.json` privado** — `{"name": "@mateusmacedo/<name>", "version": "0.0.0", "private": true}`.
   Sem ele o `nx release` **aborta** o versionamento do projeto: o `@nx/js` só
   reconhece `package.json` como manifesto, e o `nx.json` resolve a versão do disco.

3. **`dmpf-units.json`** — o `metadata_container` da RFC DMPF, obrigatório em
   todo módulo de produção desde a criação. Em Go o `include` usa **import
   paths**, não globs.

4. **Targets que o plugin não infere** — o `@nx-go/nx-go` infere `test`, `lint`,
   `tidy` e `generate`, mas **não** `build`. Declare `fmt-check`, `vet`, `build`,
   `test-race` e `govulncheck` com `cwd: "{projectRoot}"` e
   `inputs: ["go", "^go"]` (`govulncheck` com `cache: false`, porque consulta
   base remota).

   O `lint` e o `test` **não** entram no `project.json`: vêm de `targetDefaults`
   chaveado por executor (`@nx-go/nx-go:lint` e `@nx-go/nx-go:test`, em
   `nx.json`), então todo módulo Go novo já nasce com o golangci-lint no lugar do
   `go fmt` e com o `.golangci.yml` nos inputs — mudar a política invalida o
   cache em vez de devolver o verde anterior. Copiar o bloco por projeto foi o
   desenho anterior e tinha um modo de falha silencioso: um módulo sem ele roda
   `go fmt ./...`, que **reescreve** os arquivos e sai 0, ficando verde sem
   lintar nada.

   Ao mexer nesses defaults por executor, repita `dependsOn` e `cache`: o default
   por executor **substitui** o default por nome, e omiti-los faz o `test` do Go
   perder o `^build` e o cache que `targetDefaults.test` fornece.

### Manualmente

1. Crie o diretório `libs/<scope>/<name>/`
2. Crie o `project.json` com as três tags obrigatórias
3. Crie o `package.json` com `"version": "0.0.0"` e `"type": "module"`
4. Crie `tsconfig.json`, `tsconfig.lib.json` e (se tiver testes) `tsconfig.spec.json`
5. Adicione o path alias em `tsconfig.base.json`
6. Adicione a reference em `tsconfig.json` raiz

---

## Caching: o que afeta o hash

O NX computa um hash antes de rodar qualquer task cacheável. Se o hash bater
com uma execução anterior, o resultado é restaurado sem rodar o comando.

### Inputs configurados globalmente (`namedInputs`)

| Named input | Conteúdo |
|-------------|----------|
| `default` | Todos os arquivos do projeto + `sharedGlobals` |
| `production` | `default` sem arquivos de teste/spec |
| `sharedGlobals` | `tsconfig.base.json`, `nx.json`, `biome.json`, `pnpm-workspace.yaml`, `pnpm-lock.yaml` |

O prefixo `^` significa "inclui os inputs dos projetos dependentes também".
Ex: `"^production"` → mudanças nos arquivos de produção de qualquer dependência
invalidam o cache desta task.

### Depurar cache misses

```bash
# Ver a configuração completa resolvida de um projeto
pnpm nx show project @mateusmacedo/minha-lib

# Versão visual no browser
pnpm nx show project @mateusmacedo/minha-lib --web

# Limpar o cache local
pnpm nx reset
```

---

## Pipeline de dependências (`dependsOn`)

```
typecheck ──depends──▶ ^typecheck (typecheck de dependências)
build     ──depends──▶ ^build     (build de dependências)
test      ──depends──▶ ^build     (build de dependências antes do teste)
```

O símbolo `^` antes do nome do target significa "o mesmo target nos projetos
dos quais este projeto depende". Sem `^`, é um target do próprio projeto.

### Exemplo: adicionar um pre-build customizado

```json
// project.json
{
  "targets": {
    "build": {
      "dependsOn": ["^build", "gerar-tipos"]
    },
    "gerar-tipos": {
      "command": "node scripts/gerar-tipos.mjs"
    }
  }
}
```

---

## Sync generators

O workspace usa `@nx/js:typescript-sync` para manter as referências de
TypeScript Project References sincronizadas automaticamente.

**Em máquinas de desenvolvedor:** as mudanças são aplicadas automaticamente
(`sync.applyChanges: true` em `nx.json`).

**Em CI:** o `ci.yml` **não** executa `nx sync:check` hoje. Quem quiser que o
pipeline falhe em caso de referências dessincronizadas precisa adicionar o passo
explicitamente — ele reporta um diff claro do que falta sincronizar:

```yaml
      - name: Sync check
        run: pnpm nx sync:check
```

```bash
# Verificar sincronização manualmente
pnpm nx sync:check

# Aplicar sincronização manualmente
pnpm nx sync
```

---

## Referências

- [nx.json reference](https://nx.dev/reference/nx-json)
- [Project configuration reference](https://nx.dev/reference/project-configuration)
- [Inputs reference](https://nx.dev/reference/inputs)
- [Cache task results](https://nx.dev/docs/features/cache-task-results)
- [Task pipeline configuration](https://nx.dev/docs/concepts/task-pipeline-configuration)
- [Inferred tasks](https://nx.dev/docs/concepts/inferred-tasks)
- [Sync generators](https://nx.dev/docs/concepts/sync-generators)
