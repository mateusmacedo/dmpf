---
paths:
  - "biome.json"
  - "lefthook.yml"
  - "nx.json"
  - "package.json"
  - "tsconfig*.json"
  - ".github/workflows/**"
---

# CI e qualidade

## Ferramentas (exemplos)

| Categoria | Ferramentas comuns |
|-----------|--------------------|
| Lint | ESLint, Biome |
| Formatação | Prettier, Biome |
| Tipagem | TypeScript em modo estrito |
| Testes | Jest, Vitest |

A combinação real depende do projeto. Ajuste os comandos aos scripts definidos em `package.json`.

## Hooks de Git (exemplos)

- **pre-commit**: lint, format e typecheck quando configurado.
- **pre-push**: execução dos testes quando configurado.

Ferramentas usadas com frequência: Husky, Lefthook, simple-git-hooks. Cada repositório pode adotar (ou não) hooks; respeite a configuração vigente.

## Commits

- **Formato**: Conventional Commits costuma ser adotado.
- **Limite**: é comum restringir a primeira linha a 100-120 caracteres.
- **Validação**: Commitlint ou equivalente quando configurado.
- **Idioma**: Conventional Commits é tradicionalmente em inglês; confirme o padrão do projeto.

## Comandos

Rode os scripts de lint, format, typecheck e testes definidos no `package.json`. Em caso de dúvida sobre o gerenciador de pacotes (`npm`, `pnpm`, `yarn`, `bun`), verifique o campo `packageManager` e os arquivos de lockfile antes de rodar comandos.

---

## 🔹 Go: ferramentas e fluxo equivalentes

Em projetos Go o fluxo de qualidade tende a ser mais "incluso na toolchain":

| Categoria | Ferramentas comuns em Go |
|-----------|--------------------------|
| Lint | `go vet` (built-in), `golangci-lint` (agregador), `staticcheck` |
| Formatação | `gofmt` / `goimports` (built-in/oficiais), `gofumpt` (mais estrito) |
| Tipagem | Compilador Go (sempre estrito; `go build ./...`) |
| Testes | `go test ./...` (built-in), `gotestsum` (UI), `testify` (asserts) |
| Cobertura | `go test -cover` / `-coverprofile` (built-in) |
| Race detector | `go test -race ./...` |
| Build reprodutível | `go build -trimpath` |
| Vulnerabilidades | `govulncheck` (oficial) |

### Hooks de Git (em Go)

- **pre-commit**: `gofmt -l`, `goimports -l`, `golangci-lint run`, `go vet ./...`, `go build ./...`.
- **pre-push**: `go test ./...` (eventualmente `-race` em CI).

Ferramentas comuns para hooks: `lefthook`, `pre-commit` (sim, o de Python), ou um `Makefile` simples chamado pelos hooks.

### Commits

Conventional Commits também é o padrão dominante em Go. Não há ferramenta canônica de commit lint específica do ecossistema; use `commitlint` ou `commitizen` mesmo (CLI agnóstica) ou hooks que validem regex.

### Comandos

Em projetos Go a fonte de verdade costuma ser o `Makefile` (ou `Taskfile.yml` com [Task](https://taskfile.dev/)) e o `go.mod`. Antes de rodar:

- Confira a versão de Go pedida no `go.mod` (`go 1.XX`).
- `go mod download` baixa dependências; `go mod tidy` mantém o `go.mod`/`go.sum` em ordem.
- O lockfile é o próprio `go.sum` — não edite à mão.
