# Regras Core — Detalhamento

Convenções comuns para código TypeScript. Trate os itens como defaults razoáveis; o projeto pode ajustar conforme sua configuração de linter/formatter.

## `type` vs `interface`

Ambos servem. `type` é mais flexível (suporta unions, intersecções, mapped types, tuplas). Escolha um padrão para o projeto e seja consistente.

```ts
// Opção comum: type
type UserData = {
  name: string
  email: string
}

// Alternativa: interface (útil para declaration merging)
interface UserData {
  name: string
  email: string
}
```

Casos em que `interface` é tecnicamente necessário:
- Declaration merging (augmentar tipos globais em `.d.ts`).
- Interop com libs que esperam interfaces extensíveis.

## Organização de tipos

Uma opção comum é colocalizar tipos em `types.ts` junto do módulo:

```
use-case/
├── index.ts      # lógica
├── schema.ts     # schemas de validação
└── types.ts      # tipos TypeScript
```

Tipos compartilhados entre módulos podem viver em um diretório como `src/@types/` ou `src/shared/types/`.

### Propriedades opcionais

A ordenação (opcionais primeiro ou depois) é preferência de estilo. Exemplo com opcionais primeiro:

```ts
type DocumentParams = {
  category?: string
  tags?: string[]
  title: string
  url: string
}
```

### Tipos inline vs nomeados

Tipos nomeados tendem a melhorar legibilidade quando reutilizados ou quando a assinatura é complexa:

```ts
// Assinatura curta — inline aceitável
const setName = (params: { name: string }) => {}

// Assinatura com múltiplas propriedades — tipo nomeado facilita
type CreateParams = { title: string; url: string; tags?: string[] }
const create = (params: CreateParams) => {}
```

---

## Enums e constantes

### Union types como alternativa a `enum`

Union types são frequentemente preferidos por serem mais leves em runtime e interoperáveis:

```ts
type Status = 'idle' | 'loading' | 'success' | 'error'

const PLAN_STATES = ['active', 'canceled', 'inactive'] as const
type PlanState = typeof PLAN_STATES[number]
```

`enum` continua útil em integrações que esperam valores numéricos ou quando o projeto já padronizou o uso.

### `Record` para mapear union a label

```ts
const PLAN_LABELS: Record<PlanType, string> = {
  basic: 'Básico',
  advanced: 'Avançado',
  ia: 'Inteligência Artificial',
  teams: 'Times',
}
```

### `as const` para literais

```ts
const CONFIG = {
  maxRetries: 3,
  timeout: 5_000,
} as const

const mockDoc = { type: 'pdf' as const }
```

---

## Funções

### Arrow function vs `function`

Ambas são válidas. Arrow function com `const` é uma opção comum por uniformidade e por evitar hoisting:

```ts
export const fetchUser = async (id: string) => {}
```

`function` é preferível quando é necessário hoisting ou `this` dinâmico.

### Parâmetros nomeados

Para funções com múltiplos parâmetros não triviais, um objeto de opções melhora legibilidade e extensibilidade:

```ts
type FetchOptions = { url: string; timeout: number }
const fetchData = ({ url, timeout }: FetchOptions) => {}
```

Callbacks de array e comparadores (`.sort((a, b) => ...)`) são casos naturais de parâmetros posicionais.

### Nomes descritivos

Nomes que expressam o papel do parâmetro ajudam em revisão:

```ts
// Menos claro
.map(d => d.name)

// Mais claro
.map(doc => doc.name)
```

Usos idiomáticos de nomes curtos: `(a, b)` em comparadores, `_` para valores descartados, `i/j` em loops `for` tradicionais.

---

## Imports e exports

### `import type` para tipos

`import type` permite ao bundler/compilador eliminar imports apenas de tipo:

```ts
import type { User, Profile } from '@/types'
import type { Repository } from 'typeorm'
import { createUser } from '@/services/user'
```

### Aliases para imports absolutos

Aliases (`@/`, `~/`, etc.) reduzem fragilidade frente a movimentação de arquivos:

```ts
// Preferível
import { encrypt } from '@/libs/crypto'

// Frágil com refactors
import { encrypt } from '../../../libs/crypto'
```

### Named vs default exports

Named exports oferecem refatoração automática e evitam divergência de nome entre arquivos. Default exports são exigidos por certos frameworks (ex.: módulos/controllers em determinadas configurações). Siga a convenção do projeto.

```ts
export const createUser = () => {}
export type CreateUserInput = { /* ... */ }
```

### Barrel files

Barrel files (`index.ts` que apenas re-exporta) têm trade-offs: facilitam imports, mas podem complicar tree-shaking e aumentar tempos de compilação em projetos grandes. Decisão do projeto.

### Ordem de imports

Uma ordenação comum (verificável via linter):

1. Node built-ins
2. Dependências de framework (ex.: nestjs, express)
3. Pacotes npm externos
4. Aliases internos (por subgrupo: constants, libs, helpers, infra, domain, services)
5. Relativos (`../` antes de `./`)
6. `import type` agrupado ao final de cada bloco

Delegar a ordenação ao formatter/linter é a prática mais sustentável.

---

## Regras adicionais

### `T[]` vs `Array<T>`

Ambas funcionam. `T[]` é mais conciso; `Array<T>` é necessário para tipos complexos. Escolha um padrão.

### Tipos inferíveis

Evite repetir tipos que o TypeScript já infere:

```ts
// Redundante
const name: string = 'John'

// Inferido
const name = 'John'
```

Declarar o tipo explicitamente é útil quando a inferência não é óbvia (retorno complexo, função exportada como API pública).

### Wrapper types

Prefira os primitivos (`string`, `number`, `boolean`) aos wrappers (`String`, `Number`, `Boolean`). Wrappers têm semântica diferente (objetos).

### `biome-ignore` / `eslint-disable` com justificativa

Quando silenciar uma regra for necessário, deixe um comentário explicando o motivo:

```ts
// biome-ignore lint/suspicious/noExplicitAny: accessing axios internal handlers
const manager = httpClient.interceptors.request as any
```

### Nomes de índices em callbacks

Em `forEach`/`map`, `index` costuma ser mais claro que `i`. Loops `for` tradicionais aceitam `i/j` por convenção.
