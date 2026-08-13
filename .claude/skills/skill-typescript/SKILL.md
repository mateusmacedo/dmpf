---
name: skill-typescript
description: >-
  Use esta skill ao escrever, editar ou revisar código TypeScript (.ts) de backend.
  Reúne convenções comuns que diferem dos padrões nativos: `type` sobre `interface`,
  union types em vez de `enum`, arrow functions, named params, `import type`, sem barrel files,
  `T[]`, `noInferrableTypes`, entre outras. Use também quando o usuário mencionar erros de tipo,
  tipagem, discriminated union, type guard, utility types (Record, Partial, Omit, Pick, Extract,
  Exclude), keyof, typeof, satisfies, readonly, z.infer, ReturnType ou `as const`, ou ao tipar
  entities, repositories, DTOs, use cases ou respostas de API.
model: sonnet
---

# TypeScript — padrões e convenções

Guia de referência com padrões TypeScript comuns em backend. Adapte ao padrão vigente do projeto — quando houver divergência, preserve o padrão local.

## Preferências recomendadas

- Prefira `type` a `interface` (exceção: declaration merging em `.d.ts`).
- Prefira criar tipos em `types.ts` separado da lógica quando forem reusados ou expostos.
- Tipos complexos desaninhados em subtipos nomeados, referenciados no tipo principal.
- Propriedades opcionais (`?`) antes das obrigatórias no objeto tipado.
- Naming em `PascalCase`; DTOs com sufixo `Dto`, entities com `Entity`.
- Funções com 2+ parâmetros aceitam objeto desestruturado com tipo nomeado.

## Regras comuns (ajustáveis)

| Regra | Enforcement típico |
|-------|--------------------|
| `type` sobre `interface` | Convenção / Biome |
| `import type` / `export type` para tipos | `verbatimModuleSyntax` + linter |
| Sem `enum` — union types ou `as const` | Linter (`noEnum`) |
| Sem barrel files | Linter (`noBarrelFile`) |
| Sem `!` (non-null assertion) | Linter (`noNonNullAssertion`) |
| Sem `any` explícito | Linter (`noExplicitAny`) |
| `T[]` em vez de `Array<T>` | Linter (`useConsistentArrayType`) |
| Sem tipos inferíveis redundantes | Linter (`noInferrableTypes`) |
| Sem `Boolean`/`Number`/`String` wrapper | Linter (`noBannedTypes`) |
| Sem ciclos de importação | Linter (`noImportCycles`) |
| Arrow functions com `const` | Convenção |
| Named exports (sem default) | Convenção |
| Sem `;` | Formatter |
| `??` sobre `\|\|`, `?.` quando aplicável | Linter |

## Naming sugerido

| Categoria | Convenção | Exemplo |
|-----------|-----------|---------|
| Domínio | `PascalCase` | `CourtDocument`, `Profile` |
| DTO | `<Ação><Domínio>Dto` | `CreateUserDto` |
| Entity | `<Domínio>Entity` | `UserEntity` |
| Repository | `<Domínio>Repository` | `UserRepository` |
| Use Case | `<Ação><Domínio>UseCase` | `CreateUserUseCase` |
| Schema Zod | `<recurso>Schema` | `createUserSchema` |
| Constante | `UPPER_SNAKE_CASE` | `CACHE_TTL_MS` |
| Variável | `camelCase` | `selectedIds` |

## Utility types — referência rápida

| Tipo | Uso |
|------|-----|
| `Record<K, V>` | Mapear union a valores |
| `Partial<T>` | Todos opcionais |
| `Pick<T, K>` / `Omit<T, K>` | Selecionar/remover propriedades |
| `Extract<T, U>` / `Exclude<T, U>` | Filtrar/excluir de union |
| `ReturnType<T>` / `Parameters<T>` | Tipo de retorno/parâmetros |
| `Awaited<T>` | Desembrulhar Promise |
| `NonNullable<T>` | Remover null/undefined |

## Nullability

- `null` para ausência explícita (ex.: responses de API).
- `undefined` para propriedades opcionais (`label?: string`).
- Prefira `??` a `||` para evitar falsos positivos com `0` e `''`.
- Evite `!` — prefira optional chaining ou narrowing.

## Referências detalhadas (carregar sob demanda)

| Arquivo | Conteúdo |
|---------|----------|
| `references/core-rules.md` | type vs interface, enums, funções, imports/exports |
| `references/advanced-types.md` | Discriminated unions, type guards, generics |
| `references/data-patterns.md` | Zod, async, erros, services, DTOs, use cases, jobs |
| `references/backend-patterns.md` | TypeORM entities, repository interfaces, DTOs, decorators NestJS, jobs BullMQ |

## Checklist

- [ ] Uso de `type` em vez de `interface` (quando aplicável).
- [ ] Tipos em `types.ts` separado quando fizer sentido.
- [ ] Opcionais (`?`) antes das obrigatórias.
- [ ] `import type` e `export type` para tipos.
- [ ] Sem `enum` — union types ou `as const`.
- [ ] Arrow functions com `const`.
- [ ] `T[]` em vez de `Array<T>`.
- [ ] Sem tipos redundantes (`const x: string = 'foo'`).
- [ ] Discriminated unions para estados e mensagens.
- [ ] `safeParse` para runtime, `parse` para startup/env.
- [ ] `error: unknown` em catch blocks.
- [ ] Named exports; evitar barrel files.
- [ ] `readonly` em arrays que não devem ser mutados.
