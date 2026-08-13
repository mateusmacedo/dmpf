---
paths:
  - "**/*.ts"
  - "**/*.go"
---

# Padrões de código

Os itens abaixo são defaults recomendados para TypeScript. Adapte aos padrões vigentes do projeto.

## Sintaxe (sugestões)

- Configurar o formatador para controlar ponto e vírgula, comprimento de linha, etc., em vez de aplicar regras manuais.
- Evitar `var`.
- Em vez de `enum`, costuma funcionar melhor usar `as const` ou unions literais — avalie caso a caso.
- Prefira `type` a `interface` quando não houver extensão/merge.
- Arrow functions com `const` para funções no topo do módulo.
- Use `import type` para imports exclusivamente de tipos.

## Naming

- Nomes em inglês; evite misturar idiomas.
- Arquivos `.ts`: `camelCase` é comum para arquivos utilitários; classes/tipos podem usar `PascalCase` (siga a convenção do projeto).
- Variáveis/funções: `camelCase`.
- Tipos: `PascalCase`.
- Constantes globais: `UPPER_SNAKE_CASE` quando fizerem sentido semanticamente.
- Para DTOs e entidades, adote a convenção do projeto (por exemplo, DTOs com sufixo `Dto`).

## Exports

- Prefira export nomeado diretamente na declaração.
- Reexports e barrel files podem ser úteis em casos específicos (por exemplo, APIs públicas de pacotes); se o projeto evita barrels por decisão arquitetural, respeite o padrão.

## Imports

- Evite `./index` ou `../index` explícitos — o bundler resolve `'.'` ou `'..'`.
- Ordem comum: `import type` primeiro, módulos externos, módulos locais. Um formatter/linter costuma cuidar disso automaticamente.

## Parâmetros nomeados

Funções com dois ou mais parâmetros costumam ficar mais legíveis com um único objeto desestruturado:

```ts
const createDocument = ({ title, url }: CreateDocumentParams) => {}
```

Exceções razoáveis: APIs externas, callbacks como `.sort((a, b) => ...)`, funções com um único parâmetro.

## Tipos

- Considere extrair tipos para um arquivo `types.ts` próximo do consumidor.
- Em um objeto de parâmetros, agrupe opcionais antes de obrigatórios pode facilitar leitura; o essencial é manter consistência.
- Evite interseções inline complexas em assinaturas; nomeie o tipo quando ganhar clareza.

## Schemas de validação

- Zod, Valibot e class-validator são alternativas comuns. Use a ferramenta adotada pelo projeto.
- Manter schemas/DTOs em arquivos dedicados favorece reuso.

## Constantes

- Para dados estáticos repetidos (três ou mais itens), considere um módulo `constants.ts` próximo do consumidor.
- Números mágicos que aparecem mais de uma vez costumam virar constantes nomeadas.

## Comentários

- Prefira código auto-explicativo.
- Comentários são úteis para decisões não óbvias, TODOs com ticket (`// TODO: XYZ-123`) e desativações deliberadas (`// noop`).

---

## 🔹 Go: padrões equivalentes

Em Go várias decisões de estilo são impostas pela toolchain (gofmt, go vet, naming exportado por capitalização). O que cabe ao desenvolvedor é menor — o que existe está abaixo.

### Sintaxe

- Formatador é obrigatório: `gofmt`/`goimports` são canônicos. `gofumpt` é opção mais estrita.
- Não há `var` vs `let` vs `const` — Go tem `var`, `const` e `:=` (short declaration). Use `:=` dentro de funções; `var` no escopo de pacote.
- Não há `enum`. Use `const` block com `iota` ou um tipo string com constantes nomeadas:
  ```go
  type Status string
  const (
      StatusActive   Status = "active"
      StatusInactive Status = "inactive"
  )
  ```
- Não há diferença `type` vs `interface` para alias — `type` declara tudo (alias, struct, interface). Para "alias real" sem novo tipo: `type Alias = Original`.
- Funções top-level são `func nome(...)`; closures atribuídas a vars existem mas não são idiomáticas no nível de pacote.
- Imports de tipo não existem como construção — todos os imports são iguais; `goimports` cuida da ordenação.

### Naming

- Nomes em inglês.
- Arquivos `.go`: `snake_case` é o padrão (ex: `user_repository.go`, `http_handler.go`).
- Identificadores exportados: `PascalCase` (ex: `CreateUser`, `UserRepository`).
- Identificadores não exportados (privados ao pacote): `camelCase` (ex: `parseHeader`, `userCache`).
- Constantes: a regra é a mesma — `PascalCase` se exportada, `camelCase` se interna. `UPPER_SNAKE_CASE` não é idiomático.
- Acrônimos preservam capitalização inteira: `UserID` (não `UserId`), `HTTPClient`, `URLParser`.
- Receivers de método: 1-2 letras derivadas do tipo (`u *User`, não `this`/`self`).

### Exports

- Visibilidade é controlada pela primeira letra do identificador (maiúscula = exportado, minúscula = privado ao pacote).
- Não existe `export` explícito; reorganizar visibilidade é um rename.
- "Barrel files" não existem; o pacote inteiro é a unidade de export.

### Imports

- Caminhos são module paths definidos em `go.mod` (ex: `github.com/empresa/projeto/internal/user`).
- Não há aliases tipo `@/`; aliases por import existem (`alias "pacote/longo"`) mas devem ser evitados quando o nome do pacote já basta.
- `goimports` agrupa em três blocos: stdlib, externos, internos do módulo.

### Parâmetros nomeados

Go não tem named parameters. Para múltiplos parâmetros opcionais, dois padrões:

```go
// Struct de opções (mais comum em APIs internas)
type CreateDocumentParams struct {
    Title string
    URL   string
    Tags  []string
}
func CreateDocument(p CreateDocumentParams) error { ... }

// Functional options (comum em libs públicas)
func NewServer(opts ...Option) *Server { ... }
```

### Tipos

- Colocalização: arquivo `types.go` ao lado do consumidor é comum, ainda que muitos projetos prefiram declarar o tipo no mesmo arquivo onde é usado.
- Composição de struct é por embedding, não herança.
- Interseções não existem; embedding de interfaces compõe contratos.

### Schemas de validação

Não há equivalente direto a Zod/class-validator com integração ao tipo. Alternativas:

- `go-playground/validator` — tags em structs (`validate:"required,email"`).
- `ozzo-validation` — DSL programática.
- Validação manual em construtores — comum em Go idiomático (preserva tipos sem reflexão).

### Constantes

- `const` em bloco no topo do arquivo é idiomático.
- Números mágicos repetidos viram constantes nomeadas; também é comum tipar a constante (`const MaxRetries int = 3`).

### Comentários

- Comentários de godoc são parte da API: começam com o nome do identificador exportado (`// User represents...`, `// CreateUser creates...`).
- TODOs: `// TODO(usuário): descrição`. `go vet` reconhece a convenção.
- Não há diretivas `// noop` idiomáticas; um comentário curto explicando o motivo basta.
