# ADR-002: Configuração de Tasks, Cache e Pipelines no NX

**Status:** Aceito
**Data:** 2026-04-19

## Contexto

O workspace NX v22 usa um modelo de três camadas para configuração de tasks:
plugins (inferência automática) → `targetDefaults` (nx.json) → `project.json`
(por projeto). Sem uma política clara, projetos tendem a duplicar configuração
nas camadas erradas, causando:

- Cache desabilitado silenciosamente (target em `project.json` sobrescreve o
  `targetDefault` e perde `cache: true` e `inputs`)
- Boilerplate repetido em dezenas de `project.json`
- Inconsistências de tag impossibilitando `nx affected` e module boundary rules

## Decisão

### 1. Camadas de configuração

Cada camada tem responsabilidade exclusiva:

| Camada | Responsabilidade |
|--------|-----------------|
| Plugin inferido | Executor, inputs e outputs derivados do tsconfig/jest config |
| `targetDefaults` | `dependsOn`, `cache`, executor global (lint via Biome) |
| `project.json` | Tags obrigatórias + exceções específicas do projeto |

**Regra:** nunca redeclare em `project.json` o que o plugin ou `targetDefault`
já cobre. Um target vazio em `project.json` com o mesmo executor do
`targetDefault` sobrescreve silenciosamente `cache` e `inputs`, quebrando o cache.

### 2. Tags obrigatórias (3D)

Todo projeto nasce com exatamente três tags:

```
type:(lib|app)   scope:<domínio>   stack:(node|react|angular|universal)
```

Aplicadas via:
- Generator `shared-lib` para libs compartilhadas
- (Futuro) Generators de app para cada stack

### 3. `targetDefaults` cobre lint globalmente

O lint via Biome é configurado uma única vez em `targetDefaults`:

```json
"lint": {
  "executor": "nx:run-commands",
  "options": { "command": "biome lint {projectRoot}" },
  "cache": true,
  "inputs": ["default", "{workspaceRoot}/biome.json"]
}
```

Projetos **não declaram** o target `lint` em `project.json`.

### 4. Versão inicial de libs: `0.0.0`

Todos os `package.json` de libs nascem com `"version": "0.0.0"`. O NX Release
com `fallbackCurrentVersionResolver: "disk"` usa esse valor como baseline
quando não há git tag, eliminando a necessidade de `--first-release` manual.

### 5. Sync generators habilitados

`@nx/js:typescript-sync` mantém as TypeScript Project References sincronizadas.
Em dev, as mudanças são aplicadas automaticamente. Em CI, `nx sync:check`
detecta divergências antes das tasks.

## Consequências

- Todo novo projeto **deve** usar o generator correspondente, que aplica as
  regras automaticamente
- Projetos criados manualmente **devem** seguir `docs/nx-reference/tasks.md`
- O CI detecta automaticamente projetos com referências TypeScript inconsistentes
- `nx affected` funciona corretamente porque as tags estão padronizadas
- Cache funciona corretamente porque `targetDefaults` não é sobrescrito
