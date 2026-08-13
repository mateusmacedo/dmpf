---
paths:
  - "**/*"
---

# Processo e qualidade

Orientações de processo para apoiar qualidade e rastreabilidade. Adapte ao workflow do time.

## Features novas

Toda feature, módulo ou serviço novo exige uma spec correspondente em `docs/specs/` antes de a implementação começar:

- Confirme que a spec (ou documento de design) já existe antes de escrever código.
- Escreva a spec antes de implementar quando ela ainda não existir; peça a criação ao usuário sempre que o escopo depender de decisões dele.

O formato do artefato (spec, PRD, RFC, ADR) depende do processo do projeto. Bugs simples e manutenção rotineira ficam isentos de spec.

## Plano antes de implementar

Mudanças que tocam 2 ou mais arquivos exigem um plano breve aprovado antes da execução:

- Apresente o plano com o que será feito, os arquivos afetados e os riscos ou tradeoffs.
- Aguarde aprovação explícita do usuário antes de prosseguir.

## Refatorações amplas

Toda refatoração exige um plano documentado antes da execução:

- Documente o plano com motivação, escopo, arquivos impactados e estratégia de validação.
- Confirme o plano com o usuário antes de aplicar mudanças, sobretudo quando o impacto cruza camadas.

## Exceções

Estas situações dispensam spec ou plano prévio e podem ser executadas direto:

- Correção de 1 linha (typo ou bug óbvio).
- Tarefa em arquivo único com instrução clara e específica.
- Pedido explícito do usuário ("pode fazer direto", "sem plano").

Bug simples e manutenção rotineira seguem isentos de spec.

## Reutilização

Antes de criar código novo:

- Busque implementações similares no repositório.
- Reuse quando fizer sentido; duplicação sem motivo costuma gerar dívida.

## Validação

Após qualquer alteração, rode a cadeia de validação adotada pelo projeto. Tipicamente:

1. Lint.
2. Typecheck.
3. Testes.

Verifique quais passos rodam em hooks automáticos (pré-commit, pré-push) e quais dependem de execução manual. O build de distribuição costuma ser executado de forma explícita antes de releases.

---

## 🔹 Go: processo e cadeia de validação

A cadeia típica em Go é mais ampla que o trio "lint, typecheck, testes" do TS — tipagem é validada pelo build, mas adicionam-se etapas próprias (race, vet, vuln).

### Cadeia comum

1. `gofmt -l .` ou `goimports -l .` — formato (falha se diff).
2. `go vet ./...` — análise estática built-in.
3. `golangci-lint run` — agrega `staticcheck`, `errcheck`, `gosimple`, `unused`, etc.
4. `go build ./...` — equivalente ao typecheck (compila tudo).
5. `go test ./...` — testes unitários e de integração.
6. `go test -race ./...` — em CI, habilitar race detector.
7. `govulncheck ./...` — varre CVEs de dependências (oficial).

### Convenções e variações

- `Makefile` ou `Taskfile.yml` costumam encapsular esses passos (`make ci`, `task ci`).
- `go.mod` e `go.sum` são commitados; rodar `go mod tidy` antes de PR para evitar drift.
- Builds para release usam `-trimpath` e `-ldflags` para versão; geração reprodutível é viável.
- Hooks: `lefthook` ou `pre-commit` (sim, o de Python, agnóstico) para reaproveitar entre stacks.

### Reutilização e features

A orientação de spec/RFC/ADR antes de mudanças amplas se aplica igualmente. Em Go é comum existir um diretório `docs/adr/` no próprio repositório, com `adr-tools` ou Markdown manual.
