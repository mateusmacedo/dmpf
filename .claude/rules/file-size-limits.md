---
paths:
  - "**/*.ts"
  - "**/*.go"
---

# Limites de tamanho

Limites numéricos de tamanho são heurísticas: devem ser proporcionais à complexidade do código. Os valores abaixo são defaults configuráveis.

| Tipo | Default sugerido | Ação quando ultrapassar |
|------|------------------|-------------------------|
| Arquivos `.ts` | ~300 linhas | Extrair módulos, classes, funções |
| Funções `.ts` | ~80 linhas | Dividir em funções menores |

- Ao criar ou refatorar, confira se o default continua razoável.
- Se o código ultrapassar, avise e proponha decomposição.
- Não quebre módulos automaticamente sem checar com o usuário; divisões mecânicas podem piorar a coesão.

---

## 🔹 Go: limites e métricas de complexidade

Em Go a unidade de tamanho relevante é geralmente o **pacote**, não o arquivo. Como vários arquivos formam um pacote, "arquivo grande" costuma ser sintoma menor que "pacote grande".

| Tipo | Default sugerido | Ferramenta de checagem |
|------|------------------|------------------------|
| Função | ~80 linhas / complexidade ciclomática ~15 | `gocyclo`, `gocognit`, `funlen` (golangci-lint) |
| Arquivo `.go` | ~400-500 linhas (mais permissivo que TS) | manual / linter customizado |
| Pacote | Coerência semântica > linhas | `dupl`, `gocognit` agregados |

- `golangci-lint` agrega vários linters de complexidade; `funlen`, `gocyclo`, `gocognit`, `lll` (line length) são os mais comuns.
- Em Go é idiomático ter funções curtas; o pattern matching por `switch` e o tratamento de erro explícito tendem a inflar tamanho. Decompor por papel (validar, executar, mapear) ajuda.
- Pacotes muito grandes geralmente revelam falta de fronteira de domínio — quebre por subpacote (`user/`, `billing/`) ao invés de só dividir arquivos.
