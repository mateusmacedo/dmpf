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
  "name": "@lidercap-apps/minha-lib",
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

A fonte canônica desta taxonomia é o `AGENTS.md`; a tabela abaixo a repete e não
deve divergir dele.

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

---

## Como criar uma nova lib

### Via generator Nx (recomendado)

```bash
# lib compartilhada (TypeScript puro, buildable via tsc)
pnpm nx g @nx/js:lib libs/shared/<name> \
  --importPath=@lidercap-apps/shared-<name> \
  --bundler=tsc --unitTestRunner=jest --linter=none \
  --tags=type:lib,scope:shared,stack:node

# lib backend (NestJS)
pnpm nx g @nx/nest:lib libs/backend/<name> \
  --importPath=@lidercap-apps/backend-<name> \
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
pnpm nx show project @lidercap-apps/minha-lib

# Versão visual no browser
pnpm nx show project @lidercap-apps/minha-lib --web

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
